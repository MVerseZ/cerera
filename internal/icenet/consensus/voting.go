package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/protocol"
)

// ProposalType represents the type of proposal
type ProposalType string

const (
	ProposalTypeNonce     ProposalType = "NONCE"
	ProposalTypeNodeAdd   ProposalType = "NODE_ADD"
	ProposalTypeNodeRemove ProposalType = "NODE_REMOVE"
)

// Proposal represents a voting proposal
type Proposal struct {
	ProposalID   string
	ProposalType ProposalType
	Data         string
	Proposer     types.Address
	Timestamp    int64
	Votes        map[types.Address]bool // voter -> vote (true = approve, false = reject)
	Result       *bool                  // nil = pending, true = approved, false = rejected
	ResultTime   *time.Time
}

// VotingConsensus manages voting-based consensus
type VotingConsensus struct {
	mu          sync.RWMutex
	proposals   map[string]*Proposal
	voters      map[types.Address]bool // known voters
	minVotes    int                    // minimum votes required for consensus
	timeout     time.Duration          // timeout for proposals
	meshNetwork interface{}            // mesh network for broadcasting (will be set via SetMeshNetwork)
	selfAddress types.Address          // address of the current node
}

// NewVotingConsensus creates a new voting consensus manager
func NewVotingConsensus(minVotes int, timeout time.Duration) *VotingConsensus {
	return &VotingConsensus{
		proposals: make(map[string]*Proposal),
		voters:    make(map[types.Address]bool),
		minVotes:  minVotes,
		timeout:   timeout,
	}
}

// SetMeshNetwork sets the mesh network for broadcasting
func (vc *VotingConsensus) SetMeshNetwork(meshNetwork interface{}) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.meshNetwork = meshNetwork
}

// AddVoter adds a voter to the consensus
func (vc *VotingConsensus) AddVoter(addr types.Address) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.voters[addr] = true
}

// RemoveVoter removes a voter from the consensus
func (vc *VotingConsensus) RemoveVoter(addr types.Address) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	delete(vc.voters, addr)
}

// GetVoters returns all known voters
func (vc *VotingConsensus) GetVoters() []types.Address {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	voters := make([]types.Address, 0, len(vc.voters))
	for addr := range vc.voters {
		voters = append(voters, addr)
	}
	return voters
}

// generateProposalID generates a unique proposal ID
func generateProposalID(proposalType ProposalType, data string, proposer types.Address, timestamp int64) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", proposalType, data, proposer.Hex(), timestamp)))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes as ID
}

// Propose creates a new proposal
func (vc *VotingConsensus) Propose(proposalType ProposalType, data string, proposer types.Address) (*Proposal, error) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	timestamp := time.Now().Unix()
	proposalID := generateProposalID(proposalType, data, proposer, timestamp)

	// Check if proposal already exists
	if _, exists := vc.proposals[proposalID]; exists {
		return nil, fmt.Errorf("proposal already exists: %s", proposalID)
	}

	proposal := &Proposal{
		ProposalID:   proposalID,
		ProposalType: proposalType,
		Data:         data,
		Proposer:     proposer,
		Timestamp:    timestamp,
		Votes:        make(map[types.Address]bool),
	}

	vc.proposals[proposalID] = proposal

	// Start timeout goroutine
	go vc.handleProposalTimeout(proposalID)

	return proposal, nil
}

// Vote votes on a proposal
func (vc *VotingConsensus) Vote(proposalID string, voter types.Address, vote bool) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	proposal, exists := vc.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal not found: %s", proposalID)
	}

	// Check if proposal already has a result
	if proposal.Result != nil {
		return fmt.Errorf("proposal already has a result: %s", proposalID)
	}

	// Check if voter has already voted
	if _, hasVoted := proposal.Votes[voter]; hasVoted {
		return fmt.Errorf("voter already voted: %s", voter.Hex())
	}

	// Check if voter is valid
	if !vc.voters[voter] {
		return fmt.Errorf("voter not in consensus: %s", voter.Hex())
	}

	// Record vote
	proposal.Votes[voter] = vote

	// Check if we have enough votes
	vc.checkProposalResult(proposalID)

	return nil
}

