package gigea

import (
	"fmt"
	"sync"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// ConsensusType represents the type of consensus algorithm
type ConsensusType int

const (
	ConsensusTypePBFT ConsensusType = iota
	ConsensusTypeRaft
	ConsensusTypeHybrid
)

func (ct ConsensusType) String() string {
	switch ct {
	case ConsensusTypePBFT:
		return "PBFT"
	case ConsensusTypeRaft:
		return "Raft"
	case ConsensusTypeHybrid:
		return "Hybrid"
	default:
		return "Unknown"
	}
}

// ConsensusManager manages different consensus algorithms
type ConsensusManager struct {
	// Configuration
	ConsensusType ConsensusType
	NodeID        types.Address
	Peers         []types.Address

	// Consensus instances
	PBFTNode *PBFTNode
	RaftNode *RaftNode

	// Engine reference
	engine *Engine

	// State
	mu sync.RWMutex
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(consensusType ConsensusType, nodeID types.Address, peers []types.Address, engine *Engine) *ConsensusManager {
	cm := &ConsensusManager{
		ConsensusType: consensusType,
		NodeID:        nodeID,
		Peers:         peers,
		engine:        engine,
	}

	// Initialize consensus algorithms based on type
	switch consensusType {
	case ConsensusTypePBFT:
		cm.PBFTNode = NewPBFTNode(nodeID, peers, engine)
	case ConsensusTypeRaft:
		cm.RaftNode = NewRaftNode(nodeID, peers, engine)
	case ConsensusTypeHybrid:
		cm.PBFTNode = NewPBFTNode(nodeID, peers, engine)
		cm.RaftNode = NewRaftNode(nodeID, peers, engine)
	}

	return cm
}

// Start starts the consensus manager
func (cm *ConsensusManager) Start() {
	fmt.Printf("Starting consensus manager with type: %s\n", cm.ConsensusType.String())

	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			cm.PBFTNode.Start()
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			cm.RaftNode.Start()
		}
	case ConsensusTypeHybrid:
		if cm.PBFTNode != nil {
			cm.PBFTNode.Start()
		}
		if cm.RaftNode != nil {
			cm.RaftNode.Start()
		}
	}
}

// SubmitRequest submits a request to the appropriate consensus algorithm
func (cm *ConsensusManager) SubmitRequest(operation string) {
	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			cm.PBFTNode.SubmitRequest(operation)
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			cm.RaftNode.SubmitRequest(operation)
		}
	case ConsensusTypeHybrid:
		// In hybrid mode, submit to both algorithms
		if cm.PBFTNode != nil {
			cm.PBFTNode.SubmitRequest(operation)
		}
		if cm.RaftNode != nil {
			cm.RaftNode.SubmitRequest(operation)
		}
	}
}

// GetConsensusState returns the current state of all consensus algorithms
func (cm *ConsensusManager) GetConsensusState() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	state := map[string]interface{}{
		"consensusType": cm.ConsensusType.String(),
		"nodeID":        cm.NodeID.Hex(),
		"peers":         len(cm.Peers),
	}

	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			state["pbft"] = cm.PBFTNode.GetConsensusState()
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			state["raft"] = cm.RaftNode.GetConsensusState()
		}
	case ConsensusTypeHybrid:
		if cm.PBFTNode != nil {
			state["pbft"] = cm.PBFTNode.GetConsensusState()
		}
		if cm.RaftNode != nil {
			state["raft"] = cm.RaftNode.GetConsensusState()
		}
	}

	return state
}

// IsLeader checks if this node is the leader in any consensus algorithm
func (cm *ConsensusManager) IsLeader() bool {
	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			return cm.PBFTNode.IsPrimary()
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			state := cm.RaftNode.GetConsensusState()
			if stateStr, ok := state["state"].(string); ok {
				return stateStr == "RaftLeader"
			}
		}
	case ConsensusTypeHybrid:
		// In hybrid mode, consider leader if either algorithm considers this node a leader
		pbftLeader := false
		raftLeader := false

		if cm.PBFTNode != nil {
			pbftLeader = cm.PBFTNode.IsPrimary()
		}
		if cm.RaftNode != nil {
			state := cm.RaftNode.GetConsensusState()
			if stateStr, ok := state["state"].(string); ok {
				raftLeader = stateStr == "RaftLeader"
			}
		}

		return pbftLeader || raftLeader
	}

	return false
}

