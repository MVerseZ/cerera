package gigea

import (
	"context"
	"fmt"
	"sync"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// ConsensusType represents the type of consensus algorithm
type ConsensusType int

const (
	ConsensusTypeSimple ConsensusType = iota
	ConsensusTypeCustom
)

func (ct ConsensusType) String() string {
	switch ct {
	case ConsensusTypeSimple:
		return "Simple"
	case ConsensusTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// ConsensusManager manages different consensus algorithms using the template pattern
type ConsensusManager struct {
	// Configuration
	ConsensusType ConsensusType
	NodeID        types.Address
	Peers         []*PeerInfo

	// Consensus algorithm instance
	consensus ConsensusAlgorithm

	// Network manager
	NetworkManager *NetworkManager

	// Engine reference
	engine *Engine

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc

	// State
	mu sync.RWMutex
}

// ConsensusEventHandlerImpl implements ConsensusEventHandler for the engine
type ConsensusEventHandlerImpl struct {
	engine *Engine
}

func (h *ConsensusEventHandlerImpl) OnLeaderElected(nodeID types.Address, term int64) {
	fmt.Printf("Leader elected: %s (term %d)\n", nodeID.Hex(), term)
}

func (h *ConsensusEventHandlerImpl) OnLeaderLost(nodeID types.Address, term int64) {
	fmt.Printf("Leader lost: %s (term %d)\n", nodeID.Hex(), term)
}

func (h *ConsensusEventHandlerImpl) OnRequestCommitted(operation string, result interface{}) {
	fmt.Printf("Request committed: %s -> %v\n", operation, result)
}

func (h *ConsensusEventHandlerImpl) OnPeerAdded(peer types.Address) {
	fmt.Printf("Peer added: %s\n", peer.Hex())
}

func (h *ConsensusEventHandlerImpl) OnPeerRemoved(peer types.Address) {
	fmt.Printf("Peer removed: %s\n", peer.Hex())
}

func (h *ConsensusEventHandlerImpl) OnConsensusError(err error) {
	fmt.Printf("Consensus error: %v\n", err)
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(consensusType ConsensusType, nodeID types.Address, peers []*PeerInfo, engine *Engine) *ConsensusManager {
	ctx, cancel := context.WithCancel(context.Background())

	cm := &ConsensusManager{
		ConsensusType: consensusType,
		NodeID:        nodeID,
		Peers:         peers,
		engine:        engine,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Create network manager
	port := engine.Port
	cm.NetworkManager = NewNetworkManager(nodeID, port)

	// Create consensus config with just Cerera addresses
	config := DefaultConsensusConfig(nodeID)
	cereraPeers := make([]types.Address, len(peers))
	for i, peer := range peers {
		cereraPeers[i] = peer.CereraAddress
	}
	config.Peers = cereraPeers

	// Initialize consensus algorithm based on type
	switch consensusType {
	case ConsensusTypeSimple:
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, engine)
	case ConsensusTypeCustom:
		// For custom consensus, you would implement your own ConsensusAlgorithm
		// For now, fallback to simple consensus
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, engine)
	default:
		// Default to simple consensus
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, engine)
	}

	// Set up event handler if the consensus supports it
	if baseNode, ok := cm.consensus.(*SimpleConsensus); ok {
		eventHandler := &ConsensusEventHandlerImpl{engine: engine}
		baseNode.SetEventHandler(eventHandler)
	}

	return cm
}

// Start starts the consensus manager
func (cm *ConsensusManager) Start() {
	fmt.Printf("Starting consensus manager with type: %s\n", cm.ConsensusType.String())

	// Start network manager first
	if err := cm.NetworkManager.Start(); err != nil {
		fmt.Printf("Failed to start network manager: %v\n", err)
		return
	}

	// Add peers to network manager
	for _, peer := range cm.Peers {
		if peer.CereraAddress != cm.NodeID {
			cm.NetworkManager.AddPeer(peer)
		}
	}

	// Start consensus algorithm
	if cm.consensus != nil {
		if err := cm.consensus.Start(cm.ctx); err != nil {
			fmt.Printf("Failed to start consensus algorithm: %v\n", err)
			return
		}
	}
}

// SubmitRequest submits a request to the consensus algorithm
func (cm *ConsensusManager) SubmitRequest(operation string) {
	if cm.consensus != nil {
		if err := cm.consensus.SubmitRequest(operation); err != nil {
			fmt.Printf("Failed to submit request: %v\n", err)
		}
	}
}

// GetConsensusState returns the current state of the consensus algorithm
func (cm *ConsensusManager) GetConsensusState() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.consensus != nil {
		return cm.consensus.GetConsensusInfo()
	}

	return map[string]interface{}{
		"error": "No consensus algorithm initialized",
	}
}

// IsLeader checks if this node is the leader
func (cm *ConsensusManager) IsLeader() bool {
	if cm.consensus != nil {
		return cm.consensus.IsLeader()
	}
	return false
}

// GetLeader returns the current leader address
func (cm *ConsensusManager) GetLeader() types.Address {
	if cm.consensus != nil {
		return cm.consensus.GetLeader()
	}
	return types.Address{}
}

// HandleBlock handles incoming blocks from the consensus algorithms
func (cm *ConsensusManager) HandleBlock(block *block.Block) {
	// Forward to engine
	if cm.engine != nil {
		cm.engine.BlockFunnel <- block
	}
}

// SwitchConsensus switches to a different consensus algorithm
func (cm *ConsensusManager) SwitchConsensus(newType ConsensusType) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	fmt.Printf("Switching consensus from %s to %s\n", cm.ConsensusType.String(), newType.String())

	// Stop current consensus
	if cm.consensus != nil {
		cm.consensus.Stop()
	}

	// Update consensus type
	cm.ConsensusType = newType

	// Create consensus config
	config := DefaultConsensusConfig(cm.NodeID)
	cereraPeers := make([]types.Address, len(cm.Peers))
	for i, peer := range cm.Peers {
		cereraPeers[i] = peer.CereraAddress
	}
	config.Peers = cereraPeers

	// Initialize new consensus algorithm
	switch newType {
	case ConsensusTypeSimple:
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, cm.engine)
	case ConsensusTypeCustom:
		// For custom consensus, you would implement your own ConsensusAlgorithm
		// For now, fallback to simple consensus
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, cm.engine)
	default:
		// Default to simple consensus
		cm.consensus = NewSimpleConsensus(config, cm.NetworkManager, cm.engine)
	}

	// Set up event handler if the consensus supports it
	if baseNode, ok := cm.consensus.(*SimpleConsensus); ok {
		eventHandler := &ConsensusEventHandlerImpl{engine: cm.engine}
		baseNode.SetEventHandler(eventHandler)
	}

	// Start new consensus
	if cm.consensus != nil {
		cm.consensus.Start(cm.ctx)
	}
}

