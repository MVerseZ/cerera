package gigea

// import (
// 	"fmt"
// 	"strings"
// 	"testing"

// 	"github.com/cerera/internal/cerera/types"
// )

// func TestNewPeerInfo(t *testing.T) {
// 	// Create test address
// 	cereraAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")
// 	networkAddr := "192.168.1.100:8080"

// 	// Create PeerInfo
// 	peerInfo := NewPeerInfo(cereraAddr, networkAddr)

// 	// Test that PeerInfo was created correctly
// 	if peerInfo.CereraAddress != cereraAddr {
// 		t.Errorf("Expected CereraAddress %s, got %s", cereraAddr.Hex(), peerInfo.CereraAddress.Hex())
// 	}

// 	if peerInfo.NetworkAddr != networkAddr {
// 		t.Errorf("Expected NetworkAddr %s, got %s", networkAddr, peerInfo.NetworkAddr)
// 	}
// }

// func TestPeerInfoWithNetworkManager(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create network manager
// 	nm := NewNetworkManager(node1, 30001)

// 	// Create PeerInfo
// 	peerInfo := NewPeerInfo(node2, "localhost:30002")

// 	// Add peer to network manager
// 	nm.AddPeer(peerInfo)

// 	// Check that peer was added
// 	peers := nm.GetPeers()
// 	if len(peers) != 1 {
// 		t.Errorf("Expected 1 peer, got %d", len(peers))
// 	}

// 	if peers[0] != node2 {
// 		t.Errorf("Expected peer %s, got %s", node2.Hex(), peers[0].Hex())
// 	}
// }

// func TestPeerInfoWithConsensusManager(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create PeerInfo list
// 	peers := []*PeerInfo{
// 		NewPeerInfo(node1, "localhost:30001"),
// 		NewPeerInfo(node2, "localhost:30002"),
// 	}

// 	// Create engine
// 	engine := &Engine{}
// 	engine.Start(node1)

// 	// Create consensus manager
// 	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
// 	consensusManager.Start()

// 	// Check that peers were added correctly
// 	if len(consensusManager.Peers) != 2 {
// 		t.Errorf("Expected 2 peers, got %d", len(consensusManager.Peers))
// 	}

// 	// Check first peer
// 	if consensusManager.Peers[0].CereraAddress != node1 {
// 		t.Errorf("Expected first peer %s, got %s", node1.Hex(), consensusManager.Peers[0].CereraAddress.Hex())
// 	}

// 	if consensusManager.Peers[0].NetworkAddr != "localhost:30001" {
// 		t.Errorf("Expected first peer network addr localhost:30001, got %s", consensusManager.Peers[0].NetworkAddr)
// 	}

// 	// Check second peer
// 	if consensusManager.Peers[1].CereraAddress != node2 {
// 		t.Errorf("Expected second peer %s, got %s", node2.Hex(), consensusManager.Peers[1].CereraAddress.Hex())
// 	}

// 	if consensusManager.Peers[1].NetworkAddr != "localhost:30002" {
// 		t.Errorf("Expected second peer network addr localhost:30002, got %s", consensusManager.Peers[1].NetworkAddr)
// 	}
// }

// func TestEngineAddPeerWithNetworkAddress(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create engine
// 	engine := &Engine{Port: 30001}
// 	engine.Start(node1)

// 	// Add peer
// 	engine.AddPeer(node2)

// 	// Check that peer was added to consensus manager
// 	consensusInfo := engine.GetConsensusInfo()
// 	peerCount := consensusInfo["peerCount"].(int)
// 	if peerCount != 2 {
// 		t.Errorf("Expected 2 total peers in consensus, got %d", peerCount)
// 	}

// 	// Check that peer is in the consensus peers list
// 	peers := consensusInfo["peers"].([]string)
// 	found := false
// 	for _, peer := range peers {
// 		if peer == node2.Hex() {
// 			found = true
// 			break
// 		}
// 	}
// 	if !found {
// 		t.Errorf("Expected to find peer %s in consensus peers", node2.Hex())
// 	}

// 	// Check that peer was added to legacy voters list
// 	foundInVoters := false
// 	for _, voter := range C.Voters {
// 		if voter == node2 {
// 			foundInVoters = true
// 			break
// 		}
// 	}
// 	if !foundInVoters {
// 		t.Errorf("Expected to find peer %s in legacy voters", node2.Hex())
// 	}
// }

// // TestConsensusStageMessage tests that consensus stage messages are sent when peers are added
// func TestConsensusStageMessage(t *testing.T) {
// 	// Create test addresses
// 	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
// 	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

// 	// Create engine
// 	engine := &Engine{Port: 30001}
// 	engine.Start(node1)

// 	// Mock PublishConsensus function to capture messages
// 	var capturedMessage string
// 	originalPublishConsensus := PublishConsensus
// 	PublishConsensus = func(msg string) {
// 		capturedMessage = msg
// 	}
// 	defer func() { PublishConsensus = originalPublishConsensus }()

// 	// Add peer - this should trigger consensus stage message
// 	engine.AddPeer(node2)

// 	// Check that message was captured
// 	if capturedMessage == "" {
// 		t.Error("Expected consensus stage message to be sent, but none was captured")
// 	}

// 	// Check message format - should contain _CONS_TOPOLOGY: for topology changes
// 	if !strings.Contains(capturedMessage, "_CONS_TOPOLOGY:") {
// 		t.Errorf("Expected message to contain _CONS_TOPOLOGY:, got: %s", capturedMessage)
// 	}

// 	// Check that message contains expected data
// 	if !strings.Contains(capturedMessage, node1.String()) {
// 		t.Errorf("Expected message to contain node1 address %s, got: %s", node1.String(), capturedMessage)
// 	}

// 	// Check that message contains topology change information
// 	if !strings.Contains(capturedMessage, "topology_change") {
// 		t.Errorf("Expected message to contain topology_change, got: %s", capturedMessage)
// 	}

// 	fmt.Printf("Captured consensus stage message: %s\n", capturedMessage)
// }

// func TestPeerInfoNetworkAddressFormat(t *testing.T) {
// 	// Test different network address formats
// 	testCases := []struct {
// 		name        string
// 		networkAddr string
// 		valid       bool
// 	}{
// 		{"localhost with port", "localhost:8080", true},
// 		{"IP with port", "192.168.1.100:8080", true},
// 		{"IPv6 with port", "[::1]:8080", true},
// 		{"just port", ":8080", true},
// 		{"invalid format", "invalid", false},
// 		{"missing port", "localhost", false},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			cereraAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

// 			// Create PeerInfo
// 			peerInfo := NewPeerInfo(cereraAddr, tc.networkAddr)

// 			// Check that PeerInfo was created
// 			if peerInfo == nil {
// 				t.Error("PeerInfo should not be nil")
// 				return
// 			}

// 			// Check that network address was set correctly
// 			if peerInfo.NetworkAddr != tc.networkAddr {
// 				t.Errorf("Expected NetworkAddr %s, got %s", tc.networkAddr, peerInfo.NetworkAddr)
// 			}

// 			// Check that Cerera address was set correctly
// 			if peerInfo.CereraAddress != cereraAddr {
// 				t.Errorf("Expected CereraAddress %s, got %s", cereraAddr.Hex(), peerInfo.CereraAddress.Hex())
// 			}
// 		})
// 	}
// }
