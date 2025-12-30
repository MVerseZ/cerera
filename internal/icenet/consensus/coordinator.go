package consensus

import (
	"sync"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/metrics"
)

// Coordinator coordinates consensus operations
type Coordinator struct {
	mu       sync.RWMutex
	manager  Manager
	state    *ConsensusState
	isBootstrap bool
}

// NewCoordinator creates a new consensus coordinator
func NewCoordinator(manager Manager, isBootstrap bool) *Coordinator {
	return &Coordinator{
		manager:     manager,
		state:       NewConsensusState(),
		isBootstrap: isBootstrap,
	}
}

// GetManager returns the consensus manager
func (c *Coordinator) GetManager() Manager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manager
}

// GetState returns the consensus state
func (c *Coordinator) GetState() *ConsensusState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// IsBootstrap returns whether this is a bootstrap node
func (c *Coordinator) IsBootstrap() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isBootstrap
}

// Start starts the consensus coordinator
func (c *Coordinator) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.SetState(StateStarting)

	if c.isBootstrap {
		c.state.SetBootstrapReady()
		c.state.SetConsensusStarted()
		c.state.SetState(StateRunning)
		metrics.Get().UpdateConsensusStatus(1) // 1 = started
	} else {
		c.state.SetState(StateRunning)
		metrics.Get().UpdateConsensusStatus(1)
	}
}

// Stop stops the consensus coordinator
func (c *Coordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state.SetState(StateStopping)
	c.state.ResetBootstrapReady()
	c.state.ResetConsensusStarted()
	c.state.SetState(StateStopped)
	metrics.Get().UpdateConsensusStatus(0) // 0 = stopped
}

// AddNode adds a node to consensus
func (c *Coordinator) AddNode(addr types.Address, networkAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.manager.AddVoter(addr)
	c.manager.AddNode(addr, networkAddr)

	// Update metrics
	nodesMap := c.manager.GetNodes()
	metrics.Get().UpdateConsensusNodes(len(nodesMap))
	votersList := c.manager.GetVoters()
	metrics.Get().UpdateConsensusVoters(len(votersList))

	// Update nonce when adding a node (for bootstrap)
	if c.isBootstrap {
		oldNonce := c.manager.GetNonce()
		c.manager.SetNonce(oldNonce + 1)
		metrics.Get().UpdateConsensusNonce(c.manager.GetNonce())
	}

	return nil
}

// AddNodes adds multiple nodes to consensus
func (c *Coordinator) AddNodes(nodes []NodeInfo) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	addedCount := 0
	for _, node := range nodes {
		c.manager.AddVoter(node.Address)
		c.manager.AddNode(node.Address, node.NetworkAddr)
		addedCount++
	}

	// Update metrics
	nodesMap := c.manager.GetNodes()
	metrics.Get().UpdateConsensusNodes(len(nodesMap))
	votersList := c.manager.GetVoters()
	metrics.Get().UpdateConsensusVoters(len(votersList))

	// Update nonce when adding nodes (for bootstrap)
	if c.isBootstrap && addedCount > 0 {
		oldNonce := c.manager.GetNonce()
		c.manager.SetNonce(oldNonce + 1)
		metrics.Get().UpdateConsensusNonce(c.manager.GetNonce())
	}

	return addedCount
}

// NodeInfo represents information about a node
type NodeInfo struct {
	Address     types.Address
	NetworkAddr string
}

// UpdateNonce updates the consensus nonce
func (c *Coordinator) UpdateNonce(nonce uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manager.SetNonce(nonce)
}

// GetNonce returns the current nonce
func (c *Coordinator) GetNonce() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manager.GetNonce()
}

// AdvancePeriod advances to the next consensus period
func (c *Coordinator) AdvancePeriod(keepNodes []types.Address) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isBootstrap {
		return
	}

	oldNonce := c.manager.GetNonce()
	newNonce := oldNonce + 1
	c.manager.SetNonce(newNonce)
	metrics.Get().UpdateConsensusNonce(newNonce)

	c.state.ResetNodeConfirmations(keepNodes)
}

// CheckAllNodesConfirmed checks if all nodes have confirmed
func (c *Coordinator) CheckAllNodesConfirmed(connectedNodes map[types.Address]bool, minConfirmations int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isBootstrap {
		return false
	}

	confirmedCount := c.state.GetConfirmedNodesCount(minConfirmations, connectedNodes)
	return confirmedCount >= len(connectedNodes) && len(connectedNodes) > 0
}

// RecordNodeConfirmation records a node confirmation
func (c *Coordinator) RecordNodeConfirmation(addr types.Address) int {
	return c.state.IncrementNodeConfirmation(addr)
}

