package pool

import (
	"container/heap"
	"math/big"
	"sync"
	"testing"

	"github.com/cerera/core/address"
	"github.com/cerera/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTx builds a LegacyTx with the given nonce, gas, and gasPrice so that
// Cost() = gas × gasPrice is fully predictable in tests.
// gas=3333 satisfies pallada.MinTransferGas (632) for integration tests.
// A unique byte payload per nonce guarantees distinct hashes.
func makeTx(nonce uint64, gas uint64, gasPrice int64) *types.GTransaction {
	return types.NewTransaction(
		nonce,
		address.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(1),
		gas,
		big.NewInt(gasPrice),
		[]byte{byte(nonce), byte(nonce >> 8)}, // unique per nonce
	)
}

// newRawItem builds a txHeapItem directly, bypassing pool gas validation.
// Used in pure heap unit tests where AddRawTransaction is not involved.
func newRawItem(fee int64) *txHeapItem {
	tx := makeTx(uint64(fee), 1, fee)
	return &txHeapItem{tx: tx, fee: big.NewInt(fee), idx: 0}
}

// newTestPool creates a Pool, registers a Cleanup hook to stop it, and returns
// the concrete *Pool so tests can inspect internal fields (txHeap, heapItems).
func newTestPool(t *testing.T, size int) *Pool {
	t.Helper()
	iface, err := InitPool(size)
	require.NoError(t, err)
	p := iface.(*Pool)
	t.Cleanup(p.Stop)
	return p
}

// ─── TxHeap unit tests (direct heap manipulation) ───────────────────────────

// TestTxHeap_MaxHeapProperty inserts items in random fee order and verifies
// every Pop returns a fee ≤ the previous one (descending / max-heap order).
func TestTxHeap_MaxHeapProperty(t *testing.T) {
	fees := []int64{50, 200, 10, 300, 5, 150}
	h := make(TxHeap, 0, len(fees))
	for _, f := range fees {
		heap.Push(&h, newRawItem(f))
	}

	prev := int64(1<<62 - 1)
	for h.Len() > 0 {
		got := heap.Pop(&h).(*txHeapItem).fee.Int64()
		assert.LessOrEqual(t, got, prev, "heap must return fees in descending order")
		prev = got
	}
}

// TestTxHeap_IdxConsistencyAfterPush verifies that after every Push the idx
// of every element in the backing slice equals its actual position.
func TestTxHeap_IdxConsistencyAfterPush(t *testing.T) {
	h := make(TxHeap, 0, 8)
	for _, f := range []int64{40, 10, 70, 20, 60} {
		heap.Push(&h, newRawItem(f))
		for i, item := range h {
			assert.Equalf(t, i, item.idx, "idx mismatch at position %d after push (fee=%d)", i, item.fee.Int64())
		}
	}
}

// TestTxHeap_IdxConsistencyAfterPop verifies idx stays consistent after every Pop.
func TestTxHeap_IdxConsistencyAfterPop(t *testing.T) {
	h := make(TxHeap, 0, 8)
	for _, f := range []int64{100, 30, 70, 55, 20} {
		heap.Push(&h, newRawItem(f))
	}

	for h.Len() > 0 {
		heap.Pop(&h)
		for i, item := range h {
			assert.Equalf(t, i, item.idx, "idx mismatch at position %d after pop", i)
		}
	}
}

// TestTxHeap_PopSetsIdxNegative verifies that the item returned by Pop has idx = -1,
// signalling "not in any heap" for safe re-use detection.
func TestTxHeap_PopSetsIdxNegative(t *testing.T) {
	h := make(TxHeap, 0, 2)
	heap.Push(&h, newRawItem(42))
	popped := heap.Pop(&h).(*txHeapItem)
	assert.Equal(t, -1, popped.idx, "popped item must have idx = -1")
}

// TestTxHeap_SwapUpdatesIdx verifies that Swap correctly updates the idx field of
// both swapped elements, so heap.Remove / heap.Fix receive valid indices later.
func TestTxHeap_SwapUpdatesIdx(t *testing.T) {
	h := TxHeap{newRawItem(1), newRawItem(2), newRawItem(3)}
	for i, item := range h {
		item.idx = i
	}

	h.Swap(0, 2)

	assert.Equal(t, 0, h[0].idx, "h[0].idx must be 0 after Swap(0,2)")
	assert.Equal(t, 2, h[2].idx, "h[2].idx must be 2 after Swap(0,2)")
}

// TestTxHeap_HeapRemoveArbitraryElement pushes five items then removes the one
// with the middle fee using heap.Remove. Verifies the removed item is gone, the
// idx of all remaining items is correct, and the max-heap property still holds.
func TestTxHeap_HeapRemoveArbitraryElement(t *testing.T) {
	fees := []int64{80, 20, 60, 40, 10}
	h := make(TxHeap, 0, len(fees))
	items := make([]*txHeapItem, len(fees))
	for i, f := range fees {
		items[i] = newRawItem(f)
		heap.Push(&h, items[i])
	}

	// Remove the item with fee=60 (mid-range priority).
	removedFee := items[2].fee.Int64()
	heap.Remove(&h, items[2].idx)

	// The removed fee must not appear in the remaining heap.
	for _, item := range h {
		assert.NotEqual(t, removedFee, item.fee.Int64(), "removed item must not remain in heap")
	}

	// idx of every remaining item must equal its position.
	for i, item := range h {
		assert.Equalf(t, i, item.idx, "idx mismatch at position %d after heap.Remove", i)
	}

	// Max-heap property must still hold.
	prev := int64(1<<62 - 1)
	for h.Len() > 0 {
		got := heap.Pop(&h).(*txHeapItem).fee.Int64()
		assert.LessOrEqual(t, got, prev, "heap property must hold after Remove")
		prev = got
	}
}

// ─── Pool integration tests ──────────────────────────────────────────────────

// TestGetTopN_ReturnsInDescendingFeeOrder adds three transactions with known fees
// and verifies GetTopN returns them from highest to lowest Cost().
func TestGetTopN_ReturnsInDescendingFeeOrder(t *testing.T) {
	p := newTestPool(t, 50)

	// Cost = gas × gasPrice:  3333×50=166 650,  3333×100=333 300,  3333×200=666 600
	txLow := makeTx(1, 3333, 50)
	txMid := makeTx(2, 3333, 100)
	txHigh := makeTx(3, 3333, 200)
	require.NoError(t, p.AddRawTransaction(txLow))
	require.NoError(t, p.AddRawTransaction(txMid))
	require.NoError(t, p.AddRawTransaction(txHigh))

	top := p.GetTopN(3)
	require.Len(t, top, 3)
	assert.Equal(t, txHigh.Hash(), top[0].Hash(), "top[0] must be highest-fee tx")
	assert.Equal(t, txMid.Hash(), top[1].Hash(), "top[1] must be middle-fee tx")
	assert.Equal(t, txLow.Hash(), top[2].Hash(), "top[2] must be lowest-fee tx")
}

// TestGetTopN_NGreaterThanPoolSize verifies no panic and all transactions are
// returned when the requested count exceeds the actual pool size.
func TestGetTopN_NGreaterThanPoolSize(t *testing.T) {
	p := newTestPool(t, 50)
	for i := 1; i <= 3; i++ {
		require.NoError(t, p.AddRawTransaction(makeTx(uint64(i), 3333, int64(i*10))))
	}

	top := p.GetTopN(100)
	assert.Len(t, top, 3, "should return all txs when n > pool size")
}

// TestGetTopN_EmptyPool verifies GetTopN returns nil (not an empty slice) when
// the pool contains no transactions.
func TestGetTopN_EmptyPool(t *testing.T) {
	p := newTestPool(t, 50)
	assert.Nil(t, p.GetTopN(10), "GetTopN on empty pool must return nil")
}

// TestGetTopN_IsIdempotent calls GetTopN twice and asserts the results are
// identical, proving the original heap is left intact by the first call.
func TestGetTopN_IsIdempotent(t *testing.T) {
	p := newTestPool(t, 50)
	txs := []*types.GTransaction{
		makeTx(1, 3333, 300),
		makeTx(2, 3333, 100),
		makeTx(3, 3333, 200),
	}
	for _, tx := range txs {
		require.NoError(t, p.AddRawTransaction(tx))
	}

	first := p.GetTopN(3)
	second := p.GetTopN(3)
	require.Len(t, second, len(first))
	for i := range first {
		assert.Equal(t, first[i].Hash(), second[i].Hash(), "results must be identical between calls")
	}
}

// TestGetTopN_DoesNotShrinkInternalHeap verifies the internal txHeap length is
// unchanged after a call — GetTopN must pop from a copy, not the real heap.
func TestGetTopN_DoesNotShrinkInternalHeap(t *testing.T) {
	p := newTestPool(t, 50)
	for i := 1; i <= 5; i++ {
		require.NoError(t, p.AddRawTransaction(makeTx(uint64(i), 3333, int64(i*10))))
	}

	before := p.txHeap.Len()
	p.GetTopN(3)
	assert.Equal(t, before, p.txHeap.Len(), "GetTopN must not shrink the internal heap")
}

// TestRemoveFromPool_HeapRemainsValid removes the middle-fee transaction and
// verifies the remaining two come out of GetTopN in the correct order.
func TestRemoveFromPool_HeapRemainsValid(t *testing.T) {
	p := newTestPool(t, 50)
	txLow := makeTx(1, 3333, 50)
	txMid := makeTx(2, 3333, 100)
	txHigh := makeTx(3, 3333, 200)
	require.NoError(t, p.AddRawTransaction(txLow))
	require.NoError(t, p.AddRawTransaction(txMid))
	require.NoError(t, p.AddRawTransaction(txHigh))

	require.NoError(t, p.RemoveFromPool(txMid.Hash()))

	top := p.GetTopN(10)
	require.Len(t, top, 2, "should have 2 transactions after removal")
	assert.Equal(t, txHigh.Hash(), top[0].Hash(), "top[0] must be highest-fee tx")
	assert.Equal(t, txLow.Hash(), top[1].Hash(), "top[1] must be lowest-fee tx")
}

// TestRemoveFromPool_MapsStayInSync verifies that heapItems and memPool always
// have the same number of entries after a series of additions and removals.
func TestRemoveFromPool_MapsStayInSync(t *testing.T) {
	p := newTestPool(t, 50)
	txs := make([]*types.GTransaction, 5)
	for i := range txs {
		txs[i] = makeTx(uint64(i+1), 3333, int64((i+1)*10))
		require.NoError(t, p.AddRawTransaction(txs[i]))
	}

	require.NoError(t, p.RemoveFromPool(txs[1].Hash()))
	require.NoError(t, p.RemoveFromPool(txs[3].Hash()))

	assert.Equal(t, len(p.memPool), len(p.heapItems), "heapItems count must equal memPool count")
	assert.Equal(t, len(p.memPool), p.txHeap.Len(), "txHeap length must equal memPool count")

	_, stillPresent := p.heapItems[txs[1].Hash()]
	assert.False(t, stillPresent, "removed tx must not remain in heapItems")
}

// TestRemoveFromPool_IdxConsistencyAfterRemoval verifies that after removing the
// current heap root (max-fee element), all remaining elements have idx == position.
func TestRemoveFromPool_IdxConsistencyAfterRemoval(t *testing.T) {
	p := newTestPool(t, 50)
	for i := 1; i <= 6; i++ {
		require.NoError(t, p.AddRawTransaction(makeTx(uint64(i), 3333, int64(i*15))))
	}

	// Remove whatever is currently at the root (max-fee element).
	rootHash := p.txHeap[0].tx.Hash()
	require.NoError(t, p.RemoveFromPool(rootHash))

	for i, item := range p.txHeap {
		assert.Equalf(t, i, item.idx, "idx mismatch at position %d after removing root", i)
	}
}

// TestRemoveFromPool_NonExistentReturnsError verifies that removing a hash not
// present in the pool returns an error without panicking.
func TestRemoveFromPool_NonExistentReturnsError(t *testing.T) {
	p := newTestPool(t, 50)
	tx := makeTx(1, 3333, 100)
	err := p.RemoveFromPool(tx.Hash())
	assert.Error(t, err, "removing a non-existent tx must return an error")
}

// TestHeapItem_CachedFeeMatchesTxCost verifies that the fee stored in txHeapItem
// equals tx.Cost() computed at insertion time — the cache is correct.
func TestHeapItem_CachedFeeMatchesTxCost(t *testing.T) {
	p := newTestPool(t, 50)
	tx := makeTx(1, 3333, 777)
	require.NoError(t, p.AddRawTransaction(tx))

	item := p.heapItems[tx.Hash()]
	require.NotNil(t, item)

	expected := tx.Cost()
	assert.Equal(t, 0, expected.Cmp(item.fee),
		"cached fee (%s) must equal tx.Cost() (%s)", item.fee, expected)
}

// TestHeapItem_CachedFeeUsedInComparison verifies that Less() is consistent with
// the ordering of Cost() values — the cached fee drives the heap correctly.
func TestHeapItem_CachedFeeUsedInComparison(t *testing.T) {
	p := newTestPool(t, 50)

	txSmall := makeTx(1, 3333, 10)
	txLarge := makeTx(2, 3333, 1000)
	require.NoError(t, p.AddRawTransaction(txSmall))
	require.NoError(t, p.AddRawTransaction(txLarge))

	itemSmall := p.heapItems[txSmall.Hash()]
	itemLarge := p.heapItems[txLarge.Hash()]

	h := TxHeap{itemSmall, itemLarge}
	assert.True(t, h.Less(1, 0), "Less(large, small) must be true (large has higher fee)")
	assert.False(t, h.Less(0, 1), "Less(small, large) must be false")
}

// ─── Concurrency tests ────────────────────────────────────────────────────────

// TestConcurrent_GetTopNNoRace verifies concurrent GetTopN calls do not race and
// always return a non-nil, correctly-sized result.
func TestConcurrent_GetTopNNoRace(t *testing.T) {
	p := newTestPool(t, 200)
	for i := 1; i <= 100; i++ {
		require.NoError(t, p.AddRawTransaction(makeTx(uint64(i), 3333, int64(i*7))))
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			top := p.GetTopN(10)
			assert.Len(t, top, 10, "concurrent GetTopN must return exactly n results")
		}()
	}
	wg.Wait()
}

