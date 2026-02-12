package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/message"
	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// VoteTimeout is the timeout for a voting round
	VoteTimeout = 30 * time.Second
	// MaxPendingVotes is the maximum number of pending votes to keep
	MaxPendingVotes = 1000
	// CleanupInterval controls how often we check for round expiry.
	CleanupInterval = 5 * time.Second
)

// VotingMessage represents a voting message
type VotingMessage struct {
	Type        message.MType `json:"type"`
	BlockHash   common.Hash   `json:"blockHash"`
	BlockHeight int           `json:"blockHeight"`
	ViewID      int64         `json:"viewId"`
	SequenceID  int64         `json:"sequenceId"`
	VoterID     peer.ID       `json:"voterId"`
	VoteType    VoteType      `json:"voteType"`
	VoterAddr   types.Address `json:"voterAddr"`
	// Block is included only in PrePrepare to ensure availability.
	Block     *block.Block `json:"block,omitempty"`
	Signature []byte       `json:"signature"`
	Timestamp int64        `json:"timestamp"`
}

// NewVotingMessage creates a new voting message
func NewVotingMessage(msgType message.MType, blockHash common.Hash, height int, viewID, seqID int64, voterID peer.ID, voterAddr types.Address, voteType VoteType) *VotingMessage {
	return &VotingMessage{
		Type:        msgType,
		BlockHash:   blockHash,
		BlockHeight: height,
		ViewID:      viewID,
		SequenceID:  seqID,
		VoterID:     voterID,
		VoteType:    voteType,
		VoterAddr:   voterAddr,
		Timestamp:   time.Now().UnixNano(),
	}
}

// Marshal serializes the voting message
func (vm *VotingMessage) Marshal() ([]byte, error) {
	return json.Marshal(vm)
}

// SignBytes returns a canonical payload to sign/verify for this message.
// IMPORTANT: Signature and Block are excluded from the payload.
// Block is only for availability in PrePrepare and should not affect signature.
func (vm *VotingMessage) SignBytes() ([]byte, error) {
	if vm == nil {
		return nil, fmt.Errorf("nil voting message")
	}

	// Create a clone without Signature and Block for signing
	clone := *vm
	clone.Signature = nil
	clone.Block = nil
	return json.Marshal(&clone)
}

// Unmarshal deserializes a voting message
func UnmarshalVotingMessage(data []byte) (*VotingMessage, error) {
	var vm VotingMessage
	if err := json.Unmarshal(data, &vm); err != nil {
		return nil, err
	}
	return &vm, nil
}

// VotingManager handles voting for block consensus
type VotingManager struct {
	mu                  sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
	validatorSet        *ValidatorSet
	currentRound        *RoundState
	pendingPrepareVotes map[RoundKey][]*Vote
	pendingCommitVotes  map[RoundKey][]*Vote
	localPeerID         peer.ID
	localPeerAddr       types.Address

	// Callbacks
	onPrepareQuorum func(common.Hash, int)
	onCommitQuorum  func(common.Hash, int)
	onRoundTimeout  func(RoundKey, common.Hash)
	broadcastVote   func(*VotingMessage) error
}

// NewVotingManager creates a new voting manager
func NewVotingManager(ctx context.Context, localPeerID peer.ID, localPeerAddr types.Address) *VotingManager {
	ctx, cancel := context.WithCancel(ctx)

	return &VotingManager{
		ctx:                 ctx,
		cancel:              cancel,
		validatorSet:        NewValidatorSet(),
		pendingPrepareVotes: make(map[RoundKey][]*Vote),
		pendingCommitVotes:  make(map[RoundKey][]*Vote),
		localPeerID:         localPeerID,
		localPeerAddr:       localPeerAddr,
	}
}

// Start starts the voting manager
func (vm *VotingManager) Start() {
	go vm.cleanupLoop()
	consensusLogger().Infow("Voting manager started")
}

