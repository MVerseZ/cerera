package gigea

import (
	"fmt"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/types"
)

func TestPBFTConsensus(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create PBFT consensus manager
	consensusManager := NewConsensusManager(ConsensusTypePBFT, node1, peers, engine)
	consensusManager.Start()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	// Test consensus state
	state := consensusManager.GetConsensusState()
	fmt.Printf("PBFT Consensus State: %+v\n", state)

	// Submit a request
	consensusManager.SubmitRequest("test_operation_1")

	// Wait a bit for processing
	time.Sleep(200 * time.Millisecond)

	// Check if this node is primary
	isPrimary := consensusManager.IsLeader()
	fmt.Printf("Is Primary: %t\n", isPrimary)

	// Get consensus info
	info := consensusManager.GetConsensusInfo()
	fmt.Printf("Consensus Info: %+v\n", info)
}

func TestRaftConsensus(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create Raft consensus manager
	consensusManager := NewConsensusManager(ConsensusTypeRaft, node1, peers, engine)
	consensusManager.Start()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	// Test consensus state
	state := consensusManager.GetConsensusState()
	fmt.Printf("Raft Consensus State: %+v\n", state)

	// Submit a request
	consensusManager.SubmitRequest("test_operation_1")

	// Wait a bit for processing
	time.Sleep(200 * time.Millisecond)

	// Check if this node is leader
	isLeader := consensusManager.IsLeader()
	fmt.Printf("Is Leader: %t\n", isLeader)

	// Get consensus info
	info := consensusManager.GetConsensusInfo()
	fmt.Printf("Consensus Info: %+v\n", info)
}

func TestHybridConsensus(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create Hybrid consensus manager
	consensusManager := NewConsensusManager(ConsensusTypeHybrid, node1, peers, engine)
	consensusManager.Start()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	// Test consensus state
	state := consensusManager.GetConsensusState()
	fmt.Printf("Hybrid Consensus State: %+v\n", state)

	// Submit a request
	consensusManager.SubmitRequest("test_operation_1")

	// Wait a bit for processing
	time.Sleep(200 * time.Millisecond)

	// Check if this node is leader
	isLeader := consensusManager.IsLeader()
	fmt.Printf("Is Leader: %t\n", isLeader)

	// Get consensus info
	info := consensusManager.GetConsensusInfo()
	fmt.Printf("Consensus Info: %+v\n", info)
}

func TestConsensusSwitching(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create consensus manager starting with PBFT
	consensusManager := NewConsensusManager(ConsensusTypePBFT, node1, peers, engine)
	consensusManager.Start()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("Initial consensus type: %s\n", consensusManager.ConsensusType.String())

	// Switch to Raft
	consensusManager.SwitchConsensus(ConsensusTypeRaft)
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("After switch to Raft: %s\n", consensusManager.ConsensusType.String())

	// Switch to Hybrid
	consensusManager.SwitchConsensus(ConsensusTypeHybrid)
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("After switch to Hybrid: %s\n", consensusManager.ConsensusType.String())

	// Get final consensus info
	info := consensusManager.GetConsensusInfo()
	fmt.Printf("Final Consensus Info: %+v\n", info)
}

func TestPeerManagement(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	peers := []types.Address{node1}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create consensus manager
	consensusManager := NewConsensusManager(ConsensusTypePBFT, node1, peers, engine)
	consensusManager.Start()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("Initial peer count: %d\n", len(consensusManager.Peers))

	// Add a peer
	consensusManager.AddPeer(node2)
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("After adding peer: %d\n", len(consensusManager.Peers))

	// Remove a peer
	consensusManager.RemovePeer(node2)
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("After removing peer: %d\n", len(consensusManager.Peers))

	// Get consensus info
	info := consensusManager.GetConsensusInfo()
	fmt.Printf("Consensus Info: %+v\n", info)
}

// Benchmark tests for performance comparison
func BenchmarkPBFTConsensus(b *testing.B) {
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}
	engine := &Engine{}
	engine.Start(node1)

	consensusManager := NewConsensusManager(ConsensusTypePBFT, node1, peers, engine)
	consensusManager.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consensusManager.SubmitRequest(fmt.Sprintf("benchmark_operation_%d", i))
	}
}

func BenchmarkRaftConsensus(b *testing.B) {
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	peers := []types.Address{node1, node2, node3}
	engine := &Engine{}
	engine.Start(node1)

	consensusManager := NewConsensusManager(ConsensusTypeRaft, node1, peers, engine)
	consensusManager.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consensusManager.SubmitRequest(fmt.Sprintf("benchmark_operation_%d", i))
	}
}
