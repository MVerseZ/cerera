package consensus

import (
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// State represents the consensus state
type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

// ConsensusState manages the consensus state
type ConsensusState struct {
	mu                sync.RWMutex
	state             State
	bootstrapReady    bool
	consensusStarted  bool
	readyChan         chan struct{}
	readyOnce         sync.Once
	consensusChan     chan struct{}
	consensusOnce     sync.Once
	confirmedNodes    map[types.Address]int
	lastStateChange   time.Time
}

// NewConsensusState creates a new consensus state
func NewConsensusState() *ConsensusState {
	return &ConsensusState{
		state:          StateStopped,
		bootstrapReady: false,
		consensusStarted: false,
		readyChan:      make(chan struct{}),
		consensusChan:  make(chan struct{}),
		confirmedNodes: make(map[types.Address]int),
	}
}

// GetState returns the current state
func (cs *ConsensusState) GetState() State {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.state
}

// SetState sets the consensus state
func (cs *ConsensusState) SetState(state State) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.state = state
	cs.lastStateChange = time.Now()
}

// IsBootstrapReady returns whether bootstrap is ready
func (cs *ConsensusState) IsBootstrapReady() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.bootstrapReady
}

// SetBootstrapReady sets the bootstrap ready flag
func (cs *ConsensusState) SetBootstrapReady() {
	cs.mu.Lock()
	wasReady := cs.bootstrapReady
	cs.bootstrapReady = true
	readyChan := cs.readyChan
	cs.mu.Unlock()

	if !wasReady {
		cs.readyOnce.Do(func() {
			close(readyChan)
		})
	}
}

// ResetBootstrapReady resets the bootstrap ready flag
func (cs *ConsensusState) ResetBootstrapReady() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.bootstrapReady {
		cs.bootstrapReady = false
		cs.readyChan = make(chan struct{})
		cs.readyOnce = sync.Once{}
	}
}

// WaitForBootstrapReady waits for bootstrap to be ready
func (cs *ConsensusState) WaitForBootstrapReady() {
	if cs.IsBootstrapReady() {
		return
	}

	cs.mu.RLock()
	readyChan := cs.readyChan
	cs.mu.RUnlock()

	<-readyChan
}

// IsConsensusStarted returns whether consensus has started
func (cs *ConsensusState) IsConsensusStarted() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.consensusStarted
}

// SetConsensusStarted sets the consensus started flag
func (cs *ConsensusState) SetConsensusStarted() {
	cs.mu.Lock()
	wasStarted := cs.consensusStarted
	cs.consensusStarted = true
	consensusChan := cs.consensusChan
	cs.mu.Unlock()

	if !wasStarted {
		cs.consensusOnce.Do(func() {
			close(consensusChan)
		})
	}
}

// ResetConsensusStarted resets the consensus started flag
func (cs *ConsensusState) ResetConsensusStarted() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.consensusStarted {
		cs.consensusStarted = false
		cs.consensusChan = make(chan struct{})
		cs.consensusOnce = sync.Once{}
	}
}

// WaitForConsensus waits for consensus to start
func (cs *ConsensusState) WaitForConsensus() {
	if cs.IsConsensusStarted() {
		return
	}

	cs.mu.RLock()
	consensusChan := cs.consensusChan
	cs.mu.RUnlock()

	<-consensusChan
}

// IncrementNodeConfirmation increments the confirmation count for a node
func (cs *ConsensusState) IncrementNodeConfirmation(addr types.Address) int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.confirmedNodes[addr]++
	return cs.confirmedNodes[addr]
}

// GetNodeConfirmation returns the confirmation count for a node
func (cs *ConsensusState) GetNodeConfirmation(addr types.Address) int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.confirmedNodes[addr]
}

// ResetNodeConfirmations resets all node confirmations
func (cs *ConsensusState) ResetNodeConfirmations(keepNodes []types.Address) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.confirmedNodes = make(map[types.Address]int)
	for _, addr := range keepNodes {
		cs.confirmedNodes[addr] = 0
	}
}

// GetConfirmedNodesCount returns the number of nodes that have confirmed
func (cs *ConsensusState) GetConfirmedNodesCount(minConfirmations int, connectedNodes map[types.Address]bool) int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	count := 0
	for addr, confirmations := range cs.confirmedNodes {
		if connectedNodes[addr] && confirmations >= minConfirmations {
			count++
		}
	}
	return count
}

// GetLastStateChange returns the time of last state change
func (cs *ConsensusState) GetLastStateChange() time.Time {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.lastStateChange
}