// Stop stops the voting manager
func (vm *VotingManager) Stop() {
	vm.cancel()
	consensusLogger().Infow("Voting manager stopped")
}

// cleanupLoop periodically cleans up old pending votes
func (vm *VotingManager) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-vm.ctx.Done():
			return
		case <-ticker.C:
			vm.cleanup()
		}
	}
}

// cleanup removes expired votes and rounds
func (vm *VotingManager) cleanup() {
	var (
		timedOutKey  *RoundKey
		timedOutHash common.Hash
		onTimeout    func(RoundKey, common.Hash)
	)

	vm.mu.Lock()

	// Check if current round is expired
	if vm.currentRound != nil && vm.currentRound.IsExpired() {
		key := RoundKey{
			Height:     vm.currentRound.BlockHeight,
			ViewID:     vm.currentRound.ViewID,
			SequenceID: vm.currentRound.SequenceID,
		}
		consensusLogger().Warnw("Consensus round expired",
			"blockHash", vm.currentRound.BlockHash,
			"blockHeight", vm.currentRound.BlockHeight,
			"viewID", vm.currentRound.ViewID,
			"seqID", vm.currentRound.SequenceID,
		)
		timedOutKey = &key
		timedOutHash = vm.currentRound.BlockHash
		vm.currentRound = nil
	}

	// Limit pending votes (simple approach: drop arbitrary entries)
	totalPending := len(vm.pendingPrepareVotes) + len(vm.pendingCommitVotes)
	if totalPending > MaxPendingVotes {
		for k := range vm.pendingPrepareVotes {
			delete(vm.pendingPrepareVotes, k)
			totalPending--
			if totalPending <= MaxPendingVotes/2 {
				break
			}
		}
		for k := range vm.pendingCommitVotes {
			if totalPending <= MaxPendingVotes/2 {
				break
			}
			delete(vm.pendingCommitVotes, k)
			totalPending--
		}
	}

	onTimeout = vm.onRoundTimeout
	vm.mu.Unlock()

	if timedOutKey != nil && onTimeout != nil {
		go onTimeout(*timedOutKey, timedOutHash)
	}
}

// StartRound starts a new voting round for a block
func (vm *VotingManager) StartRound(b *block.Block, viewID, seqID int64) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.currentRound != nil && !vm.currentRound.IsExpired() {
		return fmt.Errorf("consensus round already in progress")
	}

	vm.currentRound = NewRoundState(b.Hash, b.Head.Height, viewID, seqID, VoteTimeout, vm.validatorSet.GetValidators())
	vm.applyPendingVotesLocked(RoundKey{Height: b.Head.Height, ViewID: viewID, SequenceID: seqID})

	validatorSnapshot := vm.validatorSet.GetValidators()
	consensusLogger().Infow("[CONSENSUS] Started consensus round",
		"blockHash", b.Hash,
		"blockHeight", b.Head.Height,
		"viewID", viewID,
		"seqID", seqID,
		"validatorCount", len(validatorSnapshot),
		"quorum", vm.currentRound.Quorum,
		"validators", validatorSnapshot,
	)

	// Send pre-prepare message
	msg := NewVotingMessage(
		message.MTPrePrepare,
		b.Hash,
		b.Head.Height,
		viewID,
		seqID,
		vm.localPeerID,
		vm.localPeerAddr,
		VoteApprove,
	)
	msg.Block = b

	if vm.broadcastVote != nil {
		if err := vm.broadcastVote(msg); err != nil {
			consensusLogger().Warnw("Failed to broadcast pre-prepare", "error", err)
		}
	}

	return nil
}

