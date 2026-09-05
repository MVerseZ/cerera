package fork

import (
	"sync"

	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/metrics"
	"github.com/cerera/icenet/service"
	"github.com/cerera/internal/logger"
	"github.com/libp2p/go-libp2p/core/peer"
)

var flogger = logger.Named("fork")

// Detector tracks competing branches and triggers reorg when a heavier chain is available.
type Detector struct {
	mu sync.Mutex

	chain   *chain.Chain
	orphans *chain.OrphanManager
	sp      *service.ServiceProvider

	onReorg func(*block.Block)

	pendingHead *block.Block
}

// NewDetector creates a fork detector wired to chain and orphan storage.
func NewDetector(bc *chain.Chain, sp *service.ServiceProvider) *Detector {
	d := &Detector{
		chain: bc,
		sp:    sp,
	}
	d.orphans = chain.NewOrphanManager(d.lookup)
	return d
}

func (d *Detector) lookup(h common.Hash) *block.Block {
	if d.chain != nil {
		if b := d.chain.GetBlock(h); b != nil {
			return b
		}
	}
	if d.orphans != nil {
		return d.orphans.Get(h)
	}
	return nil
}

// SetOnReorg registers a callback after a successful reorg (e.g. miner restart).
func (d *Detector) SetOnReorg(fn func(*block.Block)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onReorg = fn
}

// Orphans returns the orphan manager for tests and metrics.
func (d *Detector) Orphans() *chain.OrphanManager {
	return d.orphans
}

// OnCompetingBlock is called when two different hashes exist at the same height.
func (d *Detector) OnCompetingBlock(local, remote *block.Block, from peer.ID) {
	if remote == nil {
		return
	}
	metrics.RecordForkDetected("competing_tip")
	flogger.Warnw("competing block at tip",
		"height", remote.Head.Height,
		"local", localHash(local),
		"remote", remote.Hash,
		"peer", from,
	)
	d.storeCandidate(remote, from)
}

// OnPrevHashMismatch is called when the next block does not extend local head.
func (d *Detector) OnPrevHashMismatch(localHead, incoming *block.Block, from peer.ID) {
	if incoming == nil {
		return
	}
	metrics.RecordForkDetected("prev_hash_mismatch")
	flogger.Warnw("prev hash mismatch — fork candidate",
		"height", incoming.Head.Height,
		"incoming", incoming.Hash,
		"peer", from,
	)
	d.storeCandidate(incoming, from)
}

// ProcessOrphan validates and stores a block whose parent is not canonical.
func (d *Detector) ProcessOrphan(b *block.Block, from peer.ID) bool {
	if b == nil || b.Head == nil || d.orphans == nil {
		return false
	}
	parent := d.orphans.Lookup(b.Head.PrevHash)
	if parent == nil && b.Head.Height > 0 {
		if d.sp != nil {
			parent = d.sp.GetBlockByHash(b.Head.PrevHash)
		}
	}
	if d.sp != nil {
		if err := d.sp.ValidateOrphanBlock(b, parent); err != nil {
			flogger.Debugw("orphan rejected", "hash", b.Hash, "err", err)
			return false
		}
	}
	if !d.orphans.Add(b) {
		return false
	}
	metrics.SetOrphanBlocks(d.orphans.Count())
	flogger.Infow("orphan stored", "height", b.Head.Height, "hash", b.Hash, "peer", from)
	d.tryLinkAndReorg(b)
	return true
}

// OnBlockLinked notifies the detector that a new canonical block was added.
func (d *Detector) OnBlockLinked(b *block.Block) {
	if b == nil || d.orphans == nil {
		return
	}
	linked := d.orphans.TryLinkOrphans(b.Hash)
	metrics.SetOrphanBlocks(d.orphans.Count())
	for _, orphan := range linked {
		d.tryReorgFromHead(orphan)
	}
}

func (d *Detector) storeCandidate(b *block.Block, from peer.ID) {
	d.mu.Lock()
	d.pendingHead = b
	d.mu.Unlock()
	d.ProcessOrphan(b, from)
}

func (d *Detector) tryLinkAndReorg(b *block.Block) {
	if b == nil {
		return
	}
	if d.sp != nil {
		if parent := d.orphans.Lookup(b.Head.PrevHash); parent != nil || b.Head.Height == 0 {
			d.tryReorgFromHead(b)
		}
	}
}

func (d *Detector) tryReorgFromHead(head *block.Block) {
	if d.chain == nil || head == nil {
		return
	}
	branch, err := chain.BuildChainToHead(head, d.orphans.Lookup)
	if err != nil {
		flogger.Debugw("branch build failed", "err", err)
		return
	}
	localHead := d.chain.GetLatestBlock()
	if chain.CompareChainHeads(head, localHead, d.orphans.Lookup) >= 0 {
		return
	}
	flogger.Infow("heavier branch detected — reorg",
		"newHead", head.Hash,
		"height", head.Head.Height,
	)
	if d.sp != nil {
		if err := d.sp.Reorg(branch); err != nil {
			flogger.Errorw("reorg failed", "err", err)
			return
		}
	} else if err := d.chain.Reorg(branch); err != nil {
		flogger.Errorw("reorg failed", "err", err)
		return
	}
	metrics.RecordReorg(head.Head.Height)
	d.mu.Lock()
	cb := d.onReorg
	d.mu.Unlock()
	if cb != nil {
		cb(head)
	}
}

// BestCandidate returns the pending competing head if any.
func (d *Detector) BestCandidate() *block.Block {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pendingHead
}

func localHash(b *block.Block) common.Hash {
	if b == nil {
		return common.Hash{}
	}
	return b.Hash
}
