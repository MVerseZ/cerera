package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/message"
	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

func testBlock(height int, hashSeed string) *block.Block {
	h := common.BytesToHash([]byte(hashSeed))
	return &block.Block{
		Head: &block.Header{
			Height: height,
		},
		Hash: h,
	}
}

func TestVotingMessageSignBytes_ExcludesSignature(t *testing.T) {
	h := common.BytesToHash([]byte("h1"))
	msg := &VotingMessage{
		Type:        message.MTPrepare,
		BlockHash:   h,
		BlockHeight: 1,
		ViewID:      7,
		SequenceID:  9,
		VoterID:     peer.ID("v1"),
		VoteType:    VoteApprove,
		Signature:   []byte{1, 2, 3},
		Timestamp:   123,
	}

	b1, err := msg.SignBytes()
	if err != nil {
		t.Fatalf("SignBytes: %v", err)
	}

	msg2 := *msg
	msg2.Signature = nil
	b2, err := msg2.SignBytes()
	if err != nil {
		t.Fatalf("SignBytes (nil sig): %v", err)
	}

	if string(b1) != string(b2) {
		t.Fatalf("signature must not affect SignBytes; got %q vs %q", string(b1), string(b2))
	}
}

func TestConsensusFlow_PrepareCommitQuorum(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	local := peer.ID("local")
	vm := NewVotingManager(ctx, local, types.Address{})

	// 4 validators => quorum = 3 (2f+1 with f=1).
	vals := []peer.ID{"v1", "v2", "v3", "v4", local}
	for _, v := range vals {
		vm.GetValidatorSet().AddValidator(v)
	}

	prepQuorumCh := make(chan struct{}, 1)
	commitQuorumCh := make(chan struct{}, 1)

	vm.SetOnPrepareQuorum(func(_ common.Hash, _ int) {
		prepQuorumCh <- struct{}{}
	})
	vm.SetOnCommitQuorum(func(_ common.Hash, _ int) {
		commitQuorumCh <- struct{}{}
	})

	b := testBlock(1, "block-1")
	preprepare := NewVotingMessage(message.MTPrePrepare, b.Hash, b.Head.Height, 0, 1, peer.ID("v1"), types.Address{}, VoteApprove)
	preprepare.Block = b

	if err := vm.HandlePrePrepare(preprepare, peer.ID("v1")); err != nil {
		t.Fatalf("HandlePrePrepare: %v", err)
	}

	// Prepare votes (3 approvals).
	for _, v := range []peer.ID{"v1", "v2", "v3"} {
		p := NewVotingMessage(message.MTPrepare, b.Hash, b.Head.Height, 0, 1, v, types.Address{}, VoteApprove)
		if err := vm.HandlePrepare(p, v); err != nil {
			t.Fatalf("HandlePrepare(%s): %v", v, err)
		}
	}

	select {
	case <-prepQuorumCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected prepare quorum callback")
	}

	// Commit votes (3 approvals).
	for _, v := range []peer.ID{"v1", "v2", "v3"} {
		c := NewVotingMessage(message.MTCommit, b.Hash, b.Head.Height, 0, 1, v, types.Address{}, VoteApprove)
		if err := vm.HandleCommit(c, v); err != nil {
			t.Fatalf("HandleCommit(%s): %v", v, err)
		}
	}

	select {
	case <-commitQuorumCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected commit quorum callback")
	}

	if vm.GetCurrentRound() != nil {
		t.Fatalf("expected round to be cleared after finalization")
	}
}

func TestRoundTimeout_EmitsCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vm := NewVotingManager(ctx, peer.ID("local"), types.Address{})
	for _, v := range []peer.ID{"v1", "v2", "v3", "v4"} {
		vm.GetValidatorSet().AddValidator(v)
	}

	timeoutCh := make(chan RoundKey, 1)
	vm.SetOnRoundTimeout(func(key RoundKey, _ common.Hash) {
		timeoutCh <- key
	})

	b := testBlock(1, "block-timeout")
	preprepare := NewVotingMessage(message.MTPrePrepare, b.Hash, b.Head.Height, 3, 42, peer.ID("v1"), types.Address{}, VoteApprove)
	preprepare.Block = b
	if err := vm.HandlePrePrepare(preprepare, peer.ID("v1")); err != nil {
		t.Fatalf("HandlePrePrepare: %v", err)
	}

	round := vm.GetCurrentRound()
	if round == nil {
		t.Fatalf("expected current round")
	}

	round.mu.Lock()
	round.Deadline = time.Now().Add(-1 * time.Second)
	round.mu.Unlock()

	vm.cleanup()

	select {
	case key := <-timeoutCh:
		if key.Height != 1 || key.ViewID != 3 || key.SequenceID != 42 {
			t.Fatalf("unexpected timeout key: %+v", key)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected timeout callback")
	}
}

