package sync

import (
	"sync"
	"time"

	"github.com/cerera/core/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SyncState represents the current synchronization state
type SyncState int

const (
	// StateIdle means no synchronization is happening
	StateIdle SyncState = iota
	// StateSyncing means synchronization is in progress
	StateSyncing
	// StateDownloading means blocks are being downloaded
	StateDownloading
	// StateValidating means downloaded blocks are being validated
	StateValidating
	// StateCatchingUp means catching up with the network after being offline
	StateCatchingUp
)

// String returns the string representation of the sync state
func (s SyncState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateSyncing:
		return "Syncing"
	case StateDownloading:
		return "Downloading"
	case StateValidating:
		return "Validating"
	case StateCatchingUp:
		return "CatchingUp"
	default:
		return "Unknown"
	}
}

// SyncProgress tracks the progress of block synchronization
type SyncProgress struct {
	mu              sync.RWMutex
	State           SyncState   `json:"state"`
	StartHeight     int         `json:"startHeight"`
	CurrentHeight   int         `json:"currentHeight"`
	TargetHeight    int         `json:"targetHeight"`
	StartTime       time.Time   `json:"startTime"`
	LastUpdateTime  time.Time   `json:"lastUpdateTime"`
	BlocksReceived  int         `json:"blocksReceived"`
	BlocksProcessed int         `json:"blocksProcessed"`
	BytesReceived   int64       `json:"bytesReceived"`
	SyncingPeer     peer.ID     `json:"syncingPeer,omitempty"`
	LastBlockHash   common.Hash `json:"lastBlockHash,omitempty"`
	ErrorCount      int         `json:"errorCount"`
	LastError       string      `json:"lastError,omitempty"`
}

// NewSyncProgress creates a new sync progress tracker
func NewSyncProgress() *SyncProgress {
	return &SyncProgress{
		State:          StateIdle,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}
}

// SetState sets the current sync state
func (p *SyncProgress) SetState(state SyncState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = state
	p.LastUpdateTime = time.Now()
}

// GetState returns the current sync state
func (p *SyncProgress) GetState() SyncState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// StartSync starts a new synchronization session
func (p *SyncProgress) StartSync(startHeight, targetHeight int, peerID peer.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = StateSyncing
	p.StartHeight = startHeight
	p.CurrentHeight = startHeight
	p.TargetHeight = targetHeight
	p.StartTime = time.Now()
	p.LastUpdateTime = time.Now()
	p.BlocksReceived = 0
	p.BlocksProcessed = 0
	p.BytesReceived = 0
	p.SyncingPeer = peerID
	p.ErrorCount = 0
	p.LastError = ""
}

// UpdateProgress updates the sync progress
func (p *SyncProgress) UpdateProgress(currentHeight int, blocksReceived, blocksProcessed int, bytesReceived int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CurrentHeight = currentHeight
	p.BlocksReceived += blocksReceived
	p.BlocksProcessed += blocksProcessed
	p.BytesReceived += bytesReceived
	p.LastUpdateTime = time.Now()
}

// SetLastBlockHash sets the hash of the last processed block
func (p *SyncProgress) SetLastBlockHash(hash common.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastBlockHash = hash
	p.LastUpdateTime = time.Now()
}

// RecordError records a sync error
func (p *SyncProgress) RecordError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ErrorCount++
	p.LastError = err
	p.LastUpdateTime = time.Now()
}

// Complete marks synchronization as complete
func (p *SyncProgress) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = StateIdle
	p.LastUpdateTime = time.Now()
}

// GetProgress returns a copy of the current progress
func (p *SyncProgress) GetProgress() SyncProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return SyncProgress{
		State:           p.State,
		StartHeight:     p.StartHeight,
		CurrentHeight:   p.CurrentHeight,
		TargetHeight:    p.TargetHeight,
		StartTime:       p.StartTime,
		LastUpdateTime:  p.LastUpdateTime,
		BlocksReceived:  p.BlocksReceived,
		BlocksProcessed: p.BlocksProcessed,
		BytesReceived:   p.BytesReceived,
		SyncingPeer:     p.SyncingPeer,
		LastBlockHash:   p.LastBlockHash,
		ErrorCount:      p.ErrorCount,
		LastError:       p.LastError,
	}
}

