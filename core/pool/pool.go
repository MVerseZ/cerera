package pool

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/cerera/core/common"
	"github.com/cerera/core/events"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/cerera/pallada"
	"github.com/prometheus/client_golang/prometheus"
)

const POOL_SERVICE_NAME = "POOL_CERERA_001_1_3"

var pLogger = logger.Named("pool")

var (
	poolSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pool_size",
		Help: "Current number of transactions in the pool",
	})
	poolBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pool_bytes",
		Help: "Current size of the pool in bytes",
	})
	poolTxAddedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pool_tx_added_total",
		Help: "Total number of transactions added to the pool",
	})
	poolTxRemovedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pool_tx_removed_total",
		Help: "Total number of transactions removed from the pool",
	})
	poolTxRejectedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pool_tx_rejected_total",
		Help: "Total number of transactions rejected (low gas or pool full)",
	})
	poolMaxSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pool_max_size",
		Help: "Maximum size of the pool",
	})
	poolGetPendingDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "pool_get_pending_duration_seconds",
		Help:    "Time spent getting pending transactions in seconds",
		Buckets: []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1, 2},
	})
)

func init() {
	prometheus.MustRegister(
		poolSize,
		poolBytes,
		poolTxAddedTotal,
		poolTxRemovedTotal,
		poolTxRejectedTotal,
		poolMaxSize,
		poolGetPendingDurationSeconds,
	)
}

type MemPoolInfo struct {
	Size             int     // current tx count
	Bytes            int     // size of tx
	Usage            uintptr // Total mem for mempool
	MaxMempool       int     // Maximum mem for mempool
	Mempoolminfee    int     // min fee for tx
	UnbroadcastCount int     // local pool transactions
	Hashes           []common.Hash
	Txs              []*types.GTransaction
}

// for simplification of mining, pool sorts transactions by gas*gasPrice with coefficient of payload
//
// @author gnupunk 28.02.2026
type Pool struct {
	maxSize   int
	memPool   map[common.Hash]*types.GTransaction
	txHeap    TxHeap
	heapItems map[common.Hash]*txHeapItem // O(1) lookup by hash → heap slot
	graph     *TxGraph                    // CPFP dependency graph

	maintainTicker *time.Ticker
	stopCh         chan struct{}

	Status byte

	Info MemPoolInfo

	mu sync.Mutex

	txValidator func(*types.GTransaction) bool
	events      events.Publisher
}

// var (
// 	gPoolMu sync.RWMutex
// 	GPool   *TxPool
// )

// func Get() *TxPool {
// 	gPoolMu.RLock()
// 	defer gPoolMu.RUnlock()
// 	return GPool
// }

type TxPool interface {
	AddRawTransaction(tx *types.GTransaction) error
	GetInfo() MemPoolInfo
	GetRawMemPool() []any
	GetPendingTransactions() []types.GTransaction
	GetTransaction(transactionHash common.Hash) *types.GTransaction
	// GetTopN returns the n most profitable transactions for block assembly.
	// Results are ordered from highest to lowest total fee (GasPrice × Gas).
	// Fast path: O(n log n), no dependency awareness.
	GetTopN(n int) []*types.GTransaction
	// GetMiningPackage returns up to n transactions in CPFP-aware,
	// dependency-safe order. Ancestors are always emitted before descendants;
	// packages are ranked by effective fee rate (ΣfeeChain / ΣgasChain) so a
	// high-fee child can pull its cheap parent into the block.
	GetMiningPackage(n int) []*types.GTransaction
	// Exec(method string, params []any) any
	QueueTransaction(tx *types.GTransaction)
	RemoveFromPool(txHash common.Hash) error
	RequeueTransactions(txs []types.GTransaction)
	SetTxValidator(func(*types.GTransaction) bool)
	SetEventPublisher(events.Publisher)
	Stop()
}

func (p *Pool) SetEventPublisher(pub events.Publisher) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = pub
}

func (p *Pool) publishTxAccepted(tx *types.GTransaction) {
	pub := p.events
	if pub == nil || tx == nil {
		return
	}
	_ = pub.Publish(events.Event{Kind: events.TxAccepted, Tx: tx})
}

// SetTxValidator sets optional admission validation (must return true to accept).
func (p *Pool) SetTxValidator(fn func(*types.GTransaction) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.txValidator = fn
}

// TxValidatorReady reports whether admission validation is configured.
func (p *Pool) TxValidatorReady() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.txValidator != nil
}

func InitPool(maxSize int) (TxPool, error) {
	mPool := make(map[common.Hash]*types.GTransaction)
	var p = &Pool{
		memPool:        mPool,
		txHeap:         make(TxHeap, 0, maxSize),
		heapItems:      make(map[common.Hash]*txHeapItem, maxSize),
		graph:          NewTxGraph(),
		maintainTicker: time.NewTicker(1500 * time.Millisecond),
		maxSize:        maxSize,
		stopCh:         make(chan struct{}),
		Status:         0xa,
	}
	pLogger.Infow("Init pool",
		"maxSize", p.maxSize,
	)
	p.Info = MemPoolInfo{
		Size:             0,
		Bytes:            0,
		Usage:            0,
		MaxMempool:       p.maxSize,
		UnbroadcastCount: 0,
		Hashes:           make([]common.Hash, 0),
		Txs:              make([]*types.GTransaction, 0),
	}

	poolMaxSize.Set(float64(p.maxSize))
	poolSize.Set(0)
	poolBytes.Set(0)

	go p.PoolServiceLoop()

	// gPoolMu.Lock()
	// GPool = p
	// gPoolMu.Unlock()

	return p, nil
}