// GetLeader returns the current leader address
func (cm *ConsensusManager) GetLeader() types.Address {
	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			return cm.PBFTNode.GetPrimary()
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			state := cm.RaftNode.GetConsensusState()
			if leaderID, ok := state["leaderID"].(*types.Address); ok && leaderID != nil {
				return *leaderID
			}
		}
	case ConsensusTypeHybrid:
		// In hybrid mode, return the PBFT primary if available, otherwise Raft leader
		if cm.PBFTNode != nil {
			return cm.PBFTNode.GetPrimary()
		}
		if cm.RaftNode != nil {
			state := cm.RaftNode.GetConsensusState()
			if leaderID, ok := state["leaderID"].(*types.Address); ok && leaderID != nil {
				return *leaderID
			}
		}
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
	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		// PBFT doesn't have a stop method, but we can clean up
	case ConsensusTypeRaft:
		// Raft doesn't have a stop method, but we can clean up
	case ConsensusTypeHybrid:
		// Clean up both
	}

	// Update consensus type
	cm.ConsensusType = newType

	// Initialize new consensus
	switch newType {
	case ConsensusTypePBFT:
		cm.PBFTNode = NewPBFTNode(cm.NodeID, cm.Peers, cm.engine)
		cm.RaftNode = nil
	case ConsensusTypeRaft:
		cm.RaftNode = NewRaftNode(cm.NodeID, cm.Peers, cm.engine)
		cm.PBFTNode = nil
	case ConsensusTypeHybrid:
		cm.PBFTNode = NewPBFTNode(cm.NodeID, cm.Peers, cm.engine)
		cm.RaftNode = NewRaftNode(cm.NodeID, cm.Peers, cm.engine)
	}

	// Start new consensus
	cm.Start()
}

// AddPeer adds a new peer to the consensus
func (cm *ConsensusManager) AddPeer(peer types.Address) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if peer already exists
	for _, existingPeer := range cm.Peers {
		if existingPeer == peer {
			return
		}
	}

	cm.Peers = append(cm.Peers, peer)

	// Update consensus algorithms
	if cm.PBFTNode != nil {
		// For PBFT, we would need to update the replicas list
		// This is a simplified implementation
		fmt.Printf("Added peer %s to PBFT consensus\n", peer.Hex())
	}

	if cm.RaftNode != nil {
		// For Raft, we would need to update the peers list
		// This is a simplified implementation
		fmt.Printf("Added peer %s to Raft consensus\n", peer.Hex())
	}
}

// RemovePeer removes a peer from the consensus
func (cm *ConsensusManager) RemovePeer(peer types.Address) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Find and remove peer
	for i, existingPeer := range cm.Peers {
		if existingPeer == peer {
			cm.Peers = append(cm.Peers[:i], cm.Peers[i+1:]...)
			break
		}
	}

	// Update consensus algorithms
	if cm.PBFTNode != nil {
		fmt.Printf("Removed peer %s from PBFT consensus\n", peer.Hex())
	}

	if cm.RaftNode != nil {
		fmt.Printf("Removed peer %s from Raft consensus\n", peer.Hex())
	}
}

// GetConsensusInfo returns detailed information about the consensus
func (cm *ConsensusManager) GetConsensusInfo() map[string]interface{} {
	info := map[string]interface{}{
		"type":      cm.ConsensusType.String(),
		"nodeID":    cm.NodeID.Hex(),
		"isLeader":  cm.IsLeader(),
		"leader":    cm.GetLeader().Hex(),
		"peerCount": len(cm.Peers),
		"peers":     make([]string, len(cm.Peers)),
	}

	// Add peer addresses
	for i, peer := range cm.Peers {
		info["peers"].([]string)[i] = peer.Hex()
	}

	// Add algorithm-specific information
	switch cm.ConsensusType {
	case ConsensusTypePBFT:
		if cm.PBFTNode != nil {
			info["pbft"] = cm.PBFTNode.GetConsensusState()
		}
	case ConsensusTypeRaft:
		if cm.RaftNode != nil {
			info["raft"] = cm.RaftNode.GetConsensusState()
		}
	case ConsensusTypeHybrid:
		if cm.PBFTNode != nil {
			info["pbft"] = cm.PBFTNode.GetConsensusState()
		}
		if cm.RaftNode != nil {
			info["raft"] = cm.RaftNode.GetConsensusState()
		}
	}

	return info
}
