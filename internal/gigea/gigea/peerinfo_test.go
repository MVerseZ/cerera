package gigea

import (
	"testing"

	"github.com/cerera/internal/cerera/types"
)

func TestNewPeerInfo(t *testing.T) {
	// Create test address
	cereraAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")
	networkAddr := "192.168.1.100:8080"

	// Create PeerInfo
	peerInfo := NewPeerInfo(cereraAddr, networkAddr)

	// Test that PeerInfo was created correctly
	if peerInfo.CereraAddress != cereraAddr {
		t.Errorf("Expected CereraAddress %s, got %s", cereraAddr.Hex(), peerInfo.CereraAddress.Hex())
	}

	if peerInfo.NetworkAddr != networkAddr {
		t.Errorf("Expected NetworkAddr %s, got %s", networkAddr, peerInfo.NetworkAddr)
	}
}

func TestPeerInfoWithNetworkManager(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	// Create network manager
	nm := NewNetworkManager(node1, 30001)

	// Create PeerInfo
	peerInfo := NewPeerInfo(node2, "localhost:30002")

	// Add peer to network manager
	nm.AddPeer(peerInfo)

	// Check that peer was added
	peers := nm.GetPeers()
	if len(peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(peers))
	}

	if peers[0] != node2 {
		t.Errorf("Expected peer %s, got %s", node2.Hex(), peers[0].Hex())
	}
}

func TestPeerInfoWithConsensusManager(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	// Create PeerInfo list
	peers := []*PeerInfo{
		NewPeerInfo(node1, "localhost:30001"),
		NewPeerInfo(node2, "localhost:30002"),
	}

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Create consensus manager
	consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
	consensusManager.Start()

	// Check that peers were added correctly
	if len(consensusManager.Peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(consensusManager.Peers))
	}

	// Check first peer
	if consensusManager.Peers[0].CereraAddress != node1 {
		t.Errorf("Expected first peer %s, got %s", node1.Hex(), consensusManager.Peers[0].CereraAddress.Hex())
	}

	if consensusManager.Peers[0].NetworkAddr != "localhost:30001" {
		t.Errorf("Expected first peer network addr localhost:30001, got %s", consensusManager.Peers[0].NetworkAddr)
	}

	// Check second peer
	if consensusManager.Peers[1].CereraAddress != node2 {
		t.Errorf("Expected second peer %s, got %s", node2.Hex(), consensusManager.Peers[1].CereraAddress.Hex())
	}

	if consensusManager.Peers[1].NetworkAddr != "localhost:30002" {
		t.Errorf("Expected second peer network addr localhost:30002, got %s", consensusManager.Peers[1].NetworkAddr)
	}
}

func TestEngineAddPeerWithNetworkAddress(t *testing.T) {
	// Create test addresses
	node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	// Create engine
	engine := &Engine{}
	engine.Start(node1)

	// Add peer using Engine.AddPeer (which should create PeerInfo automatically)
	engine.AddPeer(node2)

	// Check that peer was added to consensus manager
	if engine.ConsensusManager == nil {
		t.Fatal("ConsensusManager should not be nil")
	}

	// The consensus manager should have the peer added
	// Note: We can't directly check the PeerInfo in the consensus manager
	// because it's created internally, but we can check that the peer was added
	networkInfo := engine.GetNetworkInfo()
	if networkInfo["error"] != nil {
		t.Errorf("Network info should not have error: %v", networkInfo["error"])
	}
}

func TestPeerInfoNetworkAddressFormat(t *testing.T) {
	// Test different network address formats
	testCases := []struct {
		name        string
		networkAddr string
		valid       bool
	}{
		{"localhost with port", "localhost:8080", true},
		{"IP with port", "192.168.1.100:8080", true},
		{"IPv6 with port", "[::1]:8080", true},
		{"just port", ":8080", true},
		{"invalid format", "invalid", false},
		{"missing port", "localhost", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cereraAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

			// Create PeerInfo
			peerInfo := NewPeerInfo(cereraAddr, tc.networkAddr)

			// Check that PeerInfo was created
			if peerInfo == nil {
				t.Error("PeerInfo should not be nil")
				return
			}

			// Check that network address was set correctly
			if peerInfo.NetworkAddr != tc.networkAddr {
				t.Errorf("Expected NetworkAddr %s, got %s", tc.networkAddr, peerInfo.NetworkAddr)
			}

			// Check that Cerera address was set correctly
			if peerInfo.CereraAddress != cereraAddr {
				t.Errorf("Expected CereraAddress %s, got %s", cereraAddr.Hex(), peerInfo.CereraAddress.Hex())
			}
		})
	}
}
