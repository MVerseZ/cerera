package icenet

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/icenet/consensus"
	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/peers"
	"github.com/cerera/internal/icenet/protocol"
	icesync "github.com/cerera/internal/icenet/sync"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

func iceLogger() *zap.SugaredLogger {
	return logger.Named("icenet")
}

const (
	IceVersion = "1.0.0"
)

// IceAddress contains network address information
type IceAddress struct {
	IP       string
	Port     string
	CAddress types.Address
	PeerID   peer.ID
}

// Ice is the main P2P network component
type Ice struct {
	Host            host.Host
	DHT             *dht.IpfsDHT
	Discovery       *Discovery
	PubSub          *PubSubManager
	PeerManager     *peers.Manager
	PeerScorer      *peers.Scorer
	SyncManager     *icesync.Manager
	Consensus       *consensus.Manager
	ServiceProvider service.ServiceProvider
	Handler         *protocol.Handler
	Address         IceAddress
	cfg             *config.Config
	ctx             context.Context
	cancel          context.CancelFunc

	// Dev-mode: treat connected peers as validators.
	devValidators bool

	mu sync.RWMutex // guards fields read by Exec() and written during Start()/SetServiceProvider()
}

// Start initializes and starts the Ice P2P network component
func Start(cfg *config.Config, ctx context.Context, port string) (*Ice, error) {
	iceLogger().Infow("Starting Ice P2P network...", "version", IceVersion, "port", port)

	iceLogger().Infow("Retrieving keys from vault...")
	priv, pub, err := storage.GetKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve keys from vault: %w", err)
	}
	iceLogger().Infow("Keys retrieved from vault", "keys", priv, pub)

	ctx, cancel := context.WithCancel(ctx)

	ice := &Ice{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	ice.devValidators = envBool("ICE_DEV_VALIDATORS", true)

	// Create libp2p host
	h, err := NewHost(ctx, cfg, port)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create host: %w", err)
	}
	ice.Host = h

	// Set address info
	ice.Address = IceAddress{
		Port:     port,
		CAddress: cfg.NetCfg.ADDR,
		PeerID:   h.ID(),
	}
	if len(h.Addrs()) > 0 {
		ice.Address.IP = h.Addrs()[0].String()
	}

	// Create peer manager
	ice.PeerManager = peers.NewManager(ctx, h, peers.DefaultMaxPeers)
	ice.PeerManager.Start()

	// Create peer scorer
	ice.PeerScorer = peers.NewScorer(ice.PeerManager)

	// Create protocol handler (initially without chain/txPool - will be set later)
	ice.Handler = protocol.NewHandler(h, nil, cfg.NetCfg.ADDR, cfg.GetVersion())
	ice.Handler.RegisterHandlers()

	// Create discovery service
	discovery, err := NewDiscovery(ctx, h, cfg)
	if err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to create discovery: %w", err)
	}
	ice.Discovery = discovery
	ice.DHT = discovery.GetDHT()

	// Start discovery
	if err := discovery.Start(); err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to start discovery: %w", err)
	}

	// Create PubSub manager
	pubsubMgr, err := NewPubSubManager(ctx, h)
	if err != nil {
		cancel()
		discovery.Stop()
		h.Close()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}
	ice.PubSub = pubsubMgr

	// Start PubSub
	if err := pubsubMgr.Start(); err != nil {
		cancel()
		discovery.Stop()
		h.Close()
		return nil, fmt.Errorf("failed to start pubsub: %w", err)
	}

	// Create consensus manager (publish under lock so Exec() sees it safely)
	consensusMgr := consensus.NewManager(ctx, h, ice.PeerManager, ice.PeerScorer, cfg.NetCfg.ADDR, nil)
	ice.mu.Lock()
	ice.Consensus = consensusMgr
	ice.mu.Unlock()

	consensusMgr.SetOnBlockFinalized(func(b *block.Block) {
		if b != nil && b.Head != nil {
			metrics.SetBlockHeight(b.Head.Height)
		}
		metrics.RecordBlockValidated()
	})
	// Dev-mode validator set: treat connected peers as validators, including self.
	if ice.devValidators {
		consensusMgr.AddValidator(h.ID())
	}

	// Setup peer callbacks
	ice.PeerManager.SetOnPeerConnected(ice.onPeerConnected)
	ice.PeerManager.SetOnPeerDisconnected(ice.onPeerDisconnected)

	// Setup pubsub callbacks
	ice.PubSub.SetOnBlock(ice.onPubSubBlock)
	ice.PubSub.SetOnTx(ice.onPubSubTx)
	ice.PubSub.SetOnConsensus(ice.onPubSubConsensus)

	// Setup consensus broadcast
	consensusMgr.SetBroadcastFunc(ice.broadcastConsensusMsg)

	// Log startup info
	iceLogger().Infow("Ice P2P network started",
		"peerID", h.ID().String(),
		"addresses", GetFullAddresses(h),
		"cAddr", ice.Address.CAddress.String(),
		"version", IceVersion,
	)

	// Update metrics
	metrics.SetPubSubTopicsJoined(3) // blocks, txs, consensus

	return ice, nil
}