// Stop gracefully shuts down the pool service loop.
func (p *Pool) Stop() {
	p.maintainTicker.Stop()
	close(p.stopCh)
}

// AddRawTransaction validates and adds a transaction to the pool.
// Returns an error if the transaction is rejected (duplicate, low gas, or pool full).
func (p *Pool) AddRawTransaction(tx *types.GTransaction) error {
	p.mu.Lock()

	if _, exists := p.memPool[tx.Hash()]; exists {
		p.mu.Unlock()
		return errors.New("transaction already in pool")
	}

	if p.txValidator != nil && !p.txValidator(tx) {
		poolTxRejectedTotal.Inc()
		p.mu.Unlock()
		return errors.New("transaction rejected by validator")
	}

	var minGas uint64
	if tx.Type() == types.LegacyTxType {
		minGas = pallada.MinTransferGas
	} else {
		var err error
		minGas, err = pallada.NewVM(tx.Data(), nil).PreCompile(tx.Data())
		if err != nil {
			pLogger.Errorw("Error precompile transaction", "error", err)
			p.mu.Unlock()
			return err
		}
	}

	if len(p.memPool) >= p.maxSize {
		poolTxRejectedTotal.Inc()
		p.mu.Unlock()
		return errors.New("pool is full")
	}
	if minGas > tx.Gas() {
		poolTxRejectedTotal.Inc()
		p.mu.Unlock()
		return errors.New("gas too low")
	}

	item := &txHeapItem{tx: tx, fee: tx.Cost()}
	p.memPool[tx.Hash()] = tx
	p.heapItems[tx.Hash()] = item
	heap.Push(&p.txHeap, item)
	p.graph.AddTx(tx) // wire CPFP dependency links (acquires graph.mu inside)
	poolTxAddedTotal.Inc()
	p.calcInfo()
	p.mu.Unlock()

	p.publishTxAccepted(tx)
	return nil
}

// Clear removes all transactions from the pool (for testing purposes).
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.memPool = make(map[common.Hash]*types.GTransaction)
	p.txHeap = make(TxHeap, 0, p.maxSize)
	p.heapItems = make(map[common.Hash]*txHeapItem, p.maxSize)
	p.graph = NewTxGraph()
	p.calcInfo()
}

// GetTopN returns up to n transactions with the highest total fee (GasPrice × Gas).
// It clones the heap items so the original heap is not modified.
// Cloning txHeapItem structs (not pointers) keeps the originals' idx intact while
// pops on the copy update only the clones - no heap.Init needed since the layout
// is structurally identical to the source heap.
func (p *Pool) GetTopN(n int) []*types.GTransaction {
	p.mu.Lock()
	defer p.mu.Unlock()

	sz := len(p.txHeap)
	if n > sz {
		n = sz
	}
	if n == 0 {
		return nil
	}

	// Clone item structs so Swap on the copy does not touch original idx values.
	tmp := make(TxHeap, sz)
	for i, item := range p.txHeap {
		clone := *item // copy struct (fee pointer shared - fee is read-only)
		tmp[i] = &clone
	}
	// tmp mirrors the same heap topology - valid heap, no Init needed.

	result := make([]*types.GTransaction, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, heap.Pop(&tmp).(*txHeapItem).tx)
	}
	return result
}

// GetMiningPackage returns up to n transactions in CPFP-aware, dependency-safe
// order by delegating to the internal TxGraph. The graph has its own lock so
// this method does not need to hold p.mu, keeping it non-blocking for miners.
func (p *Pool) GetMiningPackage(n int) []*types.GTransaction {
	return p.graph.GetMiningPackage(n)
}

// calcInfo recalculates MemPoolInfo from current memPool state. Must be called under p.mu.
func (p *Pool) calcInfo() {
	var txPoolSize = 0
	hashes := make([]common.Hash, 0, len(p.memPool))
	cp := make([]*types.GTransaction, 0, len(p.memPool))

	for _, k := range p.memPool {
		txPoolSize += int(k.Size())
		hashes = append(hashes, k.Hash())
		cp = append(cp, k)
	}

	p.Info = MemPoolInfo{
		Size:          len(hashes),
		Bytes:         txPoolSize,
		Usage:         0,
		MaxMempool:    p.maxSize,
		Mempoolminfee: 0,
		Hashes:        hashes,
		Txs:           cp,
	}
	poolSize.Set(float64(len(hashes)))
	poolBytes.Set(float64(txPoolSize))
}

// GetInfo returns a snapshot of the current pool state.
func (p *Pool) GetInfo() MemPoolInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Info
}

