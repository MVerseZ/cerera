package gigea

// import (
// 	"fmt"
// 	"testing"
// 	"time"

// 	"github.com/cerera/internal/cerera/types"
// )

// func TestNetworkManager(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	// Create network manager
// 	nm := NewNetworkManager(node1, 30001)

// 	// Start network manager
// 	if err := nm.Start(); err != nil {
// 		t.Fatalf("Failed to start network manager: %v", err)
// 	}
// 	defer nm.Stop()

// 	// Add peers with PeerInfo
// 	peerInfo2 := NewPeerInfo(node2, "localhost:30002")
// 	peerInfo3 := NewPeerInfo(node3, "localhost:30003")
// 	nm.AddPeer(peerInfo2)
// 	nm.AddPeer(peerInfo3)

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	// Test peer management
// 	peers := nm.GetPeers()
// 	fmt.Printf("Total peers: %d\n", len(peers))

// 	connectedPeers := nm.GetConnectedPeers()
// 	fmt.Printf("Connected peers: %d\n", len(connectedPeers))

// 	// Test message sending
// 	nm.SendPing(node2)
// 	nm.SendPing(node3)

// 	// Wait a bit for message processing
// 	time.Sleep(200 * time.Millisecond)
// }

// func TestPBFTConsensusWithNetwork(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	// Create PeerInfo list
// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 		NewPeerInfo(node3, "localhost:30003"),
// 	}

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Create PBFT consensus manager
// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	// Test consensus state
// 	state := consensusManager.GetConsensusState()
// 	fmt.Printf("PBFT Consensus State: %+v\n", state)

// 	// Test network info
// 	networkInfo := consensusManager.GetNetworkInfo()
// 	fmt.Printf("Network Info: %+v\n", networkInfo)

// 	// Submit a request
// 	consensusManager.SubmitRequest("test_operation_1")

// 	// Wait a bit for processing
// 	time.Sleep(200 * time.Millisecond)

// 	// Check if this node is primary
// 	isPrimary := consensusManager.IsLeader()
// 	fmt.Printf("Is Primary: %t\n", isPrimary)

// 	// Get consensus info
// 	info := consensusManager.GetConsensusInfo()
// 	fmt.Printf("Consensus Info: %+v\n", info)
// }

// func TestRaftConsensusWithNetwork(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	// Create PeerInfo list
// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 		NewPeerInfo(node3, "localhost:30003"),
// 	}

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Create Raft consensus manager
// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	// Test consensus state
// 	state := consensusManager.GetConsensusState()
// 	fmt.Printf("Raft Consensus State: %+v\n", state)

// 	// Test network info
// 	networkInfo := consensusManager.GetNetworkInfo()
// 	fmt.Printf("Network Info: %+v\n", networkInfo)

// 	// Submit a request
// 	consensusManager.SubmitRequest("test_operation_1")

// 	// Wait a bit for processing
// 	time.Sleep(200 * time.Millisecond)

// 	// Check if this node is leader
// 	isLeader := consensusManager.IsLeader()
// 	fmt.Printf("Is Leader: %t\n", isLeader)

// 	// Get consensus info
// 	info := consensusManager.GetConsensusInfo()
// 	fmt.Printf("Consensus Info: %+v\n", info)
// }

// func TestHybridConsensusWithNetwork(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	// Create PeerInfo list
// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 		NewPeerInfo(node3, "localhost:30003"),
// 	}

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Create Hybrid consensus manager
// 	consensusManager := NewConsensusManager(ConsensusTypeCustom, node1, peers, engine)
// 	consensusManager.Start()

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	// Test consensus state
// 	state := consensusManager.GetConsensusState()
// 	fmt.Printf("Hybrid Consensus State: %+v\n", state)

// 	// Test network info
// 	networkInfo := consensusManager.GetNetworkInfo()
// 	fmt.Printf("Network Info: %+v\n", networkInfo)

