package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/peers"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/message"
	"github.com/cerera/internal/service"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

func consensusLogger() *zap.SugaredLogger {
	return logger.Named("consensus")
}

// Manager manages the hybrid consensus process
type Manager struct {
	host        host.Host
	peerManager *peers.Manager
	peerScorer  *peers.Scorer
	voting      *VotingManager

	serviceProvider service.ServiceProvider

	blocksMu sync.RWMutex
	blocks   map[common.Hash]*block.Block

	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	currentView int64
	sequenceID  int64

	// Callbacks
	onBlockFinalized func(*block.Block)
	broadcastMsg     func(int, []byte, []byte) error
}

// NewManager creates a new consensus manager
func NewManager(
	ctx context.Context,
	h host.Host,
	peerManager *peers.Manager,
	peerScorer *peers.Scorer,
	localPeerAddr types.Address,
	serviceProvider service.ServiceProvider,
) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	voting := NewVotingManager(ctx, h.ID(), localPeerAddr)
	m := &Manager{
		host:            h,
		peerManager:     peerManager,
		peerScorer:      peerScorer,
		voting:          voting,
		serviceProvider: serviceProvider,
		ctx:             ctx,
		cancel:          cancel,
		currentView:     0,
		sequenceID:      0,
		blocks:          make(map[common.Hash]*block.Block),
	}

	// Set voting callbacks
	voting.SetOnPrepareQuorum(m.handlePrepareQuorum)
	voting.SetOnCommitQuorum(m.handleCommitQuorum)
	voting.SetOnRoundTimeout(m.handleRoundTimeout)
	voting.SetBroadcastVote(m.broadcastVote)

	return m
}

// Start starts the consensus manager
func (m *Manager) Start() {
	m.voting.Start()
	metrics.SetConsensusStatus(1)
	m.startConsensusMetricsUpdater()
	consensusLogger().Infow("Consensus manager started")
}

// Stop stops the consensus manager
func (m *Manager) Stop() {
	m.cancel()
	m.voting.Stop()
	metrics.SetConsensusStatus(0)
	metrics.SetConsensusNodesTotal(0)
	metrics.SetConsensusVotersTotal(0)
	metrics.SetConsensusNonce(0)
	consensusLogger().Infow("Consensus manager stopped")
}

func (m *Manager) SetServiceProvider(serviceProvider service.ServiceProvider) {
	m.serviceProvider = serviceProvider
}

// startConsensusMetricsUpdater runs a goroutine that periodically updates
// icenet_consensus_* metrics for Grafana (status, nodes_total, voters_total, nonce).
func (m *Manager) startConsensusMetricsUpdater() {
	update := func() {
		st := m.GetStatus()
		metrics.SetConsensusNodesTotal(st.ValidatorCount)
		metrics.SetConsensusVotersTotal(st.ValidatorCount)
		metrics.SetConsensusNonce(st.SequenceID)
		metrics.SetValidatorCount(st.ValidatorCount)
	}
	update() // initial update
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				update()
			}
		}
	}()
}

