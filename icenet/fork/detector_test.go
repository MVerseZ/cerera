package fork

import (
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/service"
	"github.com/libp2p/go-libp2p/core/peer"
)

func detBlock(height int, prev common.Hash, difficulty uint64, seed string) *block.Block {
	return &block.Block{
		Head: &block.Header{
			Height:     height,
			PrevHash:   prev,
			Difficulty: difficulty,
		},
		Hash: common.BytesToHash([]byte(seed)),
	}
}

func TestOnCompetingBlockStoresOrphan(t *testing.T) {
	cfg := &config.Config{
		Chain:  config.ChainConfig{ChainID: 25331, Path: "EMPTY"},
		IN_MEM: true,
	}
	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatalf("Mold: %v", err)
	}

	sp, err := service.NewServiceProvider(bc, nil, nil)
	if err != nil {
		t.Fatalf("NewServiceProvider: %v", err)
	}
	d := NewDetector(bc, sp)

	local := bc.GetLatestBlock()
	remote := detBlock(0, common.Hash{}, 500, "competing-genesis-alt")
	d.OnCompetingBlock(local, remote, peer.ID("peer-1"))

	if got := d.BestCandidate(); got == nil || got.Hash != remote.Hash {
		t.Fatal("expected competing block stored as candidate")
	}
	d.OnCompetingBlock(local, nil, peer.ID("peer-1"))
	d.OnPrevHashMismatch(local, nil, peer.ID("peer-1"))
}

func TestProcessOrphanWithoutService(t *testing.T) {
	cfg := &config.Config{
		Chain:  config.ChainConfig{ChainID: 25331, Path: "EMPTY"},
		IN_MEM: true,
	}
	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatalf("Mold: %v", err)
	}
	d := NewDetector(bc, nil)
	if d.Orphans() == nil {
		t.Fatal("missing orphan manager")
	}
	if d.ProcessOrphan(nil, peer.ID("p")) {
		t.Fatal("nil block")
	}
	genesis := bc.GetLatestBlock()
	orphan := detBlock(1, genesis.Hash, 1, "orphan-1")
	if !d.ProcessOrphan(orphan, peer.ID("p")) {
		t.Fatal("expected orphan to be stored")
	}
	if d.Orphans().Count() != 1 {
		t.Fatalf("orphans %d", d.Orphans().Count())
	}
	d.SetOnReorg(func(*block.Block) {})
	d.OnBlockLinked(genesis)
	d.OnBlockLinked(nil)
	if d.lookup(orphan.Hash) == nil && d.lookup(genesis.Hash) == nil {
		t.Fatal("lookup missed chain and orphan")
	}
	bare := NewDetector(nil, nil)
	if bare.lookup(common.Hash{}) != nil {
		t.Fatal("empty detector lookup")
	}
}

// func TestHeavierBranchTriggersReorg(t *testing.T) {
// 	cfg := &config.Config{
// 		Chain:  config.ChainConfig{ChainID: 25331, Path: "EMPTY"},
// 		IN_MEM: true,
// 	}
// 	bc, err := chain.Mold(cfg)
// 	if err != nil {
// 		t.Fatalf("Mold: %v", err)
// 	}
// 	sp, err := service.NewServiceProvider(context.Background())
// 	if err != nil {
// 		t.Fatalf("NewServiceProvider: %v", err)
// 	}
// 	sp.SetChainRef(bc)

// 	reorged := false
// 	d := NewDetector(bc, sp)
// 	d.SetOnReorg(func(*block.Block) { reorged = true })

// 	gen := bc.GetLatestBlock()
// 	light := []*block.Block{
// 		gen,
// 		detBlock(1, gen.Hash, 50, "light-1"),
// 	}
// 	if err := bc.Reorg(light); err != nil {
// 		t.Fatalf("setup branch: %v", err)
// 	}

// 	heavyMid := detBlock(1, gen.Hash, 500, "heavy-1")
// 	heavyHead := detBlock(2, heavyMid.Hash, 500, "heavy-2")

// 	d.Orphans().Add(heavyMid)
// 	d.Orphans().Add(heavyHead)

// 	d.tryReorgFromHead(heavyHead)

// 	head := bc.GetLatestBlock()
// 	if head.Hash != heavyHead.Hash {
// 		t.Fatalf("expected heavier head %s, got %s", heavyHead.Hash.Hex(), head.Hash.Hex())
// 	}
// 	if !reorged {
// 		t.Fatal("expected onReorg callback after successful reorg")
// 	}
// }