// HandlePrePrepare handles a pre-prepare message
func (vm *VotingManager) HandlePrePrepare(msg *VotingMessage, from peer.ID) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate sender is a validator
	if !vm.validatorSet.IsValidator(from) {
		return fmt.Errorf("sender is not a validator")
	}

	// Ensure block availability: PrePrepare must include the block.
	if msg.Block == nil {
		return fmt.Errorf("pre-prepare missing block payload")
	}

	// Basic consistency checks.
	if msg.Block.Hash != msg.BlockHash || msg.Block.Head == nil || msg.Block.Head.Height != msg.BlockHeight {
		return fmt.Errorf("pre-prepare block mismatch")
	}

	key := RoundKey{Height: msg.BlockHeight, ViewID: msg.ViewID, SequenceID: msg.SequenceID}

	// Create round if none / expired
	if vm.currentRound == nil || vm.currentRound.IsExpired() {
		vm.currentRound = NewRoundState(msg.BlockHash, msg.BlockHeight, msg.ViewID, msg.SequenceID, VoteTimeout, vm.validatorSet.GetValidators())
		vm.applyPendingVotesLocked(key)
	} else {
		// Ignore pre-prepare for a different active round (prevents cross-round mixing).
		if vm.currentRound.BlockHeight != msg.BlockHeight ||
			vm.currentRound.ViewID != msg.ViewID ||
			vm.currentRound.SequenceID != msg.SequenceID {
			consensusLogger().Debugw("[CONSENSUS] Ignoring pre-prepare for non-current round",
				"from", from,
				"msgHeight", msg.BlockHeight,
				"msgViewID", msg.ViewID,
				"msgSeqID", msg.SequenceID,
				"curHeight", vm.currentRound.BlockHeight,
				"curViewID", vm.currentRound.ViewID,
				"curSeqID", vm.currentRound.SequenceID,
			)
			return nil
		}
		// Conflicting block hash for same (height, view, seq) => ignore (equivocation).
		if vm.currentRound.BlockHash != msg.BlockHash {
			consensusLogger().Warnw("[CONSENSUS] Conflicting pre-prepare for same round (equivocation)",
				"from", from,
				"height", msg.BlockHeight,
				"viewID", msg.ViewID,
				"seqID", msg.SequenceID,
				"currentHash", vm.currentRound.BlockHash,
				"msgHash", msg.BlockHash,
			)
			return nil
		}
	}

	// Move to prepare phase
	vm.currentRound.SetState(StatePrepare)

	// Send prepare vote
	prepareMsg := NewVotingMessage(
		message.MTPrepare,
		msg.BlockHash,
		msg.BlockHeight,
		msg.ViewID,
		msg.SequenceID,
		vm.localPeerID,
		vm.localPeerAddr,
		VoteApprove,
	)

	if vm.broadcastVote != nil {
		if err := vm.broadcastVote(prepareMsg); err != nil {
			consensusLogger().Warnw("Failed to broadcast prepare", "error", err)
		}
	}

	return nil
}

