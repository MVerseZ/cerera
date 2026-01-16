package mesh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/protocol"
)

// NetworkConfig holds configuration for mesh network
type NetworkConfig struct {
	MaxConnections    int
	MinConnections    int
	ConnectionTimeout time.Duration
	GossipInterval    time.Duration
	PeerCleanupInterval time.Duration
	PeerMaxAge        time.Duration
}

// DefaultNetworkConfig returns default network configuration
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		MaxConnections:      16,
		MinConnections:      4,
		ConnectionTimeout:   10 * time.Second,
		GossipInterval:      5 * time.Second,
		PeerCleanupInterval: 60 * time.Second,
		PeerMaxAge:          24 * time.Hour,
	}
}

// Network represents a mesh network
type Network struct {
	mu          sync.RWMutex
	ctx         context.Context
	config      *NetworkConfig
	peerStore   *PeerStore
	discovery   *Discovery
	connManager *connection.Manager
	encoder     *protocol.Encoder
	decoder     *protocol.Decoder
	gossip      *Gossip
	started     bool
}

// NewNetwork creates a new mesh network
func NewNetwork(ctx context.Context, config *NetworkConfig, peerStore *PeerStore, discovery *Discovery, connManager *connection.Manager) *Network {
	if config == nil {
		config = DefaultNetworkConfig()
	}
	
	return &Network{
		ctx:         ctx,
		config:      config,
		peerStore:   peerStore,
		discovery:   discovery,
		connManager: connManager,
		encoder:     protocol.NewEncoder(),
		decoder:     protocol.NewDecoder(),
		gossip:      NewGossip(ctx, peerStore, connManager),
		started:     false,
	}
}

// Start starts the mesh network
func (n *Network) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if n.started {
		return fmt.Errorf("network already started")
	}
	
	// Start discovery
	if err := n.discovery.Start(); err != nil {
		return fmt.Errorf("failed to start discovery: %w", err)
	}
	
	// Start gossip
	if err := n.gossip.Start(); err != nil {
		return fmt.Errorf("failed to start gossip: %w", err)
	}
	
	// Start connection management
	go n.manageConnections()
	
	// Start peer cleanup
	go n.cleanupPeers()
	
	// Update network topology metrics
	metrics.Get().UpdateNetworkTopologyNodes(n.peerStore.Size())
	
	n.started = true
	return nil
}

// Stop stops the mesh network
func (n *Network) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if !n.started {
		return nil
	}
	
	n.gossip.Stop()
	n.discovery.Stop()
	
	n.started = false
	return nil
}

// manageConnections manages peer connections to maintain target connection count
func (n *Network) manageConnections() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.ensureConnections()
		}
	}
}

// ensureConnections ensures we have the right number of connections
func (n *Network) ensureConnections() {
	connected := n.peerStore.GetConnected()
	connectedCount := len(connected)
	
	// If we have too few connections, try to connect to more peers
	if connectedCount < n.config.MinConnections {
		needed := n.config.MinConnections - connectedCount
		peers := n.discovery.DiscoverPeers(needed)
		
		for _, peer := range peers {
			if connectedCount >= n.config.MaxConnections {
				break
			}
			
			_, err := n.discovery.ConnectToPeer(peer)
			if err != nil {
				continue
			}
			connectedCount++
		}
	}
	
	// If we have too many connections, disconnect from worst peers
	if connectedCount > n.config.MaxConnections {
		allConnected := n.peerStore.GetConnected()
		// Sort by score and disconnect worst ones
		toDisconnect := connectedCount - n.config.MaxConnections
		
		for i := 0; i < toDisconnect && i < len(allConnected); i++ {
			peer := allConnected[i]
			if conn, ok := n.connManager.GetConnectionByAddress(peer.Address); ok {
				conn.Close()
				n.connManager.RemoveConnectionByAddress(peer.Address)
				n.peerStore.UpdateConnectionStatus(peer.Address, false, "")
			}
		}
	}
}

// cleanupPeers periodically cleans up old peers
func (n *Network) cleanupPeers() {
	ticker := time.NewTicker(n.config.PeerCleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			removed := n.peerStore.Cleanup(n.config.PeerMaxAge)
			if removed > 0 {
				// Log cleanup if needed
			}
		}
	}
}

// BroadcastBlock broadcasts a block to all connected peers
func (n *Network) BroadcastBlock(b *block.Block) error {
	return n.gossip.BroadcastBlock(b)
}

// BroadcastMessage broadcasts a protocol message to all connected peers
func (n *Network) BroadcastMessage(msg protocol.Message) error {
	return n.gossip.BroadcastMessage(msg)
}

// GetConnectedPeers returns all connected peers
func (n *Network) GetConnectedPeers() []*PeerInfo {
	return n.peerStore.GetConnected()
}

// GetPeerCount returns the number of known peers
func (n *Network) GetPeerCount() int {
	count := n.peerStore.Size()
	metrics.Get().UpdateNetworkTopologyNodes(count)
	return count
}

// IsStarted returns whether the network is started
func (n *Network) IsStarted() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.started
}

