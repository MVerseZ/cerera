package consensus

import (
	"testing"
	"time"

	"github.com/cerera/internal/cerera/types"
)

func TestVotingConsensus_Propose(t *testing.T) {
	vc := NewVotingConsensus(2, 30*time.Second)
	proposer := types.HexToAddress("0x1234567890123456789012345678901234567890")

	proposal, err := vc.Propose(ProposalTypeNonce, "100", proposer)
	if err != nil {
		t.Fatalf("Failed to propose: %v", err)
	}

	if proposal == nil {
		t.Fatal("Proposal is nil")
	}

	if proposal.ProposalID == "" {
		t.Fatal("Proposal ID is empty")
	}

	if proposal.Proposer != proposer {
		t.Fatalf("Expected proposer %s, got %s", proposer.Hex(), proposal.Proposer.Hex())
	}
}

func TestVotingConsensus_Vote(t *testing.T) {
	vc := NewVotingConsensus(2, 30*time.Second)
	proposer := types.HexToAddress("0x1234567890123456789012345678901234567890")
	voter1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	voter2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

	// Add voters
	vc.AddVoter(voter1)
	vc.AddVoter(voter2)

	// Propose
	proposal, err := vc.Propose(ProposalTypeNonce, "100", proposer)
	if err != nil {
		t.Fatalf("Failed to propose: %v", err)
	}

	// Vote
	if err := vc.Vote(proposal.ProposalID, voter1, true); err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	if err := vc.Vote(proposal.ProposalID, voter2, true); err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	// Check result
	result, err := vc.GetResult(proposal.ProposalID)
	if err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if !*result {
		t.Fatal("Expected result to be true")
	}
}

func TestVotingConsensus_ProposeNonceChange(t *testing.T) {
	vc := NewVotingConsensus(2, 30*time.Second)
	proposer := types.HexToAddress("0x1234567890123456789012345678901234567890")

	proposalID, err := vc.ProposeNonceChange(100, proposer)
	if err != nil {
		t.Fatalf("Failed to propose nonce change: %v", err)
	}

	if proposalID == "" {
		t.Fatal("Proposal ID is empty")
	}

	proposal, exists := vc.GetProposal(proposalID)
	if !exists {
		t.Fatal("Proposal not found")
	}

	if proposal.ProposalType != ProposalTypeNonce {
		t.Fatalf("Expected proposal type NONCE, got %s", proposal.ProposalType)
	}
}