// HandlePrepare handles a prepare message
func (vm *VotingManager) HandlePrepare(msg *VotingMessage, from peer.ID) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate sender is a validator
	if !vm.validatorSet.IsValidator(from) {
		return fmt.Errorf("sender is not a validator")
	}

	key := RoundKey{Height: msg.BlockHeight, ViewID: msg.ViewID, SequenceID: msg.SequenceID}

	// Check if we have a current round for this (height, view, seq)
	if vm.currentRound == nil ||
		vm.currentRound.BlockHeight != msg.BlockHeight ||
		vm.currentRound.ViewID != msg.ViewID ||
		vm.currentRound.SequenceID != msg.SequenceID {
		// Store as pending prepare vote
		vm.addPendingPrepareVote(key, &Vote{
			BlockHash:   msg.BlockHash,
			BlockHeight: msg.BlockHeight,
			ViewID:      msg.ViewID,
			SequenceID:  msg.SequenceID,
			VoterID:     from,
			VoteType:    msg.VoteType,
			Signature:   msg.Signature,
			Timestamp:   time.Now(),
		})
		return nil
	}
	if vm.currentRound.BlockHash != msg.BlockHash {
		consensusLogger().Warnw("[CONSENSUS] Ignoring prepare for conflicting hash in current round",
			"from", from,
			"height", msg.BlockHeight,
			"viewID", msg.ViewID,
			"seqID", msg.SequenceID,
			"currentHash", vm.currentRound.BlockHash,
			"msgHash", msg.BlockHash,
		)
		return nil
	}
	// Enforce validator snapshot for the round (quorum stability).
	if !vm.currentRound.IsValidatorInRound(from) {
		return nil
	}

	// Add prepare vote
	vote := &Vote{
		BlockHash:   msg.BlockHash,
		BlockHeight: msg.BlockHeight,
		ViewID:      msg.ViewID,
		SequenceID:  msg.SequenceID,
		VoterID:     from,
		VoteType:    msg.VoteType,
		Signature:   msg.Signature,
		Timestamp:   time.Now(),
	}

	vm.currentRound.AddPrepareVote(vote)

	// Check for quorum
	quorum := vm.currentRound.Quorum
	prepareCount := vm.currentRound.GetPrepareVoteCount()
	consensusLogger().Debugw("[CONSENSUS] Prepare vote received",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"prepareVotes", prepareCount,
		"quorum", quorum,
		"state", vm.currentRound.GetState(),
	)

	if vm.currentRound.HasPrepareQuorum() && vm.currentRound.GetState() == StatePrepare {
		// Move to commit phase
		vm.currentRound.SetState(StateCommit)

		consensusLogger().Infow("[CONSENSUS] Prepare quorum reached - moving to commit phase",
			"blockHash", msg.BlockHash,
			"height", msg.BlockHeight,
			"prepareVotes", prepareCount,
			"quorum", quorum,
		)

		// Notify callback
		if vm.onPrepareQuorum != nil {
			go vm.onPrepareQuorum(msg.BlockHash, msg.BlockHeight)
		}

		// Send commit vote
		commitMsg := NewVotingMessage(
			message.MTCommit,
			msg.BlockHash,
			msg.BlockHeight,
			msg.ViewID,
			msg.SequenceID,
			vm.localPeerID,
			vm.localPeerAddr,
			VoteApprove,
		)

		if vm.broadcastVote != nil {
			if err := vm.broadcastVote(commitMsg); err != nil {
				consensusLogger().Warnw("Failed to broadcast commit", "error", err)
			}
		}
	}

	return nil
}

// HandleCommit handles a commit message
func (vm *VotingManager) HandleCommit(msg *VotingMessage, from peer.ID) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate sender is a validator
	if !vm.validatorSet.IsValidator(from) {
		return fmt.Errorf("sender is not a validator")
	}

	key := RoundKey{Height: msg.BlockHeight, ViewID: msg.ViewID, SequenceID: msg.SequenceID}

	// Check if we have a current round for this (height, view, seq)
	if vm.currentRound == nil ||
		vm.currentRound.BlockHeight != msg.BlockHeight ||
		vm.currentRound.ViewID != msg.ViewID ||
		vm.currentRound.SequenceID != msg.SequenceID {
		// Store as pending commit vote (can arrive before we opened the round)
		vm.addPendingCommitVote(key, &Vote{
			BlockHash:   msg.BlockHash,
			BlockHeight: msg.BlockHeight,
			ViewID:      msg.ViewID,
			SequenceID:  msg.SequenceID,
			VoterID:     from,
			VoteType:    msg.VoteType,
			Signature:   msg.Signature,
			Timestamp:   time.Now(),
		})
		return nil
	}
	if vm.currentRound.BlockHash != msg.BlockHash {
		consensusLogger().Warnw("[CONSENSUS] Ignoring commit for conflicting hash in current round",
			"from", from,
			"height", msg.BlockHeight,
			"viewID", msg.ViewID,
			"seqID", msg.SequenceID,
			"currentHash", vm.currentRound.BlockHash,
			"msgHash", msg.BlockHash,
		)
		return nil
	}
	// Enforce validator snapshot for the round (quorum stability).
	if !vm.currentRound.IsValidatorInRound(from) {
		return nil
	}

	// Add commit vote
	vote := &Vote{
		BlockHash:   msg.BlockHash,
		BlockHeight: msg.BlockHeight,
		ViewID:      msg.ViewID,
		SequenceID:  msg.SequenceID,
		VoterID:     from,
		VoteType:    msg.VoteType,
		Signature:   msg.Signature,
		Timestamp:   time.Now(),
	}

	vm.currentRound.AddCommitVote(vote)

	// Check for quorum
	quorum := vm.currentRound.Quorum
	commitCount := vm.currentRound.GetCommitVoteCount()
	consensusLogger().Debugw("[CONSENSUS] Commit vote received",
		"from", from,
		"blockHash", msg.BlockHash,
		"height", msg.BlockHeight,
		"commitVotes", commitCount,
		"quorum", quorum,
		"state", vm.currentRound.GetState(),
	)

	if vm.currentRound.HasCommitQuorum() && vm.currentRound.GetState() == StateCommit {
		// Finalize
		vm.currentRound.SetState(StateFinalized)

		consensusLogger().Infow("[CONSENSUS] Commit quorum reached - block finalized",
			"blockHash", msg.BlockHash,
			"height", msg.BlockHeight,
			"commitVotes", commitCount,
			"quorum", quorum,
		)

		// Notify callback
		if vm.onCommitQuorum != nil {
			go vm.onCommitQuorum(msg.BlockHash, msg.BlockHeight)
		}

		// Clear current round
		vm.currentRound = nil
	}

	return nil
}