// SetApiProvider sets the api provider, creates sync manager and handler with it, and starts sync.
func (ice *Ice) SetServiceProvider(provider service.ServiceProvider) {
	ice.mu.Lock()
	ice.ServiceProvider = provider

	// Consensus uses same provider for AddBlock/GetBlockByHash
	if ice.Consensus != nil {
		ice.Consensus.SetServiceProvider(provider)
	}

	// Handler with chain + txPool from same provider
	ice.Handler = protocol.NewHandler(ice.Host, provider, ice.cfg.NetCfg.ADDR, ice.cfg.GetVersion())
	ice.Handler.RegisterHandlers()

	// Sync manager using provider as chain
	ice.SyncManager = icesync.NewManager(ice.ctx, ice.Host, ice.Handler, ice.PeerManager, provider)
	ice.SyncManager.SetOnNewBlock(func(b *block.Block) {
		if b != nil && b.Head != nil {
			metrics.SetBlockHeight(b.Head.Height)
		}
	})
	ice.mu.Unlock()

	ice.SyncManager.Start()

	iceLogger().Infow("API provider set, sync manager and handler started")
}

// onPeerConnected handles new peer connections
func (ice *Ice) onPeerConnected(peerID peer.ID) {
	iceLogger().Infow("[ICE] Peer connected",
		"peerID", peerID,
		"devValidators", ice.devValidators,
	)
	metrics.RecordPeerConnected()

	// Request status from peer if sync manager is available
	if ice.SyncManager != nil {
		ice.SyncManager.HandleNewPeer(peerID)
	}

	// Auto-register as validator (dev-mode)
	if ice.devValidators && ice.Consensus != nil {
		ice.Consensus.AddValidator(peerID)
		metrics.SetValidatorCount(ice.Consensus.GetValidatorCount())
		iceLogger().Infow("[ICE] Peer registered as validator",
			"peerID", peerID,
			"validatorCount", ice.Consensus.GetValidatorCount(),
			"quorum", ice.Consensus.GetQuorum(),
		)
	}
}

// onPeerDisconnected handles peer disconnections
func (ice *Ice) onPeerDisconnected(peerID peer.ID) {
	iceLogger().Infow("[ICE] Peer disconnected",
		"peerID", peerID,
		"devValidators", ice.devValidators,
	)
	metrics.RecordPeerDisconnected()

	// Remove from validators (dev-mode)
	if ice.devValidators && ice.Consensus != nil {
		ice.Consensus.RemoveValidator(peerID)
		metrics.SetValidatorCount(ice.Consensus.GetValidatorCount())
		iceLogger().Infow("[ICE] Peer removed from validators",
			"peerID", peerID,
			"validatorCount", ice.Consensus.GetValidatorCount(),
			"quorum", ice.Consensus.GetQuorum(),
		)
	}
}

