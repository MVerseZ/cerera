# GIGEA Consensus System

This document describes the new template-based consensus system in the GIGEA engine.

## Overview

The GIGEA engine now uses a flexible, template-based consensus architecture that allows for easy implementation of different consensus algorithms. The system currently includes:

1. **Simple Consensus** - A basic leader-based consensus for demonstration
2. **Custom Consensus** - Template for implementing your own consensus algorithms

## Architecture

### Consensus Template Interface

All consensus algorithms implement the `ConsensusAlgorithm` interface:

```go
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
```

### Consensus Manager

The `ConsensusManager` manages consensus algorithm instances:

```go
type ConsensusManager struct {
    ConsensusType ConsensusType
    NodeID        types.Address
    Peers         []types.Address
    consensus     ConsensusAlgorithm
    engine        *Engine
}
```

### Base Consensus Node

The `BaseConsensusNode` provides common functionality for consensus implementations:

- State management (running, term, leader, etc.)
- Event handling system
- Metrics collection
- Thread-safe operations
- Configuration management

## Simple Consensus Implementation

The `SimpleConsensus` demonstrates how to implement the consensus interface:

Key features:
- Leader election with randomized timeouts
- Basic request processing
- Heartbeat mechanism
- Event-driven architecture
- Thread-safe operations

## Creating Custom Consensus Algorithms

To implement your own consensus algorithm:

1. **Implement the Interface**: Create a struct that implements `ConsensusAlgorithm`
2. **Extend Base Node**: Optionally embed `BaseConsensusNode` for common functionality
3. **Register Algorithm**: Add your algorithm to the `ConsensusManager`

### Example Implementation

```go
type MyConsensus struct {
    *BaseConsensusNode
    // Your custom fields
}

func NewMyConsensus(config ConsensusConfig, networkManager *NetworkManager, engine *Engine) *MyConsensus {
    base := NewBaseConsensusNode(config, networkManager, engine)
    return &MyConsensus{
        BaseConsensusNode: base,
        // Initialize your fields
    }
}

// Implement required methods...
func (mc *MyConsensus) Start(ctx context.Context) error {
    // Your implementation
}
```

## Usage

### Basic Setup

```go
// Create engine
engine := &Engine{}
engine.Start(nodeAddress)

// Consensus manager is automatically created with SimpleConsensus
consensusManager := NewConsensusManager(ConsensusTypeSimple, nodeAddress, peers, engine)
consensusManager.Start()
```

### Switching Consensus Algorithms

```go
// Switch to a different consensus algorithm
consensusManager.SwitchConsensus(ConsensusTypeCustom)
```

### Submitting Requests

```go
// Submit a request to the consensus
err := consensusManager.SubmitRequest("transfer:alice->bob:100")
if err != nil {
    log.Printf("Failed to submit request: %v", err)
}
```

### Getting Consensus Information

```go
// Get current consensus state
info := consensusManager.GetConsensusInfo()
fmt.Printf("Current leader: %s\n", info["leader"])
fmt.Printf("Node state: %s\n", info["state"])
fmt.Printf("Current term: %d\n", info["term"])
```

## Event Handling

The consensus system supports event-driven architecture through the `ConsensusEventHandler` interface:

```go
type ConsensusEventHandler interface {
    OnLeaderElected(nodeID types.Address, term int64)
    OnLeaderLost(nodeID types.Address, term int64)
    OnRequestCommitted(operation string, result interface{})
    OnPeerAdded(peer types.Address)
    OnPeerRemoved(peer types.Address)
    OnConsensusError(err error)
}
```

## Configuration

Consensus algorithms can be configured using the `ConsensusConfig` struct:

```go
config := DefaultConsensusConfig(nodeID)
config.HeartbeatInterval = 50 * time.Millisecond
config.ElectionTimeout = 500 * time.Millisecond
config.MaxRetries = 5

// Update algorithm configuration
consensusManager.consensus.UpdateConfig(config)
```

## Metrics

The system provides comprehensive metrics:

```go
metrics := consensusManager.consensus.GetMetrics()
fmt.Printf("Requests processed: %d\n", metrics["requests_processed"])
fmt.Printf("Leadership changes: %d\n", metrics["leadership_changes"])
fmt.Printf("Start time: %v\n", metrics["start_time"])
```

## Benefits of the Template System

1. **Flexibility**: Easy to implement different consensus algorithms
2. **Consistency**: Common interface ensures interoperability
3. **Extensibility**: Base classes provide common functionality
4. **Testability**: Each algorithm can be tested independently
5. **Maintainability**: Clean separation of concerns
6. **Performance**: Optimized for different use cases

## Migration Guide

If you were using the old PBFT or Raft implementations:

1. **Update Consensus Type**: Change from `ConsensusTypePBFT`/`ConsensusTypeRaft` to `ConsensusTypeSimple`
2. **Update Method Calls**: Use the new consensus interface methods
3. **Event Handling**: Implement `ConsensusEventHandler` for custom event processing
4. **Configuration**: Use the new `ConsensusConfig` system

## Future Extensions

The template system supports easy addition of new consensus algorithms:

- **PBFT**: Byzantine fault-tolerant consensus
- **Raft**: Log replication with leader election  
- **PoS**: Proof of Stake consensus
- **HotStuff**: Modern BFT consensus
- **Custom**: Your domain-specific consensus

Each can be implemented by following the `ConsensusAlgorithm` interface and extending `BaseConsensusNode`. 