// checkProposalResult checks if a proposal has reached consensus
func (vc *VotingConsensus) checkProposalResult(proposalID string) {
	proposal, exists := vc.proposals[proposalID]
	if !exists {
		return
	}

	// Count votes
	votesFor := 0
	votesAgainst := 0
	for _, vote := range proposal.Votes {
		if vote {
			votesFor++
		} else {
			votesAgainst++
		}
	}

	// Check if we have minimum votes
	if len(proposal.Votes) < vc.minVotes {
		return // Not enough votes yet
	}

	// Determine result (simple majority)
	result := votesFor > votesAgainst
	now := time.Now()
	proposal.Result = &result
	proposal.ResultTime = &now

	// Broadcast result if mesh network is available
	if vc.meshNetwork != nil {
		vc.broadcastResult(proposalID, result, votesFor, votesAgainst)
	}
}

// broadcastResult broadcasts the consensus result
func (vc *VotingConsensus) broadcastResult(proposalID string, result bool, votesFor, votesAgainst int) {
	vc.mu.RLock()
	meshNet := vc.meshNetwork
	vc.mu.RUnlock()
	
	// Use type assertion to call BroadcastMessage
	if meshNet != nil {
		if net, ok := meshNet.(interface {
			BroadcastMessage(msg protocol.Message) error
		}); ok {
			resultMsg := &protocol.ConsensusResultMessage{
				ProposalID:    proposalID,
				Result:        result,
				VotesFor:      votesFor,
				VotesAgainst:  votesAgainst,
				Timestamp:     time.Now().Unix(),
			}
			_ = net.BroadcastMessage(resultMsg) // Ignore error for now
		}
	}
}

// GetProposal returns a proposal by ID
func (vc *VotingConsensus) GetProposal(proposalID string) (*Proposal, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	proposal, exists := vc.proposals[proposalID]
	return proposal, exists
}

// GetResult returns the result of a proposal
func (vc *VotingConsensus) GetResult(proposalID string) (*bool, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	proposal, exists := vc.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal not found: %s", proposalID)
	}

	return proposal.Result, nil
}

// handleProposalTimeout handles proposal timeout
func (vc *VotingConsensus) handleProposalTimeout(proposalID string) {
	time.Sleep(vc.timeout)

	vc.mu.Lock()
	defer vc.mu.Unlock()

	proposal, exists := vc.proposals[proposalID]
	if !exists {
		return
	}

	// If no result yet, reject the proposal
	if proposal.Result == nil {
		result := false
		now := time.Now()
		proposal.Result = &result
		proposal.ResultTime = &now
	}
}

// ProcessProposalMessage processes a proposal message from the network
func (vc *VotingConsensus) ProcessProposalMessage(msg *protocol.ProposalMessage) error {
	proposalType := ProposalType(msg.ProposalType)
	proposal, err := vc.Propose(proposalType, msg.Data, msg.Proposer)
	if err != nil {
		return err
	}

	// Auto-vote on our own proposals (if self address is set)
	vc.mu.RLock()
	selfAddr := vc.selfAddress
	vc.mu.RUnlock()
	if selfAddr != (types.Address{}) && proposal.Proposer == selfAddr {
		return vc.Vote(proposal.ProposalID, proposal.Proposer, true)
	}

	return nil
}

// ProcessVoteMessage processes a vote message from the network
func (vc *VotingConsensus) ProcessVoteMessage(msg *protocol.VoteMessage) error {
	return vc.Vote(msg.ProposalID, msg.Voter, msg.Vote)
}

// ProcessConsensusResultMessage processes a consensus result message from the network
func (vc *VotingConsensus) ProcessConsensusResultMessage(msg *protocol.ConsensusResultMessage) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	proposal, exists := vc.proposals[msg.ProposalID]
	if !exists {
		// Create proposal from result (it was proposed elsewhere)
		proposal = &Proposal{
			ProposalID: msg.ProposalID,
			Votes:      make(map[types.Address]bool),
		}
		vc.proposals[msg.ProposalID] = proposal
	}

	// Update result if not set
	if proposal.Result == nil {
		proposal.Result = &msg.Result
		now := time.Now()
		proposal.ResultTime = &now
	}

	return nil
}

// ProposeNonceChange proposes a nonce change
func (vc *VotingConsensus) ProposeNonceChange(newNonce uint64, proposer types.Address) (string, error) {
	data := fmt.Sprintf("%d", newNonce)
	proposal, err := vc.Propose(ProposalTypeNonce, data, proposer)
	if err != nil {
		return "", err
	}
	return proposal.ProposalID, nil
}
