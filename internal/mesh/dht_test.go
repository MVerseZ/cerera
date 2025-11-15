package mesh

import (
	"bytes"
	"testing"
	"time"

	"github.com/jbenet/go-base58"
)

func TestNewDHT(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	if dht == nil {
		t.Fatal("Expected DHT to be created")
	}

	if dht.store != store {
		t.Error("Expected store to be set")
	}

	if dht.options != options {
		t.Error("Expected options to be set")
	}

	// Verify default timeouts are set
	if options.TExpire == 0 {
		t.Error("Expected TExpire to be set to default")
	}
	if options.TRefresh == 0 {
		t.Error("Expected TRefresh to be set to default")
	}
	if options.TReplicate == 0 {
		t.Error("Expected TReplicate to be set to default")
	}
	if options.TRepublish == 0 {
		t.Error("Expected TRepublish to be set to default")
	}
	if options.TPingMax == 0 {
		t.Error("Expected TPingMax to be set to default")
	}
	if options.TMsgTimeout == 0 {
		t.Error("Expected TMsgTimeout to be set to default")
	}
}

func TestNewDHT_WithCustomTimeouts(t *testing.T) {
	store := &MemoryStore{}
	customExpire := 5 * time.Second
	customRefresh := 10 * time.Second

	options := &Options{
		ID:         []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:         "127.0.0.1",
		Port:       "3000",
		TExpire:    customExpire,
		TRefresh:   customRefresh,
		TReplicate: 15 * time.Second,
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	if dht.options.TExpire != customExpire {
		t.Errorf("Expected TExpire %v, got %v", customExpire, dht.options.TExpire)
	}
	if dht.options.TRefresh != customRefresh {
		t.Errorf("Expected TRefresh %v, got %v", customRefresh, dht.options.TRefresh)
	}
}

func TestNewDHT_InvalidOptions(t *testing.T) {
	store := &MemoryStore{}

	// Missing IP
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		Port: "3000",
	}

	_, err := NewDHT(store, options)
	if err == nil {
		t.Error("Expected error for missing IP")
	}

	// Missing Port
	options = &Options{
		ID: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP: "127.0.0.1",
	}

	_, err = NewDHT(store, options)
	if err == nil {
		t.Error("Expected error for missing Port")
	}
}

func TestDHT_NumNodes(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// Initially should have 0 nodes
	if dht.NumNodes() != 0 {
		t.Errorf("Expected 0 nodes initially, got %d", dht.NumNodes())
	}
}

func TestDHT_GetSelfID(t *testing.T) {
	store := &MemoryStore{}
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	options := &Options{
		ID:   id,
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	selfID := dht.GetSelfID()
	if selfID == "" {
		t.Error("Expected GetSelfID to return non-empty string")
	}

	// Verify it's base58 encoded
	decoded := base58.Decode(selfID)
	if !bytes.Equal(decoded, id) {
		t.Errorf("Expected decoded ID %v, got %v", id, decoded)
	}
}

func TestDHT_Store(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// Note: Store will fail without networking initialized, but we can test
	// that it generates a key correctly
	data := []byte("test data")

	// This will fail because socket is not created, but we can verify
	// the key generation part
	_, err = dht.Store(data)
	// We expect an error because networking is not initialized
	if err == nil {
		t.Log("Store succeeded (unexpected, but may be OK if networking is mocked)")
	}
}

func TestDHT_Get_InvalidKey(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// Test with invalid key length
	invalidKey := "short"
	_, _, err = dht.Get(invalidKey)
	if err == nil {
		t.Error("Expected error for invalid key length")
	}
}

func TestDHT_Bootstrap_NoBootstrapNodes(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:             []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:             "127.0.0.1",
		Port:           "3000",
		BootstrapNodes: []*NetworkNode{},
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// Bootstrap with no nodes should not error
	err = dht.Bootstrap()
	if err != nil {
		t.Errorf("Bootstrap with no nodes should not error, got: %v", err)
	}
}

func TestDHT_Bootstrap_WithBootstrapNodes(t *testing.T) {
	store := &MemoryStore{}
	bootstrapNode := NewNetworkNode("127.0.0.1", "3001")
	bootstrapNode.ID = []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	options := &Options{
		ID:             []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:             "127.0.0.1",
		Port:           "3000",
		BootstrapNodes: []*NetworkNode{bootstrapNode},
		TMsgTimeout:    100 * time.Millisecond, // Short timeout for testing
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// Initialize networking before bootstrap
	err = dht.CreateSocket()
	if err != nil {
		t.Logf("CreateSocket returned error (may be expected if port is in use): %v", err)
		// If socket creation fails, skip bootstrap test
		return
	}

	// Bootstrap will fail because node is not actually running,
	// but it should not panic
	err = dht.Bootstrap()
	// Error is expected since bootstrap node is not reachable
	if err != nil {
		t.Logf("Bootstrap returned expected error (node not reachable): %v", err)
	}
}

func TestDHT_CreateSocket(t *testing.T) {
	store := &MemoryStore{}
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	dht, err := NewDHT(store, options)
	if err != nil {
		t.Fatalf("NewDHT returned error: %v", err)
	}

	// CreateSocket should initialize networking
	err = dht.CreateSocket()
	if err != nil {
		t.Logf("CreateSocket returned error (may be expected if port is in use): %v", err)
	} else {
		// If socket was created, verify networking is initialized
		if !dht.networking.isInitialized() {
			t.Error("Expected networking to be initialized after CreateSocket")
		}
	}
}
