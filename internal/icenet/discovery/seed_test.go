package discovery

import (
	"context"
	"testing"

	"github.com/cerera/internal/icenet/connection"
)

func TestNewSeedDiscovery(t *testing.T) {
	ctx := context.Background()
	seedNodes := []string{"192.168.1.1:31100", "192.168.1.2:31100"}
	connManager := connection.NewManager(ctx, nil)

	sd := NewSeedDiscovery(ctx, seedNodes, connManager)
	if sd == nil {
		t.Fatal("SeedDiscovery is nil")
	}

	nodes := sd.GetSeedNodes()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 seed nodes, got %d", len(nodes))
	}
}

func TestSeedDiscovery_AddSeedNode(t *testing.T) {
	ctx := context.Background()
	seedNodes := []string{}
	connManager := connection.NewManager(ctx, nil)

	sd := NewSeedDiscovery(ctx, seedNodes, connManager)

	sd.AddSeedNode("192.168.1.1:31100")
	sd.AddSeedNode("192.168.1.2:31100")

	nodes := sd.GetSeedNodes()
	if len(nodes) != 2 {
		t.Fatalf("Expected 2 seed nodes, got %d", len(nodes))
	}
}