// ProposeBlock proposes a new block for consensus
func (m *Manager) ProposeBlock(b *block.Block) error {
	consensusLogger().Debugw("[CONSENSUS] ProposeBlock called",
		"hash", b.Hash,
		"height", b.Head.Height,
		"validatorCount", m.voting.GetValidatorSet().Size(),
		"quorum", m.voting.GetValidatorSet().Quorum(),
	)

	// First, validate PoW if validator is set

	if m.serviceProvider != nil {
		serviceProvider := m.serviceProvider
		// serviceProvider.Exec("validateBlockPoW", []interface{}{b})
		err := serviceProvider.ValidateBlock(b)
		if err != nil {
			return fmt.Errorf("block validation failed: %v", err)
		}
		consensusLogger().Debugw("[CONSENSUS] Block validation passed",
			"hash", b.Hash,
			"height", b.Head.Height,
		)
	}

	// if m.validator != nil {
	// 	if !m.validator.ValidateBlockPoW(b) {
	// 		consensusLogger().Warnw("[CONSENSUS] Block failed PoW validation",
	// 			"hash", b.Hash,
	// 			"height", b.Head.Height,
	// 		)
	// 		return fmt.Errorf("block failed PoW validation")
	// 	}

	// 	if err := m.validator.ValidateBlock(b); err != nil {
	// 		consensusLogger().Warnw("[CONSENSUS] Block validation failed",
	// 			"hash", b.Hash,
	// 			"height", b.Head.Height,
	// 			"error", err,
	// 		)
	// 		return fmt.Errorf("block validation failed: %w", err)
	// 	}
	// 	consensusLogger().Debugw("[CONSENSUS] Block validation passed",
	// 		"hash", b.Hash,
	// 		"height", b.Head.Height,
	// 	)
	// }

	// Cache proposed block for later finalization.
	m.putBlock(b.Hash, b)

	m.mu.Lock()
	m.sequenceID++
	seqID := m.sequenceID
	viewID := m.currentView
	m.mu.Unlock()

	consensusLogger().Infow("[CONSENSUS] Starting consensus round",
		"hash", b.Hash,
		"height", b.Head.Height,
		"viewID", viewID,
		"seqID", seqID,
		"validatorCount", m.voting.GetValidatorSet().Size(),
		"quorum", m.voting.GetValidatorSet().Quorum(),
	)

	// Start consensus round
	if err := m.voting.StartRound(b, viewID, seqID); err != nil {
		metrics.RecordConsensusRoundFailed()
		consensusLogger().Errorw("[CONSENSUS] Failed to start consensus round",
			"hash", b.Hash,
			"height", b.Head.Height,
			"error", err,
		)
		return fmt.Errorf("failed to start consensus round: %w", err)
	}

	metrics.RecordConsensusRoundStarted()
	consensusLogger().Infow("[CONSENSUS] Block proposed for consensus",
		"hash", b.Hash,
		"height", b.Head.Height,
		"viewID", viewID,
		"seqID", seqID,
	)

	return nil
}

// HandleConsensusMessage handles incoming consensus messages
func (m *Manager) HandleConsensusMessage(msgType int, data []byte, from peer.ID) error {
	mType := message.MType(msgType)

	switch mType {
	case message.MTPrePrepare:
		return m.handlePrePrepare(data, from)
	case message.MTPrepare:
		return m.handlePrepare(data, from)
	case message.MTCommit:
		return m.handleCommit(data, from)
	case message.MTViewChange:
		return m.handleViewChange(data, from)
	default:
		return fmt.Errorf("unknown consensus message type: %d", msgType)
	}
}

// handlePrePrepare handles pre-prepare messages
func (m *Manager) handlePrePrepare(data []byte, from peer.ID) error {
	msg, err := UnmarshalVotingMessage(data)
	if err != nil {
		consensusLogger().Warnw("[CONSENSUS] Failed to unmarshal pre-prepare",
			"from", from,
			"error", err,
		)
		return fmt.Errorf("failed to unmarshal pre-prepare: %w", err)
	}

	consensusLogger().Debugw("[CONSENSUS] Received PrePrepare",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"viewID", msg.ViewID,
		"seqID", msg.SequenceID,
		"voterID", msg.VoterID,
	)

	if err := m.verifyVotingMessage(msg, from); err != nil {
		consensusLogger().Warnw("[CONSENSUS] PrePrepare signature verification failed",
			"from", from,
			"blockHash", msg.BlockHash,
			"height", msg.BlockHeight,
			"error", err,
		)
		if m.peerScorer != nil {
			m.peerScorer.RecordMisbehavior(from)
		}
		return err
	}

	// Ensure block is present and valid before voting.
	if msg.Block == nil {
		consensusLogger().Warnw("[CONSENSUS] PrePrepare missing block payload",
			"from", from,
			"blockHash", msg.BlockHash,
		)
		if m.peerScorer != nil {
			m.peerScorer.RecordMisbehavior(from)
		}
		return fmt.Errorf("pre-prepare missing block payload")
	}
	if msg.Block.Hash != msg.BlockHash || msg.Block.Head == nil || msg.Block.Head.Height != msg.BlockHeight {
		consensusLogger().Warnw("[CONSENSUS] PrePrepare block mismatch",
			"from", from,
			"blockHash", msg.BlockHash,
			"blockHeight", msg.BlockHeight,
			"actualHash", msg.Block.Hash,
			"actualHeight", func() int {
				if msg.Block.Head != nil {
					return msg.Block.Head.Height
				}
				return -1
			}(),
		)
		if m.peerScorer != nil {
			m.peerScorer.RecordMisbehavior(from)
		}
		return fmt.Errorf("pre-prepare block mismatch")
	}
	if m.serviceProvider != nil {
		if !m.serviceProvider.ValidateBlockPoW(msg.Block) {
			consensusLogger().Warnw("[CONSENSUS] PrePrepare block failed PoW",
				"from", from,
				"blockHash", msg.BlockHash,
			)
			if m.peerScorer != nil {
				m.peerScorer.RecordMisbehavior(from)
			}
			return fmt.Errorf("pre-prepare block failed PoW validation")
		}
		if err := m.serviceProvider.ValidateBlock(msg.Block); err != nil {
			consensusLogger().Warnw("[CONSENSUS] PrePrepare block validation failed",
				"from", from,
				"blockHash", msg.BlockHash,
				"error", err,
			)
			if m.peerScorer != nil {
				m.peerScorer.RecordMisbehavior(from)
			}
			return fmt.Errorf("pre-prepare block validation failed: %w", err)
		}
	}

	// Cache the block for later finalization.
	m.putBlock(msg.BlockHash, msg.Block)

	// Record consensus participation for scoring
	if m.peerScorer != nil {
		m.peerScorer.RecordConsensusHelp(from)
	}

	// Ensure voter address exists in vault (with 0 balance) so /account shows all nodes
	if v := storage.GetVault(); v != nil {
		v.EnsureAccount(msg.VoterAddr)
	}

	consensusLogger().Infow("[CONSENSUS] Processing PrePrepare",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"viewID", msg.ViewID,
		"seqID", msg.SequenceID,
	)

	// Use the logical voter ID (origin) for consensus semantics.
	return m.voting.HandlePrePrepare(msg, msg.VoterID)
}

