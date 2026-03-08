package pool

import (
	"math/big"

	"github.com/cerera/core/types"
)

// txHeapItem wraps a transaction with two pre-computed/maintained fields that make
// heap operations allocation-free and O(log n):
//
//   - fee: Cost() = GasPrice × Gas, computed once on insert and on fee update.
//     Caching avoids two big.Int allocations per Less() call (the hot path).
//   - idx: current position in the TxHeap slice, kept up-to-date by Swap.
//     Enables heap.Remove / heap.Fix in O(log n) without a full rebuild.
type txHeapItem struct {
	tx  *types.GTransaction
	fee *big.Int // immutable while in heap; replaced atomically on UpdateTx
	idx int      // current index; -1 when not in any heap
}

// TxHeap is a max-heap of txHeapItem pointers ordered by total fee descending.
// Miners see the highest-paying transaction at index 0.
type TxHeap []*txHeapItem

func (h TxHeap) Len() int { return len(h) }

// Less is the hot comparison path — zero allocations because fee is pre-cached.
func (h TxHeap) Less(i, j int) bool { return h[i].fee.Cmp(h[j].fee) > 0 }

// Swap keeps idx in sync so RemoveFromPool / UpdateTx can call heap.Remove / heap.Fix
// with the correct position in O(log n).
func (h TxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idx = i
	h[j].idx = j
}

func (h *TxHeap) Push(x any) {
	item := x.(*txHeapItem)
	item.idx = len(*h)
	*h = append(*h, item)
}

func (h *TxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // prevent GC leak
	item.idx = -1
	*h = old[:n-1]
	return item
}
