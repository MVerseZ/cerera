package chain

import (
	"testing"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
)

func forkTestBlock(height int, prev common.Hash, difficulty uint64, seed string) *block.Block {
	h := common.BytesToHash([]byte(seed))
	return &block.Block{
		Head: &block.Header{
			Height:     height,
			PrevHash:   prev,
			Difficulty: difficulty,
		},
		Hash: h,
	}
}

func forkTestLookup(blocks map[common.Hash]*block.Block) BlockLookup {
	return func(h common.Hash) *block.Block {
		return blocks[h]
	}
}

func TestFindCommonAncestor(t *testing.T) {
	gen := forkTestBlock(0, common.Hash{}, 1, "gen")
	a1 := forkTestBlock(1, gen.Hash, 10, "a1")
	a2 := forkTestBlock(2, a1.Hash, 10, "a2")
	b1 := forkTestBlock(1, gen.Hash, 20, "b1")
	b2 := forkTestBlock(2, b1.Hash, 20, "b2")

	blocks := map[common.Hash]*block.Block{
		gen.Hash: gen,
		a1.Hash:  a1,
		a2.Hash:  a2,
		b1.Hash:  b1,
		b2.Hash:  b2,
	}
	lookup := forkTestLookup(blocks)

	height, hash, err := FindCommonAncestor(a2.Hash, b2.Hash, lookup)
	if err != nil {
		t.Fatalf("FindCommonAncestor: %v", err)
	}
	if height != 0 || hash != gen.Hash {
		t.Fatalf("expected genesis ancestor at height 0, got height=%d hash=%s", height, hash.Hex())
	}

	height, hash, err = FindCommonAncestor(a2.Hash, a2.Hash, lookup)
	if err != nil {
		t.Fatalf("same hash: %v", err)
	}
	if height != 2 || hash != a2.Hash {
		t.Fatalf("expected self at height 2, got height=%d", height)
	}
}

func TestTotalDifficultyOf(t *testing.T) {
	gen := forkTestBlock(0, common.Hash{}, 5, "gen")
	b1 := forkTestBlock(1, gen.Hash, 100, "b1")
	b2 := forkTestBlock(2, b1.Hash, 200, "b2")
	blocks := map[common.Hash]*block.Block{
		gen.Hash: gen,
		b1.Hash:  b1,
		b2.Hash:  b2,
	}
	lookup := forkTestLookup(blocks)

	total := TotalDifficultyOf(b2.Hash, lookup)
	want := int64(5 + 100 + 200)
	if total.Int64() != want {
		t.Fatalf("TotalDifficultyOf = %d, want %d", total.Int64(), want)
	}
}

func TestCompareChainHeads(t *testing.T) {
	gen := forkTestBlock(0, common.Hash{}, 1, "gen")
	low := forkTestBlock(1, gen.Hash, 50, "low")
	high := forkTestBlock(1, gen.Hash, 150, "high")
	blocks := map[common.Hash]*block.Block{
		gen.Hash:  gen,
		low.Hash:  low,
		high.Hash: high,
	}
	lookup := forkTestLookup(blocks)

	if cmp := CompareChainHeads(high, low, lookup); cmp >= 0 {
		t.Fatalf("higher difficulty branch should win, got %d", cmp)
	}
	if cmp := CompareChainHeads(low, high, lookup); cmp <= 0 {
		t.Fatalf("lower difficulty branch should lose, got %d", cmp)
	}
}