// handlePrepare handles prepare messages
func (m *Manager) handlePrepare(data []byte, from peer.ID) error {
	msg, err := UnmarshalVotingMessage(data)
	if err != nil {
		consensusLogger().Warnw("[CONSENSUS] Failed to unmarshal prepare",
			"from", from,
			"error", err,
		)
		return fmt.Errorf("failed to unmarshal prepare: %w", err)
	}

	consensusLogger().Debugw("[CONSENSUS] Received Prepare",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"viewID", msg.ViewID,
		"seqID", msg.SequenceID,
		"voteType", msg.VoteType,
	)

	if err := m.verifyVotingMessage(msg, from); err != nil {
		consensusLogger().Warnw("[CONSENSUS] Prepare signature verification failed",
			"from", from,
			"blockHash", msg.BlockHash,
			"error", err,
		)
		if m.peerScorer != nil {
			m.peerScorer.RecordMisbehavior(from)
		}
		return err
	}

	if m.peerScorer != nil {
		m.peerScorer.RecordConsensusHelp(from)
	}

	if v := storage.GetVault(); v != nil {
		v.EnsureAccount(msg.VoterAddr)
	}

	// Use the logical voter ID (origin) for consensus semantics.
	return m.voting.HandlePrepare(msg, msg.VoterID)
}

// handleCommit handles commit messages
func (m *Manager) handleCommit(data []byte, from peer.ID) error {
	msg, err := UnmarshalVotingMessage(data)
	if err != nil {
		consensusLogger().Warnw("[CONSENSUS] Failed to unmarshal commit",
			"from", from,
			"error", err,
		)
		return fmt.Errorf("failed to unmarshal commit: %w", err)
	}

	consensusLogger().Debugw("[CONSENSUS] Received Commit",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"viewID", msg.ViewID,
		"seqID", msg.SequenceID,
		"voteType", msg.VoteType,
	)

	if err := m.verifyVotingMessage(msg, from); err != nil {
		consensusLogger().Warnw("[CONSENSUS] Commit signature verification failed",
			"from", from,
			"blockHash", msg.BlockHash,
			"error", err,
		)
		if m.peerScorer != nil {
			m.peerScorer.RecordMisbehavior(from)
		}
		return err
	}

	if m.peerScorer != nil {
		m.peerScorer.RecordConsensusHelp(from)
	}

	if v := storage.GetVault(); v != nil {
		v.EnsureAccount(msg.VoterAddr)
	}

	// Use the logical voter ID (origin) for consensus semantics.
	return m.voting.HandleCommit(msg, msg.VoterID)
}

