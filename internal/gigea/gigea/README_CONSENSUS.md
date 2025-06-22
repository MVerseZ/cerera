# GIGEA Consensus Algorithms

This document describes the implementation of PBFT (Practical Byzantine Fault Tolerance) and Raft consensus algorithms in the GIGEA engine.

## Overview

The GIGEA engine now supports multiple consensus algorithms:

1. **PBFT (Practical Byzantine Fault Tolerance)** - Byzantine fault-tolerant consensus
2. **Raft** - Leader-based consensus with log replication
3. **Hybrid** - Combination of both algorithms

## Architecture

### Consensus Manager

The `ConsensusManager` acts as a central coordinator for different consensus algorithms:

```go
type ConsensusManager struct {
    ConsensusType ConsensusType
    NodeID        types.Address
    Peers         []types.Address
    PBFTNode      *PBFTNode
    RaftNode      *RaftNode
    engine        *Engine
}
```

### PBFT Implementation

The PBFT implementation follows the three-phase protocol:

1. **Pre-Prepare**: Primary assigns sequence number and broadcasts pre-prepare message
2. **Prepare**: Replicas broadcast prepare messages after receiving pre-prepare
3. **Commit**: Replicas broadcast commit messages after receiving 2f+1 prepare messages

Key features:
- Byzantine fault tolerance (tolerates up to f faulty nodes in 3f+1 total nodes)
- View change mechanism for primary failure
- Message logging and verification
- Request execution after consensus

### Raft Implementation

The Raft implementation follows the standard Raft protocol:

1. **Leader Election**: Nodes compete for leadership using randomized timeouts
2. **Log Replication**: Leader replicates log entries to followers
3. **Safety**: Ensures log consistency across all nodes

Key features:
- Leader election with randomized timeouts
- Log replication with append entries RPCs
- Term-based state management
- Heartbeat mechanism for leader detection

## Usage

### Basic Setup

```go
// Create engine
engine := &Engine{}
engine.Start(nodeAddress)

// Create consensus manager
peers := []types.Address{node1, node2, node3}
consensusManager := NewConsensusManager(ConsensusTypePBFT, nodeAddress, peers, engine)
consensusManager.Start()
```

### Switching Consensus Algorithms

```go
// Switch from PBFT to Raft
consensusManager.SwitchConsensus(ConsensusTypeRaft)

// Switch to hybrid mode
consensusManager.SwitchConsensus(ConsensusTypeHybrid)
```

### Submitting Requests

```go
// Submit a transaction or operation
consensusManager.SubmitRequest("transaction:0x123...")
```

### Getting Consensus State

```go
// Get current consensus information
info := consensusManager.GetConsensusInfo()
fmt.Printf("Consensus Info: %+v\n", info)
```

## Integration with GIGEA Engine

The consensus algorithms are integrated into the existing GIGEA engine:

### Engine Integration

```go
type Engine struct {
    // ... existing fields ...
    ConsensusManager *ConsensusManager
}
```

### Automatic State Management

The engine automatically updates its state based on consensus:

- If consensus indicates this node is leader → `Leader` state
- If consensus indicates this node is not leader → `Follower` state
- Fallback to original logic for backward compatibility

### Transaction Processing

Transactions are automatically submitted to consensus:

```go
case tx := <-e.TxFunnel:
    e.Pack(tx)
    // Submit transaction to consensus
    if e.ConsensusManager != nil {
        e.ConsensusManager.SubmitRequest(fmt.Sprintf("transaction:%s", tx.Hash().Hex()))
    }
```

## Configuration

### Consensus Types

```go
const (
    ConsensusTypePBFT   ConsensusType = iota
    ConsensusTypeRaft
    ConsensusTypeHybrid
)
```

### PBFT Configuration

- **Fault Tolerance**: Automatically calculated as `(len(replicas) - 1) / 3`
- **Message Logs**: Pre-prepare, prepare, and commit message logs
- **View Management**: Automatic view changes on primary failure

### Raft Configuration

- **Heartbeat Interval**: 100ms (configurable)
- **Election Timeout**: 150-300ms randomized (configurable)
- **Log Management**: Persistent log with term-based consistency

## Testing

Run the consensus tests:

```bash
go test ./internal/gigea/gigea -v -run TestPBFTConsensus
go test ./internal/gigea/gigea -v -run TestRaftConsensus
go test ./internal/gigea/gigea -v -run TestHybridConsensus
```

Run performance benchmarks:

```bash
go test ./internal/gigea/gigea -bench=BenchmarkPBFTConsensus
go test ./internal/gigea/gigea -bench=BenchmarkRaftConsensus
```

## Network Communication

**Note**: The current implementation includes placeholder network communication. To enable full distributed consensus:

1. Implement actual network broadcasting in `broadcastPrePrepare`, `broadcastPrepare`, `broadcastCommit`
2. Implement network message handling for incoming consensus messages
3. Add proper peer discovery and connection management
4. Implement message serialization and deserialization

## Limitations

1. **Network Communication**: Currently uses placeholder network communication
2. **Persistence**: Logs are not persisted to disk
3. **View Changes**: PBFT view change mechanism is simplified
4. **Configuration Changes**: Raft configuration changes are not implemented
5. **Security**: Cryptographic signatures are not implemented

## Future Enhancements

1. **Full Network Implementation**: Complete the network communication layer
2. **Persistence**: Add disk-based log persistence
3. **Security**: Implement cryptographic signatures and verification
4. **Configuration Management**: Add dynamic configuration changes
5. **Performance Optimization**: Optimize message handling and state management
6. **Monitoring**: Add metrics and monitoring capabilities

## API Reference

### ConsensusManager

- `NewConsensusManager(consensusType, nodeID, peers, engine)` - Create new manager
- `Start()` - Start consensus algorithms
- `SubmitRequest(operation)` - Submit operation to consensus
- `GetConsensusState()` - Get current consensus state
- `IsLeader()` - Check if this node is leader
- `GetLeader()` - Get current leader address
- `SwitchConsensus(newType)` - Switch consensus algorithm
- `AddPeer(peer)` - Add new peer
- `RemovePeer(peer)` - Remove peer
- `GetConsensusInfo()` - Get detailed consensus information

### Engine Integration

- `SetConsensusType(consensusType)` - Set consensus algorithm
- `GetConsensusInfo()` - Get consensus information
- `AddPeer(peer)` - Add peer to consensus
- `RemovePeer(peer)` - Remove peer from consensus 