func (vm *VotingManager) addPendingPrepareVote(key RoundKey, vote *Vote) {
	vm.pendingPrepareVotes[key] = append(vm.pendingPrepareVotes[key], vote)
}

func (vm *VotingManager) addPendingCommitVote(key RoundKey, vote *Vote) {
	vm.pendingCommitVotes[key] = append(vm.pendingCommitVotes[key], vote)
}

// applyPendingVotesLocked applies pending votes to the current round.
// Caller must hold vm.mu (write).
func (vm *VotingManager) applyPendingVotesLocked(key RoundKey) {
	if vm.currentRound == nil ||
		vm.currentRound.BlockHeight != key.Height ||
		vm.currentRound.ViewID != key.ViewID ||
		vm.currentRound.SequenceID != key.SequenceID {
		return
	}

	if votes, ok := vm.pendingPrepareVotes[key]; ok {
		for _, v := range votes {
			// Only apply if hash matches the round.
			if v.BlockHash == vm.currentRound.BlockHash {
				vm.currentRound.AddPrepareVote(v)
			}
		}
		delete(vm.pendingPrepareVotes, key)
	}

	if votes, ok := vm.pendingCommitVotes[key]; ok {
		for _, v := range votes {
			if v.BlockHash == vm.currentRound.BlockHash {
				vm.currentRound.AddCommitVote(v)
			}
		}
		delete(vm.pendingCommitVotes, key)
	}
}

// GetCurrentRound returns the current round state
func (vm *VotingManager) GetCurrentRound() *RoundState {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.currentRound
}

// SetOnPrepareQuorum sets the callback for prepare quorum
func (vm *VotingManager) SetOnPrepareQuorum(callback func(common.Hash, int)) {
	vm.onPrepareQuorum = callback
}

// SetOnCommitQuorum sets the callback for commit quorum
func (vm *VotingManager) SetOnCommitQuorum(callback func(common.Hash, int)) {
	vm.onCommitQuorum = callback
}

// SetOnRoundTimeout sets the callback for round timeout events.
func (vm *VotingManager) SetOnRoundTimeout(callback func(RoundKey, common.Hash)) {
	vm.onRoundTimeout = callback
}

// SetBroadcastVote sets the broadcast function
func (vm *VotingManager) SetBroadcastVote(broadcast func(*VotingMessage) error) {
	vm.broadcastVote = broadcast
}

// GetValidatorSet returns the validator set
func (vm *VotingManager) GetValidatorSet() *ValidatorSet {
	return vm.validatorSet
}