// GetMinimalGasValue returns the minimum transfer cost in CER.
// 1 gas unit = 1 DUST, 1 CER = 1,000,000 DUST → cost_CER = gasUnits / 1,000,000.
func (p *Pool) GetMinimalGasValue() float64 {
	// Canonical minimal transfer bytecode for Pallada VM:
	//   1. SLOAD sender balance
	//   2. SUB amount  → write back with SSTORE
	//   3. SLOAD receiver balance
	//   4. ADD amount  → write back with SSTORE
	//   5. STOP
	transferBytecode := []byte{
		0x60, 0x00, // PUSH1 0  (sender storage key)
		0x54,       // SLOAD    (load sender balance)
		0x60, 0x01, // PUSH1 1  (amount placeholder)
		0x03,       // SUB      (sender_bal - amount)
		0x60, 0x00, // PUSH1 0  (sender storage key)
		0x55,       // SSTORE   (save new sender balance)
		0x60, 0x01, // PUSH1 1  (amount placeholder)
		0x60, 0x01, // PUSH1 1  (receiver storage key)
		0x54,       // SLOAD    (load receiver balance)
		0x01,       // ADD      (receiver_bal + amount)
		0x60, 0x01, // PUSH1 1  (receiver storage key)
		0x55, // SSTORE   (save new receiver balance)
		0x00, // STOP
	}
	gasUnits, err := pallada.NewVM(transferBytecode, nil).PreCompile(transferBytecode)
	if err != nil {
		pLogger.Errorw("Error precompile transfer bytecode", "error", err)
		return 0
	}
	return float64(gasUnits) / 1_000_000
}

// GetPendingTransactions returns a copy of all pending transactions.
func (p *Pool) GetPendingTransactions() []types.GTransaction {
	start := time.Now()
	defer func() {
		poolGetPendingDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]types.GTransaction, 0, len(p.memPool))
	for _, k := range p.memPool {
		result = append(result, *k)
	}
	return result
}

// GetRawMemPool returns hashes of all transactions currently in the pool.
func (p *Pool) GetRawMemPool() []any {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]any, 0, len(p.memPool))
	for hash := range p.memPool {
		result = append(result, hash)
	}
	return result
}

// QueueTransaction adds a transaction to the pool, discarding any error.
func (p *Pool) QueueTransaction(tx *types.GTransaction) {
	//nolint:errcheck
	_ = p.AddRawTransaction(tx)
}

// SendTransaction adds a FULL transaction to the pool and returns its hash even tx invalid. (tx will be dropped later).
func (p *Pool) SendTransaction(tx types.GTransaction) (common.Hash, error) {
	if err := p.AddRawTransaction(&tx); err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

// GetTransaction returns a copy of the transaction with the given hash, or nil if not found.
func (p *Pool) GetTransaction(transactionHash common.Hash) *types.GTransaction {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, ok := p.memPool[transactionHash]
	if !ok {
		return nil
	}
	cp := *tx
	return &cp
}

// PoolServiceLoop runs the pool maintenance ticker until Stop is called.
func (p *Pool) PoolServiceLoop() {
	for {
		select {
		case <-p.maintainTicker.C:
			// maintenance hook: eviction, repricing, etc.
		case <-p.stopCh:
			return
		}
	}
}

// RemoveFromPool deletes a transaction from the pool by hash.
func (p *Pool) RemoveFromPool(txHash common.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.memPool[txHash]; !ok {
		return errors.New("transaction not in mempool")
	}
	item := p.heapItems[txHash]
	heap.Remove(&p.txHeap, item.idx) // O(log n) — no full rebuild
	delete(p.heapItems, txHash)
	delete(p.memPool, txHash)
	p.graph.RemoveTx(txHash) // rewire CPFP links for orphaned descendants
	poolTxRemovedTotal.Inc()
	p.calcInfo()
	return nil
}

// RequeueTransactions adds executed block txs back to the mempool when a reorg orphans them.
func (p *Pool) RequeueTransactions(txs []types.GTransaction) {
	for i := range txs {
		tx := txs[i]
		if tx.Type() == types.CoinbaseTxType || tx.Type() == types.FaucetTxType {
			continue
		}
		cp := tx
		_ = p.AddRawTransaction(&cp)
	}
}

// UpdateTx replaces an existing transaction in the pool (gas or message update, not value).
// heap.Fix repositions the item in O(log n) after the fee may have changed.
func (p *Pool) UpdateTx(newTx types.GTransaction) {
	p.mu.Lock()
	item, exists := p.heapItems[newTx.Hash()]
	if !exists {
		p.mu.Unlock()
		return
	}
	pLogger.Infow("Replace sign tx in pool",
		"hash", newTx.Hash(),
		"signed", newTx.IsSigned(),
	)
	stored := &newTx
	p.memPool[newTx.Hash()] = stored
	item.tx = stored
	item.fee = stored.Cost() // recalculate cached fee
	heap.Fix(&p.txHeap, item.idx)
	p.calcInfo()
	p.mu.Unlock()

	p.publishTxAccepted(stored)
}