func envBool(key string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

// onPubSubBlock handles blocks received via PubSub
func (ice *Ice) onPubSubBlock(b *block.Block, from peer.ID) {
	metrics.RecordBlockReceived()
	metrics.RecordPubSubMessageReceived(TopicBlocks)

	if ice.SyncManager != nil {
		if err := ice.SyncManager.HandleNewBlock(b, from); err != nil {
			iceLogger().Warnw("Failed to handle pubsub block", "error", err, "from", from)
			metrics.RecordBlockRejected()
			ice.PeerScorer.RecordInvalidBlock(from)
		} else {
			iceLogger().Infow("[ICE] [CONSENSUS] Block from PubSub added to chain", "height", b.Head.Height, "hash", b.Hash.Hex(), "from", from)
			metrics.RecordBlockValidated()
			ice.PeerScorer.RecordValidBlock(from)
		}
	}
}

// onPubSubTx handles transactions received via PubSub
func (ice *Ice) onPubSubTx(tx *types.GTransaction, from peer.ID) {
	metrics.RecordTxReceived()
	metrics.RecordPubSubMessageReceived(TopicTxs)

	// if ice.ApiProvider != nil {
	// 	if err := ice.ApiProvider.AddTx(tx); err != nil {
	// 		iceLogger().Debugw("Failed to add tx to pool", "error", err, "from", from)
	// 		metrics.RecordTxRejected()
	// 		ice.PeerScorer.RecordInvalidTx(from)
	// 	} else {
	// 		metrics.RecordTxValidated()
	// 		ice.PeerScorer.RecordValidTx(from)
	// 	}
	// }
}

// onPubSubConsensus handles consensus messages received via PubSub
func (ice *Ice) onPubSubConsensus(consensusType int, data []byte, from peer.ID) {
	metrics.RecordPubSubMessageReceived(TopicConsensus)

	if ice.Consensus != nil {
		if err := ice.Consensus.HandleConsensusMessage(consensusType, data, from); err != nil {
			iceLogger().Warnw("Failed to handle consensus message", "error", err, "from", from)
		}
	}
}

// broadcastConsensusMsg broadcasts a consensus message
func (ice *Ice) broadcastConsensusMsg(consensusType int, data []byte, signature []byte) error {
	if ice.PubSub != nil {
		return ice.PubSub.BroadcastConsensus(consensusType, data, signature)
	}
	return fmt.Errorf("pubsub not initialized")
}

// BroadcastBlock broadcasts a new block to the network
func (ice *Ice) BroadcastBlock(b *block.Block) error {
	if ice.PubSub != nil {
		metrics.RecordBlockBroadcast()
		metrics.RecordPubSubMessagePublished(TopicBlocks)
		return ice.PubSub.BroadcastBlock(b)
	}
	return fmt.Errorf("pubsub not initialized")
}

// BroadcastTx broadcasts a new transaction to the network
func (ice *Ice) BroadcastTx(tx *types.GTransaction) error {
	if ice.PubSub != nil {
		metrics.RecordTxBroadcast()
		metrics.RecordPubSubMessagePublished(TopicTxs)
		return ice.PubSub.BroadcastTx(tx)
	}
	return fmt.Errorf("pubsub not initialized")
}

// ProposeBlock proposes a block for consensus
func (ice *Ice) ProposeBlock(b *block.Block) error {
	if ice.Consensus != nil {
		return ice.Consensus.ProposeBlock(b)
	}
	return fmt.Errorf("consensus not initialized")
}

// GetPeerCount returns the number of connected peers
func (ice *Ice) GetPeerCount() int {
	if ice.PeerManager != nil {
		return ice.PeerManager.GetPeerCount()
	}
	return 0
}

// GetPeers returns information about all connected peers
func (ice *Ice) GetPeers() []*peers.PeerInfo {
	if ice.PeerManager != nil {
		return ice.PeerManager.GetPeers()
	}
	return nil
}

// IsSyncing returns whether sync is in progress
func (ice *Ice) IsSyncing() bool {
	if ice.SyncManager != nil {
		return ice.SyncManager.IsSyncing()
	}
	return false
}

// GetSyncProgress returns the current sync progress
func (ice *Ice) GetSyncProgress() *icesync.SyncProgress {
	if ice.SyncManager != nil {
		progress := ice.SyncManager.GetProgress()
		return &progress
	}
	return nil
}

// ForceSync forces a sync with the best peer
func (ice *Ice) ForceSync() error {
	if ice.SyncManager != nil {
		return ice.SyncManager.ForceSync()
	}
	return fmt.Errorf("sync manager not initialized")
}

// GetConsensusStatus returns the current consensus status
func (ice *Ice) GetConsensusStatus() *consensus.ConsensusStatus {
	if ice.Consensus != nil {
		status := ice.Consensus.GetStatus()
		return &status
	}
	return nil
}

// Stop gracefully shuts down the Ice component
func (ice *Ice) Stop(ctx context.Context) {
	iceLogger().Infow("Stopping Ice P2P network...")

	// Stop consensus
	if ice.Consensus != nil {
		ice.Consensus.Stop()
	}

	// Stop sync manager
	if ice.SyncManager != nil {
		ice.SyncManager.Stop()
	}

	// Stop PubSub
	if ice.PubSub != nil {
		ice.PubSub.Stop()
	}

	// Stop discovery
	if ice.Discovery != nil {
		ice.Discovery.Stop()
	}

	// Stop peer manager
	if ice.PeerManager != nil {
		ice.PeerManager.Stop()
	}

	// Close host
	if ice.Host != nil {
		if err := ice.Host.Close(); err != nil {
			iceLogger().Warnw("Error closing host", "err", err)
		}
	}

	// Cancel context
	if ice.cancel != nil {
		ice.cancel()
	}

	iceLogger().Infow("Ice P2P network stopped")
}

// Status represents the current status of the Ice network component
type Status struct {
	Version      string                     `json:"version"`
	PeerID       string                     `json:"peerId"`
	Addresses    []string                   `json:"addresses"`
	PeerCount    int                        `json:"peerCount"`
	IsSyncing    bool                       `json:"isSyncing"`
	SyncProgress *icesync.SyncProgress      `json:"syncProgress,omitempty"`
	Consensus    *consensus.ConsensusStatus `json:"consensus,omitempty"`
	DHTTableSize int                        `json:"dhtTableSize"`
}

const ICE_SERVICE_NAME = "ICE_CERERA_001_1_0"

// ServiceName returns the service name for registry
func (ice *Ice) ServiceName() string {
	return ICE_SERVICE_NAME
}

// Exec executes a method on the Ice service
func (ice *Ice) Exec(method string, params []interface{}) interface{} {
	ice.mu.RLock()
	defer ice.mu.RUnlock()

	switch method {
	case "broadcastBlock":
		if len(params) > 0 {
			if b, ok := params[0].(*block.Block); ok {
				return ice.BroadcastBlock(b)
			}
		}
		return fmt.Errorf("invalid block parameter")
	case "proposeBlock":
		if len(params) > 0 {
			if b, ok := params[0].(*block.Block); ok {
				return ice.ProposeBlock(b)
			}
		}
		return fmt.Errorf("invalid block parameter")
	case "broadcastTx":
		if len(params) > 0 {
			if tx, ok := params[0].(*types.GTransaction); ok {
				return ice.BroadcastTx(tx)
			}
		}
		return fmt.Errorf("invalid tx parameter")
	case "isBootstrapReady":
		return ice.PeerManager != nil && ice.PeerManager.GetPeerCount() > 0
	case "isConsensusStarted":
		if ice.Consensus != nil {
			// Consensus is considered started if we have validators
			return ice.Consensus.GetValidatorCount() > 0
		}
		return false
	case "getPeerCount":
		return ice.GetPeerCount()
	case "getStatus":
		return ice.GetStatus()
	case "forceSync":
		return ice.ForceSync()
	}
	return nil
}

// GetStatus returns the current status of the Ice component
func (ice *Ice) GetStatus() *Status {
	status := &Status{
		Version:   IceVersion,
		PeerID:    ice.Host.ID().String(),
		Addresses: GetFullAddresses(ice.Host),
		PeerCount: ice.GetPeerCount(),
		IsSyncing: ice.IsSyncing(),
	}

	if ice.IsSyncing() {
		status.SyncProgress = ice.GetSyncProgress()
	}

	status.Consensus = ice.GetConsensusStatus()

	if ice.DHT != nil {
		status.DHTTableSize = ice.DHT.RoutingTable().Size()
		metrics.SetDHTRoutingTableSize(status.DHTTableSize)
	}

	// Update metrics
	metrics.PeersConnected.Set(float64(status.PeerCount))

	return status
}