// 	// Submit a request
// 	consensusManager.SubmitRequest("test_operation_1")

// 	// Wait a bit for processing
// 	time.Sleep(200 * time.Millisecond)

// 	// Check if this node is leader
// 	isLeader := consensusManager.IsLeader()
// 	fmt.Printf("Is Leader: %t\n", isLeader)

// 	// Get consensus info
// 	info := consensusManager.GetConsensusInfo()
// 	fmt.Printf("Consensus Info: %+v\n", info)
// }

// func TestNetworkPeerManagement(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create PeerInfo list
// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 	}

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Create consensus manager
// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	fmt.Printf("Initial peer count: %d\n", len(consensusManager.Peers))

// 	// Add a peer with PeerInfo
// 	peerInfo2 := NewPeerInfo(node2, "localhost:30002")
// 	consensusManager.AddPeer(peerInfo2)
// 	time.Sleep(100 * time.Millisecond)

// 	fmt.Printf("After adding peer: %d\n", len(consensusManager.Peers))

// 	// Test network info
// 	networkInfo := consensusManager.GetNetworkInfo()
// 	fmt.Printf("Network Info: %+v\n", networkInfo)

// 	// Remove a peer
// 	consensusManager.RemovePeer(node2)
// 	time.Sleep(100 * time.Millisecond)

// 	fmt.Printf("After removing peer: %d\n", len(consensusManager.Peers))

// 	// Get consensus info
// 	info := consensusManager.GetConsensusInfo()
// 	fmt.Printf("Consensus Info: %+v\n", info)
// }

// func TestEngineNetworkIntegration(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Wait a bit for initialization
// 	time.Sleep(100 * time.Millisecond)

// 	// Test network info
// 	networkInfo := engine.GetNetworkInfo()
// 	fmt.Printf("Engine Network Info: %+v\n", networkInfo)

// 	// Test consensus info
// 	consensusInfo := engine.GetConsensusInfo()
// 	fmt.Printf("Engine Consensus Info: %+v\n", consensusInfo)

// 	// Add a peer
// 	engine.AddPeer(node2)
// 	time.Sleep(100 * time.Millisecond)

// 	// Test connected peers
// 	connectedPeers := engine.GetConnectedPeers()
// 	fmt.Printf("Connected peers: %d\n", len(connectedPeers))

// 	// Test total peers
// 	totalPeers := engine.GetTotalPeers()
// 	fmt.Printf("Total peers: %d\n", len(totalPeers))

// 	// Remove a peer
// 	engine.RemovePeer(node2)
// 	time.Sleep(100 * time.Millisecond)

// 	// Test final state
// 	finalNetworkInfo := engine.GetNetworkInfo()
// 	fmt.Printf("Final Network Info: %+v\n", finalNetworkInfo)
// }

// // Benchmark tests for network performance
// func BenchmarkNetworkMessageSending(b *testing.B) {
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	nm := NewNetworkManager(node1, 30001)
// 	nm.Start()
// 	defer nm.Stop()

// 	peerInfo := NewPeerInfo(node2, "localhost:30002")
// 	nm.AddPeer(peerInfo)
// 	time.Sleep(100 * time.Millisecond)

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		nm.SendPing(node2)
// 	}
// }

// func BenchmarkPBFTConsensusWithNetwork(b *testing.B) {
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 		NewPeerInfo(node3, "localhost:30003"),
// 	}
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	time.Sleep(100 * time.Millisecond)

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		consensusManager.SubmitRequest(fmt.Sprintf("benchmark_operation_%d", i))
// 	}
// }

// func BenchmarkRaftConsensusWithNetwork(b *testing.B) {
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
// 	node3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 		NewPeerInfo(node3, "localhost:30003"),
// 	}
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	time.Sleep(100 * time.Millisecond)

// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		consensusManager.SubmitRequest(fmt.Sprintf("benchmark_operation_%d", i))
// 	}
// }
