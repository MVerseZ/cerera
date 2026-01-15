package mesh

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	dhtmesh "github.com/cerera/internal/mesh"
)

// DHTAdapter adapts the existing DHT from internal/mesh to work with icenet/mesh
type DHTAdapter struct {
	mu          sync.RWMutex
	ctx         context.Context
	dht         *dhtmesh.DHT
	peerStore   *PeerStore
	connManager  *connection.Manager
	seedNodes   []string
	started     bool
}

// NewDHTAdapter creates a new DHT adapter
func NewDHTAdapter(ctx context.Context, peerStore *PeerStore, connManager *connection.Manager, seedNodes []string, nodeID []byte, ip string, port string) (*DHTAdapter, error) {
	// Convert seed nodes to NetworkNode format
	bootstrapNodes := make([]*dhtmesh.NetworkNode, 0, len(seedNodes))
	for _, seed := range seedNodes {
		host, portStr, err := net.SplitHostPort(seed)
		if err != nil {
			continue
		}
		bootstrapNodes = append(bootstrapNodes, dhtmesh.NewNetworkNode(host, portStr))
	}

	// Create DHT options
	options := &dhtmesh.Options{
		ID:             nodeID,
		IP:             ip,
		Port:           port,
		UseStun:        false,
		BootstrapNodes: bootstrapNodes,
		TExpire:        time.Second * 86410,
		TRefresh:       time.Second * 3600,
		TReplicate:     time.Second * 3600,
		TRepublish:     time.Second * 86400,
		TPingMax:       time.Second * 1,
		TMsgTimeout:    time.Second * 2,
	}

	// Create DHT with memory store
	store := &dhtmesh.MemoryStore{}
	dht, err := dhtmesh.NewDHT(store, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	return &DHTAdapter{
		ctx:        ctx,
		dht:        dht,
		peerStore:  peerStore,
		connManager: connManager,
		seedNodes:  seedNodes,
		started:    false,
	}, nil
}

// Start starts the DHT adapter
func (a *DHTAdapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return fmt.Errorf("DHT adapter already started")
	}

	// Create socket
	if err := a.dht.CreateSocket(); err != nil {
		return fmt.Errorf("failed to create DHT socket: %w", err)
	}

	// Start listening
	if err := a.dht.Listen(); err != nil {
		return fmt.Errorf("failed to start DHT listener: %w", err)
	}

	// Bootstrap DHT if we have seed nodes
	if len(a.seedNodes) > 0 {
		go func() {
			if err := a.dht.Bootstrap(); err != nil {
				// Log error but don't fail - DHT can work without bootstrap
			}
		}()
	}

	// Start periodic discovery
	go a.periodicDiscovery()

	a.started = true
	return nil
}

// Stop stops the DHT adapter
func (a *DHTAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return nil
	}

	if a.dht != nil {
		if err := a.dht.Close(); err != nil {
			return err
		}
	}

	a.started = false
	return nil
}

// DiscoverPeers discovers peers using DHT
func (a *DHTAdapter) DiscoverPeers(maxPeers int) ([]*PeerInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.started || a.dht == nil {
		return nil, fmt.Errorf("DHT adapter not started")
	}

	// Use DHT to find closest nodes to our own ID
	// We'll use our own ID as the target to find nodes near us
	// Note: DHT discovery is handled through periodic discovery
	
	// Decode self ID to bytes for iteration
	// The DHT uses base58 encoding, but we need bytes for iteration
	// For now, we'll use a simple approach: find nodes by doing a lookup
	
	// Get number of nodes in DHT
	numNodes := a.dht.NumNodes()
	if numNodes == 0 {
		return nil, nil
	}

	// We can't directly get all nodes from DHT, so we'll use the peer store
	// which should be populated by periodic discovery
	peers := a.peerStore.GetAll()
	
	// Limit to maxPeers
	if len(peers) > maxPeers {
		peers = peers[:maxPeers]
	}

	return peers, nil
}

// periodicDiscovery periodically discovers peers from DHT and adds them to peer store
func (a *DHTAdapter) periodicDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.discoverAndAddPeers()
		}
	}
}

// discoverAndAddPeers discovers peers from DHT routing table and adds them to peer store
func (a *DHTAdapter) discoverAndAddPeers() {
	// The DHT doesn't expose its routing table directly, so we need to work around this
	// For now, we'll trigger a find node operation to discover peers
	// In a real implementation, we might need to modify the DHT to expose routing table
	
	// We can't directly access the routing table, so we'll use a workaround:
	// Do a find node operation with a random target to populate the DHT
	// This will cause the DHT to discover and store nodes
	
	// For now, we'll rely on the DHT's internal mechanisms to discover nodes
	// and we'll add a method to extract nodes from DHT when they're discovered
	
	// Since we can't directly access the routing table, we'll use a different approach:
	// When DHT discovers nodes through its normal operations, we need to hook into that
	// For now, this is a placeholder - in a full implementation, we'd need to modify
	// the DHT to call back when nodes are discovered
}

// AddDiscoveredNode adds a node discovered by DHT to the peer store
func (a *DHTAdapter) AddDiscoveredNode(addr types.Address, networkAddr string) {
	a.peerStore.AddOrUpdate(addr, networkAddr)
}

// GetNetworkAddr returns the network address from DHT
func (a *DHTAdapter) GetNetworkAddr() string {
	if a.dht == nil {
		return ""
	}
	return a.dht.GetNetworkAddr()
}

// IsStarted returns whether the adapter is started
func (a *DHTAdapter) IsStarted() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.started
}

// ConvertNetworkNodeToPeerInfo converts a DHT NetworkNode to PeerInfo
func ConvertNetworkNodeToPeerInfo(node *dhtmesh.NetworkNode) (*PeerInfo, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}

	// Convert node ID to Address
	// The node ID is a byte array, we need to convert it to types.Address
	// For now, we'll create a simple conversion
	// In a real implementation, we might need a better mapping
	
	// Create address from node ID (assuming first 20 bytes are used for address)
	var addr types.Address
	if len(node.ID) >= 20 {
		copy(addr[:], node.ID[:20])
	} else {
		// Pad with zeros if needed
		copy(addr[:], node.ID)
	}

	networkAddr := fmt.Sprintf("%s:%d", node.IP.String(), node.Port)

	return &PeerInfo{
		Address:     addr,
		NetworkAddr: networkAddr,
		LastSeen:    time.Now(),
		FirstSeen:   time.Now(),
		IsConnected: false,
		Score:       1.0,
	}, nil
}

// ConvertSeedNodesToNetworkNodes converts seed node strings to NetworkNode format
func ConvertSeedNodesToNetworkNodes(seedNodes []string) []*dhtmesh.NetworkNode {
	nodes := make([]*dhtmesh.NetworkNode, 0, len(seedNodes))
	for _, seed := range seedNodes {
		host, portStr, err := net.SplitHostPort(seed)
		if err != nil {
			continue
		}
		nodes = append(nodes, dhtmesh.NewNetworkNode(host, portStr))
	}
	return nodes
}
