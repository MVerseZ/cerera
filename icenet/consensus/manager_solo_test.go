package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/cerera/core/events"
	"github.com/cerera/core/types"
	"github.com/cerera/icenet/peers"
	"github.com/cerera/internal/observer"
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

	bus := observer.NewBus()
	finalized := make(chan struct{}, 1)
	bus.Subscribe(observer.SubscriberFunc(func(ev events.Event) error {
		if ev.Kind != events.ChainHeadUpdated || ev.HeadSource != events.HeadFinalized {
			return nil
		}
		b := testBlock(1, "solo-block")
		if ev.Block == nil || ev.Block.Hash != b.Hash {
			t.Errorf("unexpected finalized block: %+v", ev.Block)
		}
		finalized <- struct{}{}
		return nil
	}))
	mgr.SetEventPublisher(bus)

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
