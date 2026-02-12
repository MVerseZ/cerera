package peers

import (
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// InitialScore is the starting score for new peers
	InitialScore = 100.0
	// MaxScore is the maximum peer score
	MaxScore = 200.0
	// MinScore is the minimum peer score (below this, peer is banned)
	MinScore = 0.0
	// BanThreshold is the score threshold for banning
	BanThreshold = 10.0
	// BanDuration is the default ban duration
	BanDuration = 24 * time.Hour

	// Score adjustments
	ScoreValidBlock      = 5.0   // Valid block received
	ScoreInvalidBlock    = -20.0 // Invalid block received
	ScoreValidTx         = 1.0   // Valid transaction received
	ScoreInvalidTx       = -5.0  // Invalid transaction received
	ScoreSyncHelp        = 10.0  // Helped with sync
	ScoreTimeout         = -3.0  // Request timeout
	ScoreMisbehavior     = -15.0 // Protocol misbehavior
	ScoreGoodLatency     = 2.0   // Good ping latency (< 100ms)
	ScoreBadLatency      = -1.0  // Bad ping latency (> 500ms)
	ScoreDisconnect      = -2.0  // Unexpected disconnect
	ScoreConsensusHelp   = 3.0   // Participated in consensus
	ScoreConsensusFailed = -10.0 // Failed consensus contribution
)

// ScoreReason represents the reason for a score change
type ScoreReason string

const (
	ReasonValidBlock      ScoreReason = "valid_block"
	ReasonInvalidBlock    ScoreReason = "invalid_block"
	ReasonValidTx         ScoreReason = "valid_tx"
	ReasonInvalidTx       ScoreReason = "invalid_tx"
	ReasonSyncHelp        ScoreReason = "sync_help"
	ReasonTimeout         ScoreReason = "timeout"
	ReasonMisbehavior     ScoreReason = "misbehavior"
	ReasonGoodLatency     ScoreReason = "good_latency"
	ReasonBadLatency      ScoreReason = "bad_latency"
	ReasonDisconnect      ScoreReason = "disconnect"
	ReasonConsensusHelp   ScoreReason = "consensus_help"
	ReasonConsensusFailed ScoreReason = "consensus_failed"
)

// ScoreChange represents a change in peer score
type ScoreChange struct {
	PeerID    peer.ID
	OldScore  float64
	NewScore  float64
	Change    float64
	Reason    ScoreReason
	Timestamp time.Time
}

// Scorer manages peer scoring
type Scorer struct {
	manager   *Manager
	mu        sync.RWMutex
	history   map[peer.ID][]ScoreChange
	maxHistory int

	// Callbacks
	onScoreChange func(ScoreChange)
	onBan         func(peer.ID, ScoreReason)
}

// NewScorer creates a new peer scorer
func NewScorer(manager *Manager) *Scorer {
	return &Scorer{
		manager:    manager,
		history:    make(map[peer.ID][]ScoreChange),
		maxHistory: 100, // Keep last 100 changes per peer
	}
}

// AdjustScore adjusts a peer's score by the given delta
func (s *Scorer) AdjustScore(peerID peer.ID, delta float64, reason ScoreReason) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := s.manager.GetPeer(peerID)
	if info == nil {
		return 0
	}

	oldScore := info.Score
	newScore := math.Max(MinScore, math.Min(MaxScore, oldScore+delta))

	// Update peer score
	s.manager.mu.Lock()
	if peerInfo, exists := s.manager.peers[peerID]; exists {
		peerInfo.Score = newScore
	}
	s.manager.mu.Unlock()

	// Record history
	change := ScoreChange{
		PeerID:    peerID,
		OldScore:  oldScore,
		NewScore:  newScore,
		Change:    delta,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	if s.history[peerID] == nil {
		s.history[peerID] = make([]ScoreChange, 0, s.maxHistory)
	}
	s.history[peerID] = append(s.history[peerID], change)

	// Trim history if needed
	if len(s.history[peerID]) > s.maxHistory {
		s.history[peerID] = s.history[peerID][len(s.history[peerID])-s.maxHistory:]
	}

	// Call callback
	if s.onScoreChange != nil {
		go s.onScoreChange(change)
	}

	// Check if peer should be banned
	if newScore <= BanThreshold {
		s.banPeer(peerID, reason)
	}

	peersLogger().Debugw("Score adjusted",
		"peer", peerID,
		"oldScore", oldScore,
		"newScore", newScore,
		"delta", delta,
		"reason", reason,
	)

	return newScore
}

