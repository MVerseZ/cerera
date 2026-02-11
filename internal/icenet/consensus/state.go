package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ConsensusState represents the current state of consensus
type ConsensusState int

const (
	// StateIdle means no consensus is in progress
	StateIdle ConsensusState = iota
	// StatePrePrepare means waiting for pre-prepare phase
	StatePrePrepare
	// StatePrepare means in prepare phase
	StatePrepare
	// StateCommit means in commit phase
	StateCommit
	// StateFinalized means block is finalized
	StateFinalized
)

// String returns the string representation of the consensus state
func (s ConsensusState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StatePrePrepare:
		return "PrePrepare"
	case StatePrepare:
		return "Prepare"
	case StateCommit:
		return "Commit"
	case StateFinalized:
		return "Finalized"
	default:
		return "Unknown"
	}
}

// VoteType represents the type of vote
type VoteType int

const (
	VoteApprove VoteType = iota
	VoteReject
	VoteAbstain
)

// Vote represents a single vote from a validator
type Vote struct {
	BlockHash   common.Hash `json:"blockHash"`
	BlockHeight int         `json:"blockHeight"`
	ViewID      int64       `json:"viewId"`
	SequenceID  int64       `json:"sequenceId"`
	VoterID     peer.ID     `json:"voterId"`
	VoteType    VoteType    `json:"voteType"`
	Signature   []byte      `json:"signature"`
	Timestamp   time.Time   `json:"timestamp"`
}

// RoundKey uniquely identifies a consensus round (ignores block hash).
// It is used to prevent messages from different heights/views/sequences mixing.
type RoundKey struct {
	Height     int
	ViewID     int64
	SequenceID int64
}

// RoundState tracks the state of a single consensus round
type RoundState struct {
	mu           sync.RWMutex
	BlockHash    common.Hash       `json:"blockHash"`
	BlockHeight  int               `json:"blockHeight"`
	State        ConsensusState    `json:"state"`
	ViewID       int64             `json:"viewId"`
	SequenceID   int64             `json:"sequenceId"`
	ValidatorCount int             `json:"validatorCount"`
	Quorum         int             `json:"quorum"`
	validators     map[peer.ID]bool
	PrepareVotes map[peer.ID]*Vote `json:"prepareVotes"`
	CommitVotes  map[peer.ID]*Vote `json:"commitVotes"`
	StartTime    time.Time         `json:"startTime"`
	Deadline     time.Time         `json:"deadline"`
}

// NewRoundState creates a new consensus round state
func NewRoundState(blockHash common.Hash, blockHeight int, viewID, seqID int64, timeout time.Duration, validatorSnapshot []peer.ID) *RoundState {
	validators := make(map[peer.ID]bool, len(validatorSnapshot))
	for _, v := range validatorSnapshot {
		validators[v] = true
	}
	validatorCount := len(validatorSnapshot)

	return &RoundState{
		BlockHash:    blockHash,
		BlockHeight:  blockHeight,
		State:        StatePrePrepare,
		ViewID:       viewID,
		SequenceID:   seqID,
		ValidatorCount: validatorCount,
		Quorum:         quorumForN(validatorCount),
		validators:     validators,
		PrepareVotes: make(map[peer.ID]*Vote),
		CommitVotes:  make(map[peer.ID]*Vote),
		StartTime:    time.Now(),
		Deadline:     time.Now().Add(timeout),
	}
}

// AddPrepareVote adds a prepare vote
func (rs *RoundState) AddPrepareVote(vote *Vote) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.PrepareVotes[vote.VoterID]; exists {
		return false // Already voted
	}

	rs.PrepareVotes[vote.VoterID] = vote
	return true
}

// AddCommitVote adds a commit vote
func (rs *RoundState) AddCommitVote(vote *Vote) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.CommitVotes[vote.VoterID]; exists {
		return false // Already voted
	}

	rs.CommitVotes[vote.VoterID] = vote
	return true
}

// GetPrepareVoteCount returns the number of prepare votes
func (rs *RoundState) GetPrepareVoteCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.PrepareVotes)
}

// GetCommitVoteCount returns the number of commit votes
func (rs *RoundState) GetCommitVoteCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.CommitVotes)
}

// HasPrepareQuorum checks if prepare quorum is reached (based on round snapshot).
func (rs *RoundState) HasPrepareQuorum() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	approveCount := 0
	for _, v := range rs.PrepareVotes {
		if v.VoteType == VoteApprove {
			approveCount++
		}
	}
	return approveCount >= rs.Quorum
}

// HasCommitQuorum checks if commit quorum is reached (based on round snapshot).
func (rs *RoundState) HasCommitQuorum() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	approveCount := 0
	for _, v := range rs.CommitVotes {
		if v.VoteType == VoteApprove {
			approveCount++
		}
	}
	return approveCount >= rs.Quorum
}

// IsValidatorInRound checks membership in the validator snapshot for this round.
func (rs *RoundState) IsValidatorInRound(peerID peer.ID) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.validators[peerID]
}

func quorumForN(n int) int {
	if n == 0 {
		return 0
	}
	f := (n - 1) / 3
	return 2*f + 1
}

// SetState sets the consensus state
func (rs *RoundState) SetState(state ConsensusState) {
	rs.mu.Lock()
	prev := rs.State
	rs.State = state
	height := rs.BlockHeight
	rs.mu.Unlock()
	if prev != state {
		fmt.Printf("[CONSENSUS] status %d -> %d (%s -> %s) height=%d\n",
			prev, state, prev.String(), state.String(), height)
	}
}

// GetState returns the current consensus state
func (rs *RoundState) GetState() ConsensusState {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.State
}

// IsExpired checks if the round has expired
func (rs *RoundState) IsExpired() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return time.Now().After(rs.Deadline)
}

// ValidatorSet manages the set of validators
type ValidatorSet struct {
	mu         sync.RWMutex
	validators map[peer.ID]bool
}

// NewValidatorSet creates a new validator set
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		validators: make(map[peer.ID]bool),
	}
}

// AddValidator adds a validator
func (vs *ValidatorSet) AddValidator(peerID peer.ID) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.validators[peerID] = true
}

// RemoveValidator removes a validator
func (vs *ValidatorSet) RemoveValidator(peerID peer.ID) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.validators, peerID)
}

// IsValidator checks if a peer is a validator
func (vs *ValidatorSet) IsValidator(peerID peer.ID) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.validators[peerID]
}

// GetValidators returns all validators
func (vs *ValidatorSet) GetValidators() []peer.ID {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	validators := make([]peer.ID, 0, len(vs.validators))
	for v := range vs.validators {
		validators = append(validators, v)
	}
	return validators
}

// Size returns the number of validators
func (vs *ValidatorSet) Size() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.validators)
}

// Quorum returns the quorum size (2f + 1 where f = (n-1)/3)
func (vs *ValidatorSet) Quorum() int {
	n := vs.Size()
	if n == 0 {
		return 0
	}
	f := (n - 1) / 3
	return 2*f + 1
}
