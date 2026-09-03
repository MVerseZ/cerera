package chain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

const (
	maxOrphanBlocks  = 256
	maxOrphanHeights = 64
)

// BlockLookup resolves a block hash from canonical chain and/or orphan store.
type BlockLookup func(common.Hash) *block.Block

// ReorgHandler replays vault/tx state after a chain reorg.
var ReorgHandler func(blocks []*block.Block) error

// SetReorgHandler registers the callback invoked after Reorg truncates chain data.
func SetReorgHandler(fn func(blocks []*block.Block) error) {
	ReorgHandler = fn
}

// BlockDifficulty returns the header difficulty as big.Int.
func BlockDifficulty(b *block.Block) *big.Int {
	if b == nil || b.Head == nil {
		return big.NewInt(0)
	}
	return new(big.Int).SetUint64(b.Head.Difficulty)
}

// TotalDifficultyForBlocks sums header difficulties along a block slice.
func TotalDifficultyForBlocks(blocks []*block.Block) *big.Int {
	total := big.NewInt(0)
	for _, b := range blocks {
		total.Add(total, BlockDifficulty(b))
	}
	return total
}

// TotalDifficulty returns cumulative difficulty of the canonical chain.
func (bc *Chain) TotalDifficulty() *big.Int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return TotalDifficultyForBlocks(bc.data)
}

// TotalDifficultyOf returns cumulative difficulty up to and including block hash.
func TotalDifficultyOf(headHash common.Hash, lookup BlockLookup) *big.Int {
	total := big.NewInt(0)
	h := headHash
	seen := make(map[common.Hash]struct{})
	for h != (common.Hash{}) {
		if _, ok := seen[h]; ok {
			break
		}
		seen[h] = struct{}{}
		var b *block.Block
		if lookup != nil {
			b = lookup(h)
		}
		if b == nil || b.Head == nil {
			break
		}
		total.Add(total, BlockDifficulty(b))
		if b.Head.Height == 0 {
			break
		}
		h = b.Head.PrevHash
	}
	return total
}

// GetBlockByHeight returns the canonical block at the given height.
func (bc *Chain) GetBlockByHeight(height int) *block.Block {
	if height < 0 {
		return nil
	}
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	for _, b := range bc.data {
		if b != nil && b.Head != nil && b.Head.Height == height {
			return b
		}
	}
	return nil
}

// GetBlockHashAtHeight returns the hash of the canonical block at height.
func (bc *Chain) GetBlockHashAtHeight(height int) common.Hash {
	if b := bc.GetBlockByHeight(height); b != nil {
		return b.Hash
	}
	return common.EmptyHash()
}

// FindCommonAncestor walks two branches back to the shared block.
func FindCommonAncestor(hashA, hashB common.Hash, lookup BlockLookup) (height int, hash common.Hash, err error) {
	if hashA == (common.Hash{}) || hashB == (common.Hash{}) {
		return 0, common.EmptyHash(), errors.New("empty hash")
	}
	if hashA == hashB {
		b := lookup(hashA)
		if b == nil || b.Head == nil {
			return 0, hashA, nil
		}
		return b.Head.Height, hashA, nil
	}

	heightA := blockHeightOf(hashA, lookup)
	heightB := blockHeightOf(hashB, lookup)
	hA, hB := hashA, hashB

	for heightA > heightB {
		b := lookup(hA)
		if b == nil || b.Head == nil {
			return 0, common.EmptyHash(), fmt.Errorf("missing block %s", hA.Hex())
		}
		hA = b.Head.PrevHash
		heightA--
	}
	for heightB > heightA {
		b := lookup(hB)
		if b == nil || b.Head == nil {
			return 0, common.EmptyHash(), fmt.Errorf("missing block %s", hB.Hex())
		}
		hB = b.Head.PrevHash
		heightB--
	}

	for hA != hB {
		bA := lookup(hA)
		bB := lookup(hB)
		if bA == nil || bA.Head == nil || bB == nil || bB.Head == nil {
			return 0, common.EmptyHash(), errors.New("ancestor walk failed")
		}
		if bA.Head.Height == 0 {
			return 0, bA.Hash, nil
		}
		hA = bA.Head.PrevHash
		hB = bB.Head.PrevHash
		heightA--
	}
	return heightA, hA, nil
}

