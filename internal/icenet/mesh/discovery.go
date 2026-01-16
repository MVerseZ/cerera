package mesh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/protocol"
)

// DiscoveryMethod represents a peer discovery method
type DiscoveryMethod int

const (
	MethodBootstrap DiscoveryMethod = iota
	MethodPeerExchange
	MethodDHT
)

// Discovery manages peer discovery
type Discovery struct {
	mu            sync.RWMutex
	ctx           context.Context
	peerStore     *PeerStore
	seedNodes     []string // список seed nodes для первоначального подключения
	connManager   *connection.Manager
	encoder       *protocol.Encoder
	decoder       *protocol.Decoder

	// Discovery methods
	enabledMethods []DiscoveryMethod
}

// NewDiscovery creates a new peer discovery
func NewDiscovery(ctx context.Context, peerStore *PeerStore, seedNodes []string, connManager *connection.Manager) *Discovery {
	return &Discovery{
		ctx:            ctx,
		peerStore:      peerStore,
		seedNodes:      seedNodes,
		connManager:    connManager,
		encoder:        protocol.NewEncoder(),
		decoder:        protocol.NewDecoder(),
		enabledMethods: []DiscoveryMethod{MethodPeerExchange, MethodDHT}, // убрали MethodBootstrap
	}
}

// Start starts the discovery process
func (d *Discovery) Start() error {
	// Start seed nodes discovery if enabled
	if len(d.seedNodes) > 0 {
		go d.discoverFromSeedNodes()
	}

	// Start peer exchange if enabled
	if d.isMethodEnabled(MethodPeerExchange) {
		go d.startPeerExchange()
	}

	// Start DHT discovery if enabled
	if d.isMethodEnabled(MethodDHT) {
		go d.discoverFromDHT()
	}

	return nil
}

// Stop stops the discovery process
func (d *Discovery) Stop() {
	// Discovery will stop when context is cancelled
}

// isMethodEnabled checks if a discovery method is enabled
func (d *Discovery) isMethodEnabled(method DiscoveryMethod) bool {
	for _, m := range d.enabledMethods {
		if m == method {
			return true
		}
	}
	return false
}

// discoverFromSeedNodes discovers peers from seed nodes
func (d *Discovery) discoverFromSeedNodes() {
	// TODO: Реализовать подключение к seed nodes и получение списка пиров
	// Это будет реализовано в следующем этапе
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			// Seed nodes discovery will be implemented in seed_discovery task
		}
	}
}

// discoverFromDHT discovers peers using DHT
func (d *Discovery) discoverFromDHT() {
	// TODO: Интегрировать с существующим DHT из internal/mesh
	// Пока что используем периодический поиск через PeerStore
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			// Периодически пытаемся найти новых пиров
			// Это будет улучшено при интеграции с DHT из internal/mesh
			availablePeers := d.DiscoverPeers(5)
			if len(availablePeers) > 0 {
				// Пытаемся подключиться к найденным пирам
				for _, peer := range availablePeers {
					if _, err := d.ConnectToPeer(peer); err != nil {
						continue
					}
				}
			}
		}
	}
}

// startPeerExchange starts peer exchange discovery
func (d *Discovery) startPeerExchange() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			// Exchange peer lists with connected peers
			d.exchangePeerLists()
		}
	}
}

// exchangePeerLists exchanges peer lists with connected peers
func (d *Discovery) exchangePeerLists() {
	// Get connected peers
	connectedPeers := d.peerStore.GetConnected()

	// Limit to a few random peers to avoid flooding
	maxExchange := 3
	if len(connectedPeers) < maxExchange {
		maxExchange = len(connectedPeers)
	}

	// Get our peer list
	ourPeers := d.peerStore.GetBestPeers(10)

	// Create nodes message
	nodes := make([]protocol.NodeInfo, len(ourPeers))
	for i, peer := range ourPeers {
		nodes[i] = protocol.NodeInfo{
			Address:     peer.Address,
			NetworkAddr: peer.NetworkAddr,
		}
	}

	// Send to selected peers (simplified - actual implementation would use protocol messages)
	_ = nodes
	_ = maxExchange
}

// AddPeerFromSeed adds a peer discovered from seed node
func (d *Discovery) AddPeerFromSeed(addr types.Address, networkAddr string) {
	d.peerStore.AddOrUpdate(addr, networkAddr)
}

// AddPeerFromExchange adds a peer discovered from peer exchange
func (d *Discovery) AddPeerFromExchange(addr types.Address, networkAddr string) {
	d.peerStore.AddOrUpdate(addr, networkAddr)
}

// GetPeerStore returns the peer store
func (d *Discovery) GetPeerStore() *PeerStore {
	return d.peerStore
}

// DiscoverPeers attempts to discover new peers
func (d *Discovery) DiscoverPeers(maxPeers int) []*PeerInfo {
	// Get best peers that are not connected
	allPeers := d.peerStore.GetAll()
	availablePeers := make([]*PeerInfo, 0)

	for _, peer := range allPeers {
		if !peer.IsConnected {
			availablePeers = append(availablePeers, peer)
		}
	}

	if len(availablePeers) > maxPeers {
		return availablePeers[:maxPeers]
	}

	return availablePeers
}

// ConnectToPeer attempts to connect to a peer
func (d *Discovery) ConnectToPeer(peer *PeerInfo) (*connection.Connection, error) {
	if peer.NetworkAddr == "" {
		return nil, fmt.Errorf("peer has no network address")
	}

	conn, err := d.connManager.Connect(peer.NetworkAddr, peer.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	// Update peer store
	d.peerStore.UpdateConnectionStatus(peer.Address, true, conn.ID)

	return conn, nil
}
