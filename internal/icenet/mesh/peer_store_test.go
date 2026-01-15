package mesh

import (
	"testing"
	"time"

	"github.com/cerera/internal/cerera/types"
)

func TestPeerStore_AddOrUpdate(t *testing.T) {
	ps := NewPeerStore()
	addr := types.HexToAddress("0x1234567890123456789012345678901234567890")
	networkAddr := "192.168.1.1:31100"

	peer := ps.AddOrUpdate(addr, networkAddr)
	if peer == nil {
		t.Fatal("Peer is nil")
	}

	if peer.Address != addr {
		t.Fatalf("Expected address %s, got %s", addr.Hex(), peer.Address.Hex())
	}

	if peer.NetworkAddr != networkAddr {
		t.Fatalf("Expected network address %s, got %s", networkAddr, peer.NetworkAddr)
	}
}

func TestPeerStore_GetConnected(t *testing.T) {
	ps := NewPeerStore()
	addr1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	ps.AddOrUpdate(addr1, "192.168.1.1:31100")
	ps.AddOrUpdate(addr2, "192.168.1.2:31100")

	// Mark first peer as connected
	ps.UpdateConnectionStatus(addr1, true, "conn1")

	connected := ps.GetConnected()
	if len(connected) != 1 {
		t.Fatalf("Expected 1 connected peer, got %d", len(connected))
	}

	if connected[0].Address != addr1 {
		t.Fatalf("Expected connected peer address %s, got %s", addr1.Hex(), connected[0].Address.Hex())
	}
}

func TestPeerStore_Cleanup(t *testing.T) {
	ps := NewPeerStore()
	addr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	ps.AddOrUpdate(addr, "192.168.1.1:31100")

	// Wait a bit and cleanup old peers
	time.Sleep(10 * time.Millisecond)
	removed := ps.Cleanup(1 * time.Millisecond)

	if removed != 1 {
		t.Fatalf("Expected 1 peer removed, got %d", removed)
	}
}
