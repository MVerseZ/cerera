package peers

import (
	"context"
	"sync"
	"time"

	"github.com/cerera/core/types"
	"github.com/cerera/internal/cerera/logger"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// DefaultMaxPeers is the default maximum number of peers
	DefaultMaxPeers = 50
	// DefaultMinPeers is the minimum number of peers to maintain
	DefaultMinPeers = 5
	// PeerCleanupInterval is the interval for cleaning up inactive peers
	PeerCleanupInterval = 5 * time.Minute
	// PeerInactiveTimeout is the timeout for considering a peer inactive
	PeerInactiveTimeout = 10 * time.Minute
)

func peersLogger() *zap.SugaredLogger {
	return logger.Named("peers")
}

// PeerInfo contains information about a connected peer
type PeerInfo struct {
	ID           peer.ID       `json:"id"`
	Address      types.Address `json:"address,omitempty"` // Cerera address
	Height       int           `json:"height"`
	BestHash     string        `json:"bestHash,omitempty"`
	Version      string        `json:"version,omitempty"`
	FirstSeen    time.Time     `json:"firstSeen"`
	LastSeen     time.Time     `json:"lastSeen"`
	LastPingTime time.Duration `json:"lastPingTime,omitempty"`
	Score        float64       `json:"score"`
	IsSyncing    bool          `json:"isSyncing"`
	Direction    string        `json:"direction"` // "inbound" or "outbound"
}

// Manager manages peer connections and information
type Manager struct {
	host           host.Host
	mu             sync.RWMutex
	peers          map[peer.ID]*PeerInfo
	confirmedPeers map[types.Address]*PeerInfo // extends network communication => same as DNS, ip <-> libp2p_addr <-> Cerera address
	banned         map[peer.ID]time.Time
	ctx            context.Context
	cancel         context.CancelFunc
	maxPeers       int
	minPeers       int

	// Callbacks
	onPeerConnected    func(peer.ID)
	onPeerDisconnected func(peer.ID)
}

// NewManager creates a new peer manager
func NewManager(ctx context.Context, h host.Host, maxPeers int) *Manager {
	if maxPeers <= 0 {
		maxPeers = DefaultMaxPeers
	}

	ctx, cancel := context.WithCancel(ctx)

	m := &Manager{
		host:           h,
		peers:          make(map[peer.ID]*PeerInfo),
		confirmedPeers: make(map[types.Address]*PeerInfo),
		banned:         make(map[peer.ID]time.Time),
		ctx:            ctx,
		cancel:         cancel,
		maxPeers:       maxPeers,
		minPeers:       DefaultMinPeers,
	}

	// Setup connection notifier
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF:    m.handleConnect,
		DisconnectedF: m.handleDisconnect,
	})

	return m
}

// Start starts the peer manager
func (m *Manager) Start() {
	go m.cleanupLoop()
	peersLogger().Infow("Peer manager started", "maxPeers", m.maxPeers)
}

// Stop stops the peer manager
func (m *Manager) Stop() {
	m.cancel()
	peersLogger().Infow("Peer manager stopped")
}

// handleConnect handles new peer connections
func (m *Manager) handleConnect(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	// Check if banned
	m.mu.RLock()
	banTime, isBanned := m.banned[peerID]
	m.mu.RUnlock()

	if isBanned && time.Now().Before(banTime) {
		peersLogger().Warnw("Banned peer tried to connect, disconnecting",
			"peer", peerID,
			"banExpires", banTime,
		)
		conn.Close()
		return
	}

	// Check peer limit
	m.mu.RLock()
	peerCount := len(m.peers)
	m.mu.RUnlock()

	if peerCount >= m.maxPeers {
		peersLogger().Warnw("Max peers reached, rejecting connection",
			"peer", peerID,
			"maxPeers", m.maxPeers,
		)
		conn.Close()
		return
	}

	// Determine direction
	direction := "inbound"
	if conn.Stat().Direction == network.DirOutbound {
		direction = "outbound"
	}

	// Add peer
	m.mu.Lock()
	if _, exists := m.peers[peerID]; !exists {
		m.peers[peerID] = &PeerInfo{
			ID:        peerID,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			Score:     InitialScore,
			Direction: direction,
		}
	}
	m.mu.Unlock()

	peersLogger().Infow("[PEERS MANAGER] Peer connected",
		"peer", peerID,
		"direction", direction,
		"totalPeers", m.GetPeerCount(),
	)

	// Call callback
	if m.onPeerConnected != nil {
		go m.onPeerConnected(peerID)
	}
}

// handleDisconnect handles peer disconnections
func (m *Manager) handleDisconnect(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	m.mu.Lock()
	delete(m.peers, peerID)
	m.mu.Unlock()

	peersLogger().Infow("[PEERSMANAGER] Peer disconnected",
		"peer", peerID,
		"totalPeers", m.GetPeerCount(),
	)

	// Call callback
	if m.onPeerDisconnected != nil {
		go m.onPeerDisconnected(peerID)
	}
}