// banPeer bans a peer due to low score
func (s *Scorer) banPeer(peerID peer.ID, reason ScoreReason) {
	s.manager.BanPeer(peerID, BanDuration, string(reason))

	if s.onBan != nil {
		go s.onBan(peerID, reason)
	}
}

// RecordValidBlock records that a peer sent a valid block
func (s *Scorer) RecordValidBlock(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreValidBlock, ReasonValidBlock)
}

// RecordInvalidBlock records that a peer sent an invalid block
func (s *Scorer) RecordInvalidBlock(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreInvalidBlock, ReasonInvalidBlock)
}

// RecordValidTx records that a peer sent a valid transaction
func (s *Scorer) RecordValidTx(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreValidTx, ReasonValidTx)
}

// RecordInvalidTx records that a peer sent an invalid transaction
func (s *Scorer) RecordInvalidTx(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreInvalidTx, ReasonInvalidTx)
}

// RecordSyncHelp records that a peer helped with syncing
func (s *Scorer) RecordSyncHelp(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreSyncHelp, ReasonSyncHelp)
}

// RecordTimeout records that a request to a peer timed out
func (s *Scorer) RecordTimeout(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreTimeout, ReasonTimeout)
}

// RecordMisbehavior records that a peer misbehaved
func (s *Scorer) RecordMisbehavior(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreMisbehavior, ReasonMisbehavior)
}

// RecordLatency records the latency of a peer
func (s *Scorer) RecordLatency(peerID peer.ID, latency time.Duration) float64 {
	if latency < 100*time.Millisecond {
		return s.AdjustScore(peerID, ScoreGoodLatency, ReasonGoodLatency)
	} else if latency > 500*time.Millisecond {
		return s.AdjustScore(peerID, ScoreBadLatency, ReasonBadLatency)
	}
	return s.GetScore(peerID)
}

// RecordDisconnect records that a peer disconnected unexpectedly
func (s *Scorer) RecordDisconnect(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreDisconnect, ReasonDisconnect)
}

// RecordConsensusHelp records that a peer participated in consensus
func (s *Scorer) RecordConsensusHelp(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreConsensusHelp, ReasonConsensusHelp)
}

// RecordConsensusFailed records that a peer failed to contribute to consensus
func (s *Scorer) RecordConsensusFailed(peerID peer.ID) float64 {
	return s.AdjustScore(peerID, ScoreConsensusFailed, ReasonConsensusFailed)
}

// GetScore returns the current score of a peer
func (s *Scorer) GetScore(peerID peer.ID) float64 {
	info := s.manager.GetPeer(peerID)
	if info == nil {
		return 0
	}
	return info.Score
}

// GetScoreHistory returns the score history for a peer
func (s *Scorer) GetScoreHistory(peerID peer.ID) []ScoreChange {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, exists := s.history[peerID]
	if !exists {
		return nil
	}

	// Return a copy
	result := make([]ScoreChange, len(history))
	copy(result, history)
	return result
}

// SetOnScoreChange sets the callback for score changes
func (s *Scorer) SetOnScoreChange(callback func(ScoreChange)) {
	s.onScoreChange = callback
}

// SetOnBan sets the callback for peer bans
func (s *Scorer) SetOnBan(callback func(peer.ID, ScoreReason)) {
	s.onBan = callback
}

// GetTopPeers returns the top N peers by score
func (s *Scorer) GetTopPeers(n int) []*PeerInfo {
	peers := s.manager.GetPeers()

	// Sort by score (descending)
	for i := 0; i < len(peers)-1; i++ {
		for j := i + 1; j < len(peers); j++ {
			if peers[j].Score > peers[i].Score {
				peers[i], peers[j] = peers[j], peers[i]
			}
		}
	}

	if n > len(peers) {
		n = len(peers)
	}

	return peers[:n]
}

// GetPeersAboveScore returns peers with score above the threshold
func (s *Scorer) GetPeersAboveScore(threshold float64) []*PeerInfo {
	peers := s.manager.GetPeers()
	result := make([]*PeerInfo, 0)

	for _, p := range peers {
		if p.Score >= threshold {
			result = append(result, p)
		}
	}

	return result
}