// Percentage returns the sync progress percentage
func (p *SyncProgress) Percentage() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.TargetHeight <= p.StartHeight {
		return 100.0
	}

	total := float64(p.TargetHeight - p.StartHeight)
	current := float64(p.CurrentHeight - p.StartHeight)
	return (current / total) * 100.0
}

// BlocksRemaining returns the number of blocks remaining to sync
func (p *SyncProgress) BlocksRemaining() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	remaining := p.TargetHeight - p.CurrentHeight
	if remaining < 0 {
		return 0
	}
	return remaining
}

// EstimatedTimeRemaining estimates the time remaining based on current rate
func (p *SyncProgress) EstimatedTimeRemaining() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.BlocksProcessed == 0 {
		return 0
	}

	elapsed := time.Since(p.StartTime)
	rate := float64(p.BlocksProcessed) / elapsed.Seconds()
	if rate == 0 {
		return 0
	}

	remaining := p.TargetHeight - p.CurrentHeight
	return time.Duration(float64(remaining)/rate) * time.Second
}

// IsSyncing returns true if synchronization is in progress
func (p *SyncProgress) IsSyncing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State != StateIdle
}

// PeerSyncState tracks the sync state with a specific peer
type PeerSyncState struct {
	PeerID         peer.ID
	Height         int
	BestHash       common.Hash
	LastSyncTime   time.Time
	BlocksReceived int
	FailureCount   int
	Latency        time.Duration
	IsUsable       bool
}

// PeerSyncTracker tracks sync state across multiple peers
type PeerSyncTracker struct {
	mu    sync.RWMutex
	peers map[peer.ID]*PeerSyncState
}

// NewPeerSyncTracker creates a new peer sync tracker
func NewPeerSyncTracker() *PeerSyncTracker {
	return &PeerSyncTracker{
		peers: make(map[peer.ID]*PeerSyncState),
	}
}

// UpdatePeer updates or adds a peer's sync state
func (t *PeerSyncTracker) UpdatePeer(peerID peer.ID, height int, bestHash common.Hash) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.peers[peerID]; exists {
		state.Height = height
		state.BestHash = bestHash
		state.LastSyncTime = time.Now()
	} else {
		t.peers[peerID] = &PeerSyncState{
			PeerID:       peerID,
			Height:       height,
			BestHash:     bestHash,
			LastSyncTime: time.Now(),
			IsUsable:     true,
		}
	}
}

// RecordBlocksReceived records blocks received from a peer
func (t *PeerSyncTracker) RecordBlocksReceived(peerID peer.ID, count int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.peers[peerID]; exists {
		state.BlocksReceived += count
		state.LastSyncTime = time.Now()
	}
}

// RecordFailure records a sync failure for a peer
func (t *PeerSyncTracker) RecordFailure(peerID peer.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.peers[peerID]; exists {
		state.FailureCount++
		if state.FailureCount >= 3 {
			state.IsUsable = false
		}
	}
}

// RecordLatency records latency for a peer
func (t *PeerSyncTracker) RecordLatency(peerID peer.ID, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.peers[peerID]; exists {
		state.Latency = latency
	}
}

// GetPeer returns the sync state for a peer
func (t *PeerSyncTracker) GetPeer(peerID peer.ID) *PeerSyncState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if state, exists := t.peers[peerID]; exists {
		copy := *state
		return &copy
	}
	return nil
}

// GetBestPeers returns peers sorted by height (descending), filtered by usability
func (t *PeerSyncTracker) GetBestPeers(minHeight int) []*PeerSyncState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	peers := make([]*PeerSyncState, 0)
	for _, state := range t.peers {
		if state.IsUsable && state.Height >= minHeight {
			copy := *state
			peers = append(peers, &copy)
		}
	}

	// Sort by height (descending), then by latency (ascending)
	for i := 0; i < len(peers)-1; i++ {
		for j := i + 1; j < len(peers); j++ {
			if peers[j].Height > peers[i].Height ||
				(peers[j].Height == peers[i].Height && peers[j].Latency < peers[i].Latency) {
				peers[i], peers[j] = peers[j], peers[i]
			}
		}
	}

	return peers
}

// RemovePeer removes a peer from tracking
func (t *PeerSyncTracker) RemovePeer(peerID peer.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.peers, peerID)
}

// ResetPeer resets failure count and marks peer as usable
func (t *PeerSyncTracker) ResetPeer(peerID peer.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.peers[peerID]; exists {
		state.FailureCount = 0
		state.IsUsable = true
	}
}
