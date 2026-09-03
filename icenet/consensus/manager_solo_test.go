package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
	"github.com/cerera/icenet/peers"
	"github.com/libp2p/go-libp2p"
)

func TestManager_SoloModeFinalizesBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatalf("libp2p.New: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	pm := peers.NewManager(ctx, h, 10)
	scorer := peers.NewScorer(pm)
	mgr := NewManager(ctx, h, pm, scorer, types.Address{}, nil)
	mgr.SetBroadcastFunc(func(int, []byte, []byte) error { return nil })

	finalized := make(chan struct{}, 1)
	mgr.SetOnBlockFinalized(func(b *block.Block) {
		if b == nil || b.Hash != testBlock(1, "solo-block").Hash {
			t.Errorf("unexpected finalized block: %+v", b)
		}
		finalized <- struct{}{}
	})

	mgr.Start()
	defer mgr.Stop()

	if !mgr.isSoloMode() {
		t.Fatal("expected solo mode with zero peers")
	}
	if !mgr.IsValidator(h.ID()) {
		t.Fatal("expected self registered as validator at start")
	}

	b := testBlock(1, "solo-block")
	if err := mgr.ProposeBlock(b); err != nil {
		t.Fatalf("ProposeBlock: %v", err)
	}

	select {
	case <-finalized:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for block finalization in solo mode")
	}
}
