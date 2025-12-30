package mesh

import (
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// PeerInfo represents information about a peer
type PeerInfo struct {
	Address      types.Address
	NetworkAddr  string
	LastSeen     time.Time
	FirstSeen    time.Time
	IsConnected  bool
	ConnectionID string
	Latency      time.Duration
	Score        float64 // Peer score based on reliability, latency, etc.
}

// PeerStore stores information about known peers
type PeerStore struct {
	mu    sync.RWMutex
	peers map[types.Address]*PeerInfo
}

// NewPeerStore creates a new peer store
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[types.Address]*PeerInfo),
	}
}

// AddOrUpdate adds or updates a peer
func (ps *PeerStore) AddOrUpdate(addr types.Address, networkAddr string) *PeerInfo {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[addr]
	if !exists {
		peer = &PeerInfo{
			Address:     addr,
			NetworkAddr: networkAddr,
			FirstSeen:   time.Now(),
			LastSeen:    time.Now(),
			IsConnected: false,
			Score:       1.0,
		}
		ps.peers[addr] = peer
	} else {
		peer.LastSeen = time.Now()
		if networkAddr != "" && peer.NetworkAddr != networkAddr {
			peer.NetworkAddr = networkAddr
		}
	}

	return peer
}

// Get retrieves a peer by address
func (ps *PeerStore) Get(addr types.Address) (*PeerInfo, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	peer, ok := ps.peers[addr]
	return peer, ok
}

// Remove removes a peer
func (ps *PeerStore) Remove(addr types.Address) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.peers, addr)
}

// GetAll returns all peers
func (ps *PeerStore) GetAll() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetConnected returns all connected peers
func (ps *PeerStore) GetConnected() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peers := make([]*PeerInfo, 0)
	for _, peer := range ps.peers {
		if peer.IsConnected {
			peers = append(peers, peer)
		}
	}
	return peers
}

// GetBestPeers returns the best peers sorted by score
func (ps *PeerStore) GetBestPeers(maxCount int) []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peers = append(peers, peer)
	}

	// Sort by score (simple selection sort for small lists)
	for i := 0; i < len(peers) && i < maxCount; i++ {
		bestIdx := i
		for j := i + 1; j < len(peers); j++ {
			if peers[j].Score > peers[bestIdx].Score {
				bestIdx = j
			}
		}
		peers[i], peers[bestIdx] = peers[bestIdx], peers[i]
	}

	if len(peers) > maxCount {
		return peers[:maxCount]
	}
	return peers
}

// UpdateConnectionStatus updates the connection status of a peer
func (ps *PeerStore) UpdateConnectionStatus(addr types.Address, isConnected bool, connectionID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[addr]
	if exists {
		peer.IsConnected = isConnected
		peer.ConnectionID = connectionID
		if isConnected {
			peer.LastSeen = time.Now()
		}
	}
}

// UpdateLatency updates the latency for a peer
func (ps *PeerStore) UpdateLatency(addr types.Address, latency time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[addr]
	if exists {
		peer.Latency = latency
		// Update score based on latency (lower latency = higher score)
		if latency > 0 {
			// Simple scoring: 1.0 for < 10ms, decreasing for higher latency
			if latency < 10*time.Millisecond {
				peer.Score = 1.0
			} else if latency < 100*time.Millisecond {
				peer.Score = 0.8
			} else if latency < 500*time.Millisecond {
				peer.Score = 0.6
			} else {
				peer.Score = 0.4
			}
		}
	}
}

// UpdateScore updates the score for a peer
func (ps *PeerStore) UpdateScore(addr types.Address, score float64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[addr]
	if exists {
		peer.Score = score
	}
}

// Cleanup removes peers that haven't been seen recently
func (ps *PeerStore) Cleanup(maxAge time.Duration) int {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	now := time.Now()
	removed := 0

	for addr, peer := range ps.peers {
		if !peer.IsConnected && now.Sub(peer.LastSeen) > maxAge {
			delete(ps.peers, addr)
			removed++
		}
	}

	return removed
}

// Size returns the number of peers
func (ps *PeerStore) Size() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

