package gigea

import (
	"context"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// ConsensusAlgorithm defines the interface that all consensus algorithms must implement
type ConsensusAlgorithm interface {
	// Lifecycle methods
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool

	// Core consensus methods
	SubmitRequest(operation string) error
	ProposeBlock(block interface{}) error

	// State methods
	IsLeader() bool
	GetLeader() types.Address
	GetCurrentTerm() int64
	GetNodeState() string

	// Peer management
	AddPeer(peer types.Address) error
	RemovePeer(peer types.Address) error
	GetPeers() []types.Address

	// Information methods
	GetConsensusInfo() map[string]interface{}
	GetMetrics() map[string]interface{}

	// Configuration
	UpdateConfig(config ConsensusConfig) error
	GetConfig() ConsensusConfig
}

// ConsensusConfig holds configuration for consensus algorithms
type ConsensusConfig struct {
	// Basic settings
	NodeID types.Address   `json:"node_id"`
	Peers  []types.Address `json:"peers"`

	// Timing settings
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ElectionTimeout   time.Duration `json:"election_timeout"`
	RequestTimeout    time.Duration `json:"request_timeout"`

	// Algorithm-specific settings
	MaxRetries       int `json:"max_retries"`
	BatchSize        int `json:"batch_size"`
	LogRetentionSize int `json:"log_retention_size"`

	// Network settings
	NetworkTimeout   time.Duration `json:"network_timeout"`
	MaxConcurrentReq int           `json:"max_concurrent_requests"`

	// Custom parameters for algorithm-specific configuration
	CustomParams map[string]interface{} `json:"custom_params"`
}

// DefaultConsensusConfig returns a default configuration
func DefaultConsensusConfig(nodeID types.Address) ConsensusConfig {
	return ConsensusConfig{
		NodeID:            nodeID,
		Peers:             make([]types.Address, 0),
		HeartbeatInterval: 1000 * time.Millisecond,
		ElectionTimeout:   3 * time.Second,
		RequestTimeout:    5 * time.Second,
		MaxRetries:        3,
		BatchSize:         100,
		LogRetentionSize:  1000,
		NetworkTimeout:    2 * time.Second,
		MaxConcurrentReq:  100,
		CustomParams:      make(map[string]interface{}),
	}
}

// ConsensusEvent represents events that occur in the consensus system
type ConsensusEvent struct {
	Type      string                 `json:"type"`
	NodeID    types.Address          `json:"node_id"`
	Term      int64                  `json:"term"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ConsensusEventHandler defines the interface for handling consensus events
type ConsensusEventHandler interface {
	OnLeaderElected(nodeID types.Address, term int64)
	OnLeaderLost(nodeID types.Address, term int64)
	OnRequestCommitted(operation string, result interface{})
	OnPeerAdded(peer types.Address)
	OnPeerRemoved(peer types.Address)
	OnConsensusError(err error)
}

// BaseConsensusNode provides common functionality for consensus implementations
type BaseConsensusNode struct {
	config         ConsensusConfig
	eventHandler   ConsensusEventHandler
	networkManager *NetworkManager
	engine         *Engine

	// State
	running     bool
	currentTerm int64
	nodeState   string
	leader      types.Address

	// Synchronization
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// Metrics
	metricsLock sync.RWMutex
	metrics     map[string]interface{}
}

// NewBaseConsensusNode creates a new base consensus node
func NewBaseConsensusNode(config ConsensusConfig, networkManager *NetworkManager, engine *Engine) *BaseConsensusNode {
	ctx, cancel := context.WithCancel(context.Background())

	return &BaseConsensusNode{
		config:         config,
		networkManager: networkManager,
		engine:         engine,
		running:        false,
		currentTerm:    0,
		nodeState:      "follower",
		leader:         types.Address{},
		ctx:            ctx,
		cancel:         cancel,
		metrics:        make(map[string]interface{}),
	}
}

// Common methods that can be used by consensus implementations

func (b *BaseConsensusNode) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

func (b *BaseConsensusNode) GetCurrentTerm() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentTerm
}

func (b *BaseConsensusNode) GetNodeState() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.nodeState
}

func (b *BaseConsensusNode) GetLeader() types.Address {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.leader
}

func (b *BaseConsensusNode) GetPeers() []types.Address {
	b.mu.RLock()
	defer b.mu.RUnlock()
	peers := make([]types.Address, len(b.config.Peers))
	copy(peers, b.config.Peers)
	return peers
}

func (b *BaseConsensusNode) GetConfig() ConsensusConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

func (b *BaseConsensusNode) UpdateConfig(config ConsensusConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config = config
	return nil
}

func (b *BaseConsensusNode) GetMetrics() map[string]interface{} {
	b.metricsLock.RLock()
	defer b.metricsLock.RUnlock()

	result := make(map[string]interface{})
	for k, v := range b.metrics {
		result[k] = v
	}
	return result
}

func (b *BaseConsensusNode) UpdateMetric(key string, value interface{}) {
	b.metricsLock.Lock()
	defer b.metricsLock.Unlock()
	b.metrics[key] = value
}

func (b *BaseConsensusNode) SetEventHandler(handler ConsensusEventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eventHandler = handler
}

// Helper methods for state management
func (b *BaseConsensusNode) setState(state string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodeState = state
}

func (b *BaseConsensusNode) setLeader(leader types.Address) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.leader = leader
}

func (b *BaseConsensusNode) setTerm(term int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentTerm = term
}

// Event emission helpers
func (b *BaseConsensusNode) emitLeaderElected(nodeID types.Address, term int64) {
	if b.eventHandler != nil {
		b.eventHandler.OnLeaderElected(nodeID, term)
	}
}

func (b *BaseConsensusNode) emitLeaderLost(nodeID types.Address, term int64) {
	if b.eventHandler != nil {
		b.eventHandler.OnLeaderLost(nodeID, term)
	}
}

func (b *BaseConsensusNode) emitRequestCommitted(operation string, result interface{}) {
	if b.eventHandler != nil {
		b.eventHandler.OnRequestCommitted(operation, result)
	}
}

func (b *BaseConsensusNode) emitPeerAdded(peer types.Address) {
	if b.eventHandler != nil {
		b.eventHandler.OnPeerAdded(peer)
	}
}

func (b *BaseConsensusNode) emitPeerRemoved(peer types.Address) {
	if b.eventHandler != nil {
		b.eventHandler.OnPeerRemoved(peer)
	}
}

func (b *BaseConsensusNode) emitConsensusError(err error) {
	if b.eventHandler != nil {
		b.eventHandler.OnConsensusError(err)
	}
}