func TestOrphanManagerAddAndLink(t *testing.T) {
	gen := forkTestBlock(0, common.Hash{}, 1, "gen")
	orphan := forkTestBlock(2, common.BytesToHash([]byte("missing-parent")), 10, "orphan")
	blocks := map[common.Hash]*block.Block{gen.Hash: gen}
	lookup := forkTestLookup(blocks)
	om := NewOrphanManager(lookup)

	if !om.Add(orphan) {
		t.Fatal("expected orphan add to succeed")
	}
	if om.Count() != 1 {
		t.Fatalf("orphan count = %d, want 1", om.Count())
	}

	parent := forkTestBlock(1, gen.Hash, 10, "parent")
	blocks[parent.Hash] = parent
	linked := om.TryLinkOrphans(parent.Hash)
	if len(linked) != 0 {
		t.Fatalf("orphan parent still unknown to lookup, expected no link, got %d", len(linked))
	}

	blocks[common.BytesToHash([]byte("missing-parent"))] = forkTestBlock(1, gen.Hash, 10, "missing-parent")
	linked = om.TryLinkOrphans(blocks[common.BytesToHash([]byte("missing-parent"))].Hash)
	if len(linked) != 1 {
		t.Fatalf("expected one linked orphan, got %d", len(linked))
	}
	if om.Count() != 0 {
		t.Fatalf("linked orphan should be removed, count=%d", om.Count())
	}
}

func TestBuildChainToHead(t *testing.T) {
	gen := forkTestBlock(0, common.Hash{}, 1, "gen")
	b1 := forkTestBlock(1, gen.Hash, 10, "b1")
	b2 := forkTestBlock(2, b1.Hash, 10, "b2")
	blocks := map[common.Hash]*block.Block{
		gen.Hash: gen,
		b1.Hash:  b1,
		b2.Hash:  b2,
	}
	chain, err := BuildChainToHead(b2, forkTestLookup(blocks))
	if err != nil {
		t.Fatalf("BuildChainToHead: %v", err)
	}
	if len(chain) != 3 || chain[0].Head.Height != 0 || chain[2].Hash != b2.Hash {
		t.Fatalf("unexpected chain order: len=%d heights=%d,%d,%d",
			len(chain), chain[0].Head.Height, chain[1].Head.Height, chain[2].Head.Height)
	}
}

// func TestReorgSwitchesToHeavierBranch(t *testing.T) {
// 	cfg := &config.Config{
// 		Chain:  config.ChainConfig{ChainID: 25331, Path: "EMPTY"},
// 		IN_MEM: true,
// 	}
// 	bc, err := Mold(cfg)
// 	if err != nil {
// 		t.Fatalf("Mold: %v", err)
// 	}

// 	gen := bc.GetLatestBlock()
// 	a1 := forkTestBlock(1, gen.Hash, 100, "a1")
// 	a2 := forkTestBlock(2, a1.Hash, 100, "a2")
// 	branchA := []*block.Block{gen, a1, a2}

// 	if err := bc.Reorg(branchA); err != nil {
// 		t.Fatalf("first reorg: %v", err)
// 	}
// 	if bc.GetLatestBlock().Head.Height != 2 {
// 		t.Fatalf("expected height 2 after branch A, got %d", bc.GetLatestBlock().Head.Height)
// 	}
// 	if bc.GetBlockHashAtHeight(0) != gen.Hash {
// 		t.Fatal("genesis hash mismatch at height 0")
// 	}

// 	b1 := forkTestBlock(1, gen.Hash, 300, "b1")
// 	b2 := forkTestBlock(2, b1.Hash, 300, "b2")
// 	branchB := []*block.Block{gen, b1, b2}

// 	replayed := 0
// 	SetReorgHandler(func(blocks []*block.Block) error {
// 		replayed = len(blocks)
// 		return nil
// 	})
// 	t.Cleanup(func() { SetReorgHandler(nil) })

// 	if err := bc.Reorg(branchB); err != nil {
// 		t.Fatalf("reorg to heavier branch: %v", err)
// 	}
// 	head := bc.GetLatestBlock()
// 	if head.Hash != b2.Hash {
// 		t.Fatalf("head hash = %s, want %s", head.Hash.Hex(), b2.Hash.Hex())
// 	}
// 	if replayed != 3 {
// 		t.Fatalf("ReorgHandler replayed %d blocks, want 3", replayed)
// 	}
// }