// AddPeer adds a new peer to the consensus
func (cm *ConsensusManager) AddPeer(peerInfo *PeerInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if peer already exists
	for _, existingPeer := range cm.Peers {
		if existingPeer.CereraAddress == peerInfo.CereraAddress {
			return
		}
	}

	cm.Peers = append(cm.Peers, peerInfo)

	// Add to network manager
	if cm.NetworkManager != nil {
		cm.NetworkManager.AddPeer(peerInfo)
	}

	// Add to consensus algorithm
	if cm.consensus != nil {
		if err := cm.consensus.AddPeer(peerInfo.CereraAddress); err != nil {
			fmt.Printf("Failed to add peer %s to consensus: %v\n", peerInfo.CereraAddress.Hex(), err)
		}
	}
}

// RemovePeer removes a peer from the consensus
func (cm *ConsensusManager) RemovePeer(peer types.Address) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Find and remove peer
	for i, existingPeer := range cm.Peers {
		if existingPeer.CereraAddress == peer {
			cm.Peers = append(cm.Peers[:i], cm.Peers[i+1:]...)
			break
		}
	}

	// Remove from network manager
	if cm.NetworkManager != nil {
		cm.NetworkManager.RemovePeer(peer)
	}

	// Remove from consensus algorithm
	if cm.consensus != nil {
		if err := cm.consensus.RemovePeer(peer); err != nil {
			fmt.Printf("Failed to remove peer %s from consensus: %v\n", peer.Hex(), err)
		}
	}
}

// GetConsensusInfo returns detailed information about the consensus
func (cm *ConsensusManager) GetConsensusInfo() map[string]interface{} {
	baseInfo := map[string]interface{}{
		"manager_type": cm.ConsensusType.String(),
		"nodeID":       cm.NodeID.Hex(),
		"isLeader":     cm.IsLeader(),
		"leader":       cm.GetLeader().Hex(),
		"peerCount":    len(cm.Peers),
		"peers":        make([]string, len(cm.Peers)),
		"voters":       C.Voters,
	}

	// Add peer addresses
	for i, peer := range cm.Peers {
		baseInfo["peers"].([]string)[i] = peer.CereraAddress.Hex()
	}

	// Add consensus algorithm information
	if cm.consensus != nil {
		consensusInfo := cm.consensus.GetConsensusInfo()
		for k, v := range consensusInfo {
			baseInfo[k] = v
		}
	}

	return baseInfo
}

// GetNetworkInfo returns network information
func (cm *ConsensusManager) GetNetworkInfo() map[string]interface{} {
	if cm.NetworkManager == nil {
		return map[string]interface{}{
			"error": "Network manager not initialized",
		}
	}

	return map[string]interface{}{
		"nodeID":            cm.NodeID.Hex(),
		"listenAddr":        cm.NetworkManager.ListenAddr,
		"port":              cm.NetworkManager.Port,
		"isRunning":         cm.NetworkManager.IsRunning,
		"totalPeers":        len(cm.NetworkManager.GetPeers()),
		"connectedPeers":    len(cm.NetworkManager.GetConnectedPeers()),
		"peers":             cm.NetworkManager.GetPeers(),
		"connectedPeerList": cm.NetworkManager.GetConnectedPeers(),
	}
}