func blockHeightOf(h common.Hash, lookup BlockLookup) int {
	b := lookup(h)
	if b == nil || b.Head == nil {
		return -1
	}
	return b.Head.Height
}

// CompareChainHeads compares two heads by total difficulty, height, then hash.
// Returns -1 if a is heavier, 1 if b is heavier, 0 if equal.
func CompareChainHeads(a, b *block.Block, lookup BlockLookup) int {
	if a == nil {
		if b == nil {
			return 0
		}
		return 1
	}
	if b == nil {
		return -1
	}
	diffA := TotalDifficultyOf(a.Hash, lookup)
	diffB := TotalDifficultyOf(b.Hash, lookup)
	if cmp := diffA.Cmp(diffB); cmp != 0 {
		return -cmp
	}
	if a.Head != nil && b.Head != nil {
		if a.Head.Height != b.Head.Height {
			if a.Head.Height > b.Head.Height {
				return -1
			}
			return 1
		}
	}
	if a.Hash.Hex() > b.Hash.Hex() {
		return -1
	}
	if a.Hash.Hex() < b.Hash.Hex() {
		return 1
	}
	return 0
}

// OrphanManager stores blocks whose parent is not yet canonical.
type OrphanManager struct {
	mu sync.RWMutex

	byHash   map[common.Hash]*block.Block
	byHeight map[int][]common.Hash
	lookup   BlockLookup
}

// NewOrphanManager creates an orphan block store.
func NewOrphanManager(lookup BlockLookup) *OrphanManager {
	return &OrphanManager{
		byHash:   make(map[common.Hash]*block.Block),
		byHeight: make(map[int][]common.Hash),
		lookup:   lookup,
	}
}

// Count returns the number of stored orphan blocks.
func (om *OrphanManager) Count() int {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return len(om.byHash)
}

// Get returns an orphan by hash.
func (om *OrphanManager) Get(hash common.Hash) *block.Block {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.byHash[hash]
}

// Lookup resolves hash from canonical chain first, then orphans.
func (om *OrphanManager) Lookup(hash common.Hash) *block.Block {
	if om.lookup != nil {
		if b := om.lookup(hash); b != nil {
			return b
		}
	}
	return om.Get(hash)
}

// Add stores a block as orphan when parent is unknown to canonical chain.
func (om *OrphanManager) Add(b *block.Block) bool {
	if b == nil || b.Head == nil {
		return false
	}
	om.mu.Lock()
	defer om.mu.Unlock()

	if len(om.byHash) >= maxOrphanBlocks {
		return false
	}
	if _, exists := om.byHash[b.Hash]; exists {
		return false
	}
	om.byHash[b.Hash] = b
	h := b.Head.Height
	om.byHeight[h] = append(om.byHeight[h], b.Hash)
	if len(om.byHeight) > maxOrphanHeights {
		om.evictOldestHeight()
	}
	return true
}

func (om *OrphanManager) evictOldestHeight() {
	minH := -1
	for h := range om.byHeight {
		if minH < 0 || h < minH {
			minH = h
		}
	}
	if minH < 0 {
		return
	}
	for _, hash := range om.byHeight[minH] {
		delete(om.byHash, hash)
	}
	delete(om.byHeight, minH)
}

// TryLinkOrphans returns orphan chains whose parent hash is now known.
func (om *OrphanManager) TryLinkOrphans(parentHash common.Hash) []*block.Block {
	om.mu.Lock()
	defer om.mu.Unlock()

	var linked []*block.Block
	for hash, b := range om.byHash {
		if b.Head == nil || b.Head.PrevHash != parentHash {
			continue
		}
		linked = append(linked, b)
		delete(om.byHash, hash)
		if heights, ok := om.byHeight[b.Head.Height]; ok {
			om.byHeight[b.Head.Height] = removeHash(heights, hash)
		}
	}
	return linked
}

func removeHash(list []common.Hash, target common.Hash) []common.Hash {
	out := list[:0]
	for _, h := range list {
		if h != target {
			out = append(out, h)
		}
	}
	return out
}

