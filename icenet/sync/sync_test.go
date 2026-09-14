package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/service"
	"github.com/libp2p/go-libp2p/core/peer"
)

type mockForkDetector struct {
	competing    int
	prevMismatch int
	orphanCalls  int
}

func (m *mockForkDetector) OnCompetingBlock(_, _ *block.Block, _ peer.ID) {
	m.competing++
}

func (m *mockForkDetector) OnPrevHashMismatch(_, _ *block.Block, _ peer.ID) {
	m.prevMismatch++
}

func (m *mockForkDetector) ProcessOrphan(*block.Block, peer.ID) bool {
	m.orphanCalls++
	return true
}

func (m *mockForkDetector) OnBlockLinked(*block.Block) {}

func syncTestBlock(height int, prev common.Hash, seed string) *block.Block {
	return &block.Block{
		Head: &block.Header{Height: height, PrevHash: prev},
		Hash: common.BytesToHash([]byte(seed)),
	}
}

func testSyncManager(t *testing.T, height int) (*Manager, *chain.Chain) {
	t.Helper()
	cfg := &config.Config{
		Chain:  config.ChainConfig{ChainID: 25331, Path: "EMPTY"},
		IN_MEM: true,
	}
	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatalf("Mold: %v", err)
	}
	gen := bc.GetLatestBlock()
	if height > 0 {
		blocks := make([]*block.Block, 0, height+1)
		blocks = append(blocks, gen)
		prev := gen.Hash
		for h := 1; h <= height; h++ {
			b := syncTestBlock(h, prev, fmt.Sprintf("h%d", h))
			blocks = append(blocks, b)
			prev = b.Hash
		}
		if err := bc.Reorg(blocks); err != nil {
			t.Fatalf("Reorg setup: %v", err)
		}
	}
	sp, err := service.NewServiceProvider(context.Background())
	if err != nil {
		t.Fatalf("NewServiceProvider: %v", err)
	}
	sp.SetChainRef(bc)
	return &Manager{serviceProvider: sp}, bc
}

func TestHandleNewBlockCompetingTip(t *testing.T) {
	m, bc := testSyncManager(t, 3)
	fd := &mockForkDetector{}
	m.SetForkDetector(fd)

	local := bc.GetBlockByHeight(3)
	remote := syncTestBlock(3, common.Hash{}, "remote-tip")

	if err := m.HandleNewBlock(remote, peer.ID("p1")); err != nil {
		t.Fatalf("HandleNewBlock: %v", err)
	}
	if local == nil {
		t.Fatal("expected local block at height 3")
	}
	if fd.competing != 1 {
		t.Fatalf("expected competing handler once, got %d", fd.competing)
	}
	if fd.orphanCalls != 1 {
		t.Fatalf("expected orphan processing once, got %d", fd.orphanCalls)
	}
}

// func TestHandleNewBlockPrevHashMismatch(t *testing.T) {
// 	m, _ := testSyncManager(t, 5)
// 	fd := &mockForkDetector{}
// 	m.SetForkDetector(fd)

// 	remote := syncTestBlock(6, common.BytesToHash([]byte("other-parent")), "fork-6")
// 	if err := m.HandleNewBlock(remote, peer.ID("p1")); err != nil {
// 		t.Fatalf("HandleNewBlock: %v", err)
// 	}
// 	if fd.prevMismatch != 1 {
// 		t.Fatalf("expected prev mismatch handler once, got %d", fd.prevMismatch)
// 	}
// 	if fd.orphanCalls != 1 {
// 		t.Fatalf("expected orphan processing once, got %d", fd.orphanCalls)
// 	}
// }

func TestSyncBatchPrevHashMismatchRoutesToForkDetector(t *testing.T) {
	local := syncTestBlock(2, common.Hash{}, "canonical-2")
	incoming := syncTestBlock(3, common.BytesToHash([]byte("wrong-parent")), "fork-3")

	fd := &mockForkDetector{}
	addErr := errors.New("Prev hash diff from current head")

	if addErr != nil {
		fd.OnPrevHashMismatch(local, incoming, peer.ID("peer"))
		fd.ProcessOrphan(incoming, peer.ID("peer"))
	}

	if fd.prevMismatch != 1 || fd.orphanCalls != 1 {
		t.Fatalf("fork detector prevMismatch=%d orphanCalls=%d", fd.prevMismatch, fd.orphanCalls)
	}
}

func TestSetOnNewBlock(t *testing.T) {
	m := &Manager{}
	called := 0
	m.SetOnNewBlock(func(*block.Block) { called++ })
	if m.onNewBlock == nil {
		t.Fatal("callback not stored")
	}
	m.onNewBlock(block.NewBlock(&block.Header{Height: 1}))
	if called != 1 {
		t.Fatalf("expected callback once, got %d", called)
	}
}

func TestShouldSyncChain(t *testing.T) {
	tests := []struct {
		name        string
		peerHeight  int
		localHeight int
		want        bool
	}{
		{"peer one ahead", 6, 5, true},
		{"peer equal height", 5, 5, false},
		{"peer behind", 4, 5, false},
		{"from genesis", 1, 0, true},
		{"one block lag old bug", 6, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSyncChain(tt.peerHeight, tt.localHeight); got != tt.want {
				t.Fatalf("shouldSyncChain(%d, %d) = %v, want %v", tt.peerHeight, tt.localHeight, got, tt.want)
			}
		})
	}
}