// handleViewChange handles view change messages
func (m *Manager) handleViewChange(data []byte, from peer.ID) error {
	var vcMsg message.ViewChange
	if err := json.Unmarshal(data, &vcMsg); err != nil {
		return fmt.Errorf("failed to unmarshal view change: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update view if the new view is higher
	if vcMsg.NewViewID > m.currentView {
		consensusLogger().Infow("View change",
			"oldView", m.currentView,
			"newView", vcMsg.NewViewID,
			"from", from,
		)
		m.currentView = vcMsg.NewViewID
	}

	return nil
}

// handlePrepareQuorum is called when prepare quorum is reached
func (m *Manager) handlePrepareQuorum(blockHash common.Hash, height int) {
	round := m.voting.GetCurrentRound()
	prepareCount := 0
	if round != nil {
		prepareCount = round.GetPrepareVoteCount()
		metrics.SetPrepareVotes(prepareCount)
	}
	consensusLogger().Infow("[CONSENSUS] Prepare quorum reached",
		"blockHash", blockHash,
		"height", height,
		"prepareVotes", prepareCount,
		"quorum", func() int {
			if round != nil {
				return round.Quorum
			}
			return m.voting.GetValidatorSet().Quorum()
		}(),
	)
}

// handleCommitQuorum is called when commit quorum is reached
func (m *Manager) handleCommitQuorum(blockHash common.Hash, height int) {
	round := m.voting.GetCurrentRound()
	commitCount := 0
	if round != nil {
		commitCount = round.GetCommitVoteCount()
		metrics.SetCommitVotes(commitCount)
	}
	consensusLogger().Infow("[CONSENSUS] Commit quorum reached - finalizing block",
		"blockHash", blockHash,
		"height", height,
		"commitVotes", commitCount,
		"quorum", func() int {
			if round != nil {
				return round.Quorum
			}
			return m.voting.GetValidatorSet().Quorum()
		}(),
	)

	// Retrieve block (prefer in-memory cache; fall back to chain lookup if present).
	b := m.getBlock(blockHash)
	if b == nil && m.serviceProvider != nil {
		consensusLogger().Debugw("[CONSENSUS] Block not in cache, fetching from chain",
			"blockHash", blockHash,
		)
		b = m.serviceProvider.GetBlockByHash(blockHash)
	}
	if b == nil {
		consensusLogger().Warnw("[CONSENSUS] Finalized block not found locally",
			"blockHash", blockHash,
			"height", height,
		)
		return
	}

	consensusLogger().Debugw("[CONSENSUS] Validating finalized block",
		"blockHash", blockHash,
		"height", height,
	)

	// Validate again before adding.
	if m.serviceProvider != nil {
		if !m.serviceProvider.ValidateBlockPoW(b) {
			consensusLogger().Warnw("[CONSENSUS] Finalized block failed PoW validation",
				"blockHash", blockHash,
				"height", height,
			)
			return
		}
		if err := m.serviceProvider.ValidateBlock(b); err != nil {
			consensusLogger().Warnw("[CONSENSUS] Finalized block failed validation",
				"error", err,
				"blockHash", blockHash,
				"height", height,
			)
			return
		}
	}

	// Add to chain if available.
	if m.serviceProvider != nil {
		consensusLogger().Infow("[CONSENSUS] Adding finalized block to chain",
			"blockHash", blockHash,
			"height", height,
		)
		if err := m.serviceProvider.AddBlock(b); err != nil {
			metrics.RecordConsensusRoundFailed()
			consensusLogger().Errorw("[CONSENSUS] Failed to add finalized block to chain",
				"error", err,
				"blockHash", blockHash,
				"height", height,
			)
			return
		}
		metrics.RecordConsensusRoundCompleted()
		consensusLogger().Infow("[CONSENSUS] Block successfully added to chain",
			"blockHash", blockHash,
			"height", height,
		)
	} else {
		consensusLogger().Warnw("[CONSENSUS] Chain provider not set; cannot add block",
			"blockHash", blockHash,
			"height", height,
		)
	}

	// Notify.
	if m.onBlockFinalized != nil {
		m.onBlockFinalized(b)
	}

	// Cleanup cache entry and reset vote gauges for next round
	m.deleteBlock(blockHash)
	metrics.SetPrepareVotes(0)
	metrics.SetCommitVotes(0)
}

func (m *Manager) handleRoundTimeout(key RoundKey, blockHash common.Hash) {
	metrics.RecordConsensusRoundFailed()
	consensusLogger().Warnw("[CONSENSUS] Round timeout - initiating view change",
		"blockHash", blockHash,
		"height", key.Height,
		"viewID", key.ViewID,
		"seqID", key.SequenceID,
		"currentView", m.GetCurrentView(),
	)

	// Minimal view-change: bump view and broadcast.
	newView := m.GetCurrentView() + 1
	consensusLogger().Infow("[CONSENSUS] Requesting view change",
		"oldView", m.GetCurrentView(),
		"newView", newView,
	)
	if err := m.RequestViewChange(newView); err != nil {
		consensusLogger().Errorw("[CONSENSUS] Failed to request view change",
			"error", err,
			"newView", newView,
		)
	} else {
		consensusLogger().Infow("[CONSENSUS] View change requested successfully",
			"newView", newView,
		)
	}
}

// broadcastVote broadcasts a voting message
func (m *Manager) broadcastVote(msg *VotingMessage) error {
	// Ensure the message is signed before broadcasting.
	if err := m.signVotingMessage(msg); err != nil {
		return fmt.Errorf("failed to sign vote: %w", err)
	}

	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal vote: %w", err)
	}

	if m.broadcastMsg != nil {
		return m.broadcastMsg(int(msg.Type), data, nil)
	}

	return nil
}

func (m *Manager) signVotingMessage(msg *VotingMessage) error {
	if msg == nil {
		return fmt.Errorf("nil voting message")
	}
	// Force voter ID to self for outbound messages.
	msg.VoterID = m.host.ID()

	priv := m.host.Peerstore().PrivKey(m.host.ID())
	if priv == nil {
		return fmt.Errorf("missing private key for local peer")
	}

	payload, err := msg.SignBytes()
	if err != nil {
		return err
	}

	sig, err := priv.Sign(payload)
	if err != nil {
		return err
	}
	msg.Signature = sig

	if privKey, pubKey, err := storage.GetKeys(); err == nil && privKey != nil && pubKey != nil {
		fmt.Printf("Get keys: \t%s\r\n\t%s\r\n", privKey.B58Serialize(), pubKey.B58Serialize())
	}
	if msg.Type == message.MTPrePrepare {
		fmt.Printf("Block header: Node: \t%+s\r\n", msg.Block.Head.Node)
	}
	fmt.Printf("Voter ID: \t%s\r\n", msg.VoterID)
	fmt.Printf("Voter Address: \t%s\r\n", msg.VoterAddr)
	fmt.Printf("Message signature: \t%x\r\n", msg.Signature)
	// fmt.Printf("Block header: Node: \t%+s\r\n", msg.Block.Head.Node)
	return nil
}

func (m *Manager) verifyVotingMessage(msg *VotingMessage, from peer.ID) error {
	if msg == nil {
		return fmt.Errorf("nil voting message")
	}
	if msg.VoterID == "" {
		return fmt.Errorf("missing voter ID")
	}
	if len(msg.Signature) == 0 {
		return fmt.Errorf("missing signature")
	}

	// Always verify against the logical voter (message origin), not the relay peer.
	voterID := msg.VoterID

	pub := m.host.Peerstore().PubKey(voterID)
	if pub == nil {
		// Attempt extraction for inline public keys.
		if extracted, err := voterID.ExtractPublicKey(); err == nil && extracted != nil {
			pub = extracted
			m.host.Peerstore().AddPubKey(voterID, pub)
		}
	}
	if pub == nil {
		return fmt.Errorf("missing public key for peer %s", voterID)
	}

	payload, err := msg.SignBytes()
	if err != nil {
		return err
	}
	ok, err := pub.Verify(payload, msg.Signature)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid signature")
	}

	// If the transport relay differs from the logical voter, log it for diagnostics
	// but do not treat it as an error (this is normal for GossipSub).
	if from != "" && from != voterID {
		consensusLogger().Debugw("[CONSENSUS] Voting message relayed by different peer",
			"voterID", voterID,
			"relayFrom", from,
		)
	}

	// Best-effort: ensure peerstore has the pubkey cached for later.
	m.host.Peerstore().AddPubKey(voterID, pub)

	return nil
}