// BuildChainToHead walks prevHash from head through lookup to assemble a branch.
func BuildChainToHead(head *block.Block, lookup BlockLookup) ([]*block.Block, error) {
	if head == nil || head.Head == nil {
		return nil, errors.New("head is nil")
	}
	var stack []*block.Block
	cur := head
	seen := make(map[common.Hash]struct{})
	for cur != nil {
		if _, ok := seen[cur.Hash]; ok {
			return nil, errors.New("cycle in orphan branch")
		}
		seen[cur.Hash] = struct{}{}
		stack = append(stack, cur)
		if cur.Head.Height == 0 {
			break
		}
		cur = lookup(cur.Head.PrevHash)
	}
	for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
		stack[i], stack[j] = stack[j], stack[i]
	}
	return stack, nil
}

// ResetHeightLock clears the height lock after a reorg.
func (bc *Chain) ResetHeightLock() {
	bc.heightMu.Lock()
	defer bc.heightMu.Unlock()
	bc.lockedHeight = 0
	oldCh := bc.cancelCh
	bc.cancelCh = make(chan struct{})
	if oldCh != nil {
		close(oldCh)
	}
}

// Reorg switches canonical chain to newCanonical blocks sharing a common ancestor.
func (bc *Chain) Reorg(newCanonical []*block.Block) error {
	if len(newCanonical) == 0 {
		return errors.New("empty reorg chain")
	}
	newHead := newCanonical[len(newCanonical)-1]
	if newHead == nil || newHead.Head == nil {
		return errors.New("invalid reorg head")
	}

	localHead := bc.GetLatestBlock()
	if localHead == nil {
		return errors.New("local chain empty")
	}

	branchByHash := make(map[common.Hash]*block.Block, len(newCanonical))
	for _, b := range newCanonical {
		if b != nil {
			branchByHash[b.Hash] = b
		}
	}
	lookup := func(h common.Hash) *block.Block {
		if b := branchByHash[h]; b != nil {
			return b
		}
		return bc.GetBlock(h)
	}
	forkHeight, _, err := FindCommonAncestor(localHead.Hash, newHead.Hash, lookup)
	if err != nil {
		return err
	}

	bc.mu.Lock()
	if forkHeight+1 > len(bc.data) {
		bc.mu.Unlock()
		return fmt.Errorf("fork height %d beyond chain length %d", forkHeight, len(bc.data))
	}
	bc.data = append([]*block.Block(nil), bc.data[:forkHeight+1]...)
	if len(bc.data) > 0 {
		bc.currentBlock = bc.data[len(bc.data)-1]
	}

	for _, b := range newCanonical {
		if b == nil || b.Head == nil || b.Head.Height <= forkHeight {
			continue
		}
		bc.data = append(bc.data, b)
		bc.currentBlock = b
	}
	bc.recalcInfoLocked()
	bc.mu.Unlock()

	bc.ResetHeightLock()

	if bc.storage != nil && bc.chainPath != "" {
		bc.mu.RLock()
		dataCopy := append([]*block.Block(nil), bc.data...)
		bc.mu.RUnlock()
		if err := RewriteChainFile(dataCopy, bc.chainPath); err != nil {
			return err
		}
	}

	if ReorgHandler != nil {
		bc.mu.RLock()
		dataCopy := append([]*block.Block(nil), bc.data...)
		bc.mu.RUnlock()
		return ReorgHandler(dataCopy)
	}
	return nil
}

func (bc *Chain) recalcInfoLocked() {
	bc.info = BlockChainStatus{
		Total: len(bc.data),
	}
	if len(bc.data) == 0 {
		bc.info.Latest = common.EmptyHash()
		return
	}
	head := bc.data[len(bc.data)-1]
	bc.info.Latest = head.GetHash()
	if head.Head != nil {
		bc.info.Difficulty = head.Head.Difficulty
	}
	var size int64
	var txs uint64
	var gas uint64
	for _, b := range bc.data {
		if b == nil || b.Head == nil {
			continue
		}
		size += int64(b.Head.Size)
		txs += uint64(len(b.Transactions))
		for _, tx := range b.Transactions {
			txType := tx.Type()
			if txType != types.CoinbaseTxType && txType != types.FaucetTxType {
				gas += uint64(tx.Gas())
			}
		}
		bc.info.ChainWork += b.Head.Size
	}
	bc.info.Size = size
	bc.info.Txs = txs
	bc.info.Gas = gas
	if bc.metrics != nil && head.Head != nil {
		bc.metrics.height.Set(float64(head.Head.Height))
	}
}