// cleanupLoop periodically cleans up inactive peers and expired bans
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(PeerCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup removes inactive peers and expired bans
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Clean up inactive peers
	for peerID, info := range m.peers {
		if now.Sub(info.LastSeen) > PeerInactiveTimeout {
			// Check if actually disconnected
			if m.host.Network().Connectedness(peerID) != network.Connected {
				delete(m.peers, peerID)
				peersLogger().Debugw("Removed inactive peer", "peer", peerID)
			}
		}
	}

	// Clean up expired bans
	for peerID, banTime := range m.banned {
		if now.After(banTime) {
			delete(m.banned, peerID)
			peersLogger().Debugw("Ban expired", "peer", peerID)
		}
	}
}

// UpdatePeerInfo updates information about a peer
func (m *Manager) UpdatePeerInfo(peerID peer.ID, height int, bestHash string, version string, address types.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, exists := m.peers[peerID]; exists {
		info.Height = height
		info.BestHash = bestHash
		info.Version = version
		info.Address = address
		info.LastSeen = time.Now()
	}
}

// UpdatePeerHeight updates just the height of a peer
func (m *Manager) UpdatePeerHeight(peerID peer.ID, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, exists := m.peers[peerID]; exists {
		info.Height = height
		info.LastSeen = time.Now()
	}
}

// UpdatePeerPing updates the last ping time for a peer
func (m *Manager) UpdatePeerPing(peerID peer.ID, pingTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, exists := m.peers[peerID]; exists {
		info.LastPingTime = pingTime
		info.LastSeen = time.Now()
	}
}

// MarkPeerSyncing marks a peer as syncing or not
func (m *Manager) MarkPeerSyncing(peerID peer.ID, isSyncing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, exists := m.peers[peerID]; exists {
		info.IsSyncing = isSyncing
	}
}

// GetPeer returns information about a peer
func (m *Manager) GetPeer(peerID peer.ID) *PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.peers[peerID]; exists {
		// Return a copy
		copy := *info
		return &copy
	}
	return nil
}

// GetPeers returns all connected peers
func (m *Manager) GetPeers() []*PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(m.peers))
	for _, info := range m.peers {
		copy := *info
		peers = append(peers, &copy)
	}
	return peers
}

// GetPeerIDs returns all connected peer IDs
func (m *Manager) GetPeerIDs() []peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]peer.ID, 0, len(m.peers))
	for id := range m.peers {
		ids = append(ids, id)
	}
	return ids
}

// GetPeerCount returns the number of connected peers
func (m *Manager) GetPeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// GetBestPeer returns the peer with the highest height
func (m *Manager) GetBestPeer() *PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *PeerInfo
	for _, info := range m.peers {
		if best == nil || info.Height > best.Height {
			copy := *info
			best = &copy
		}
	}
	return best
}

// GetPeersAboveHeight returns peers with height greater than the given height
func (m *Manager) GetPeersAboveHeight(height int) []*PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]*PeerInfo, 0)
	for _, info := range m.peers {
		if info.Height > height {
			copy := *info
			peers = append(peers, &copy)
		}
	}
	return peers
}

// BanPeer bans a peer for the specified duration
func (m *Manager) BanPeer(peerID peer.ID, duration time.Duration, reason string) {
	m.mu.Lock()
	m.banned[peerID] = time.Now().Add(duration)
	delete(m.peers, peerID)
	m.mu.Unlock()

	// Disconnect the peer
	if err := m.host.Network().ClosePeer(peerID); err != nil {
		peersLogger().Warnw("Failed to disconnect banned peer", "peer", peerID, "error", err)
	}

	peersLogger().Warnw("Peer banned",
		"peer", peerID,
		"duration", duration,
		"reason", reason,
	)
}

// UnbanPeer removes a ban from a peer
func (m *Manager) UnbanPeer(peerID peer.ID) {
	m.mu.Lock()
	delete(m.banned, peerID)
	m.mu.Unlock()

	peersLogger().Infow("Peer unbanned", "peer", peerID)
}

// IsBanned checks if a peer is banned
func (m *Manager) IsBanned(peerID peer.ID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	banTime, exists := m.banned[peerID]
	return exists && time.Now().Before(banTime)
}

// SetOnPeerConnected sets the callback for peer connections
func (m *Manager) SetOnPeerConnected(callback func(peer.ID)) {
	m.onPeerConnected = callback
}

// SetOnPeerDisconnected sets the callback for peer disconnections
func (m *Manager) SetOnPeerDisconnected(callback func(peer.ID)) {
	m.onPeerDisconnected = callback
}

// NeedMorePeers returns true if the peer count is below minimum
func (m *Manager) NeedMorePeers() bool {
	return m.GetPeerCount() < m.minPeers
}

// CanAcceptPeer returns true if we can accept more peers
func (m *Manager) CanAcceptPeer() bool {
	return m.GetPeerCount() < m.maxPeers
}