// AddValidator adds a validator to the set
func (m *Manager) AddValidator(peerID peer.ID) {
	m.voting.GetValidatorSet().AddValidator(peerID)
	newCount := m.voting.GetValidatorSet().Size()
	newQuorum := m.voting.GetValidatorSet().Quorum()
	consensusLogger().Infow("[CONSENSUS] Validator added",
		"peer", peerID,
		"validatorCount", newCount,
		"quorum", newQuorum,
	)
}

// RemoveValidator removes a validator from the set
func (m *Manager) RemoveValidator(peerID peer.ID) {
	m.voting.GetValidatorSet().RemoveValidator(peerID)
	newCount := m.voting.GetValidatorSet().Size()
	newQuorum := m.voting.GetValidatorSet().Quorum()
	consensusLogger().Infow("[CONSENSUS] Validator removed",
		"peer", peerID,
		"validatorCount", newCount,
		"quorum", newQuorum,
	)
}

// IsValidator checks if a peer is a validator
func (m *Manager) IsValidator(peerID peer.ID) bool {
	return m.voting.GetValidatorSet().IsValidator(peerID)
}

// GetValidatorCount returns the number of validators
func (m *Manager) GetValidatorCount() int {
	return m.voting.GetValidatorSet().Size()
}

// GetQuorum returns the required quorum size
func (m *Manager) GetQuorum() int {
	return m.voting.GetValidatorSet().Quorum()
}