// TestConcurrent_AddAndGetTopNNoRace mixes concurrent AddRawTransaction and
// GetTopN calls. The -race detector must find no data races.
func TestConcurrent_AddAndGetTopNNoRace(t *testing.T) {
	p := newTestPool(t, 500)

	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = p.AddRawTransaction(makeTx(uint64(n), 3333, int64(n*3)))
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.GetTopN(5)
		}()
	}
	wg.Wait()
}

// TestConcurrent_RemoveAndGetTopNNoRace interleaves concurrent RemoveFromPool and
// GetTopN calls. Confirms no data races and no panics from stale idx values.
func TestConcurrent_RemoveAndGetTopNNoRace(t *testing.T) {
	p := newTestPool(t, 200)
	txs := make([]*types.GTransaction, 60)
	for i := range txs {
		txs[i] = makeTx(uint64(i+1), 3333, int64((i+1)*5))
		require.NoError(t, p.AddRawTransaction(txs[i]))
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = p.RemoveFromPool(txs[idx].Hash())
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.GetTopN(5)
		}()
	}
	wg.Wait()
}

// ─── Benchmarks ──────────────────────────────────────────────────────────────

// BenchmarkTxHeapLess_Allocs measures allocations in the Less() hot path.
// With cached fee, the result must be 0 allocs/op — no big.Int created per call.
func BenchmarkTxHeapLess_Allocs(b *testing.B) {
	a := &txHeapItem{fee: big.NewInt(12_345_678)}
	c := &txHeapItem{fee: big.NewInt(87_654_321)}
	h := TxHeap{a, c}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Less(0, 1)
	}
}

// BenchmarkGetTopN_10k measures GetTopN(100) throughput on a pool of 10 000 txs.
func BenchmarkGetTopN_10k(b *testing.B) {
	p, _ := InitPool(11_000)
	defer p.Stop()

	for i := 1; i <= 10_000; i++ {
		_ = p.AddRawTransaction(makeTx(uint64(i), 3333, int64(i)))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.GetTopN(100)
	}
}

// BenchmarkAddRawTransaction_Allocs measures per-Add allocations to track
// overhead introduced by txHeapItem creation.
func BenchmarkAddRawTransaction_Allocs(b *testing.B) {
	p, _ := InitPool(b.N + 10)
	defer p.Stop()

	txs := make([]*types.GTransaction, b.N)
	for i := range txs {
		txs[i] = makeTx(uint64(i+1), 3333, int64((i+1)*3))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.AddRawTransaction(txs[i])
	}
}