// GetCurrentView returns the current view ID
func (m *Manager) GetCurrentView() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentView
}

// GetSequenceID returns the current sequence ID
func (m *Manager) GetSequenceID() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sequenceID
}

// SetOnBlockFinalized sets the callback for finalized blocks
func (m *Manager) SetOnBlockFinalized(callback func(*block.Block)) {
	m.onBlockFinalized = callback
}

// SetBroadcastFunc sets the function for broadcasting consensus messages
func (m *Manager) SetBroadcastFunc(broadcast func(int, []byte, []byte) error) {
	m.broadcastMsg = broadcast
}

// GetCurrentRound returns the current consensus round state
func (m *Manager) GetCurrentRound() *RoundState {
	return m.voting.GetCurrentRound()
}

// RequestViewChange initiates a view change
func (m *Manager) RequestViewChange(newView int64) error {
	m.mu.Lock()
	if newView <= m.currentView {
		m.mu.Unlock()
		return fmt.Errorf("new view must be greater than current view")
	}
	m.mu.Unlock()

	vcMsg := &message.ViewChange{
		NewViewID: newView,
		NodeID:    0, // Would need actual node ID
		CMsg:      make(map[int64]*message.CheckPoint),
		PMsg:      make(map[int64]*message.PTuple),
	}

	data, err := json.Marshal(vcMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal view change: %w", err)
	}

	if m.broadcastMsg != nil {
		return m.broadcastMsg(int(message.MTViewChange), data, nil)
	}

	return nil
}

// Status returns the current consensus status
type ConsensusStatus struct {
	CurrentView    int64          `json:"currentView"`
	SequenceID     int64          `json:"sequenceId"`
	ValidatorCount int            `json:"validatorCount"`
	Quorum         int            `json:"quorum"`
	IsInRound      bool           `json:"isInRound"`
	RoundState     ConsensusState `json:"roundState,omitempty"`
	RoundBlockHash string         `json:"roundBlockHash,omitempty"`
}

// GetStatus returns the current consensus status
func (m *Manager) GetStatus() ConsensusStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ConsensusStatus{
		CurrentView:    m.currentView,
		SequenceID:     m.sequenceID,
		ValidatorCount: m.voting.GetValidatorSet().Size(),
		Quorum:         m.voting.GetValidatorSet().Quorum(),
	}

	round := m.voting.GetCurrentRound()
	if round != nil {
		status.IsInRound = true
		status.RoundState = round.GetState()
		status.RoundBlockHash = round.BlockHash.String()
	}

	return status
}

// AutoRegisterValidators automatically registers connected peers as validators
func (m *Manager) AutoRegisterValidators() {
	peers := m.peerManager.GetPeerIDs()
	for _, peerID := range peers {
		if !m.IsValidator(peerID) {
			m.AddValidator(peerID)
		}
	}

	// Also add self
	if !m.IsValidator(m.host.ID()) {
		m.AddValidator(m.host.ID())
	}
}

// MonitorValidators monitors validator status
func (m *Manager) MonitorValidators() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Update validator set based on connected peers
			m.AutoRegisterValidators()
		}
	}
}

func (m *Manager) putBlock(hash common.Hash, b *block.Block) {
	if b == nil {
		return
	}
	m.blocksMu.Lock()
	m.blocks[hash] = b
	m.blocksMu.Unlock()
}

func (m *Manager) getBlock(hash common.Hash) *block.Block {
	m.blocksMu.RLock()
	b := m.blocks[hash]
	m.blocksMu.RUnlock()
	return b
}

func (m *Manager) deleteBlock(hash common.Hash) {
	m.blocksMu.Lock()
	delete(m.blocks, hash)
	m.blocksMu.Unlock()
}
