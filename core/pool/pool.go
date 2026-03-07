package pool

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/observer"
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

	maintainTicker *time.Ticker
	stopCh         chan struct{}

	Status byte

	observers []observer.Observer

	Info MemPoolInfo

	mu          sync.Mutex
	observersMu sync.RWMutex
}

var (
	gPoolMu sync.RWMutex
	GPool   TxPool
)

func Get() TxPool {
	gPoolMu.RLock()
	defer gPoolMu.RUnlock()
	return GPool
}

type TxPool interface {
	AddRawTransaction(tx *types.GTransaction) error
	GetInfo() MemPoolInfo
	GetRawMemPool() []any
	GetPendingTransactions() []types.GTransaction
	GetTransaction(transactionHash common.Hash) *types.GTransaction
	// GetTopN returns the n most profitable transactions for block assembly.
	// Results are ordered from highest to lowest total fee (GasPrice × Gas).
	GetTopN(n int) []*types.GTransaction
	Exec(method string, params []any) any
	QueueTransaction(tx *types.GTransaction)
	Register(observer observer.Observer)
	RemoveFromPool(txHash common.Hash) error
	ServiceName() string
	Stop()
}

func InitPool(maxSize int) (TxPool, error) {
	mPool := make(map[common.Hash]*types.GTransaction)
	var p = &Pool{
		memPool:        mPool,
		txHeap:         make(TxHeap, 0, maxSize),
		heapItems:      make(map[common.Hash]*txHeapItem, maxSize),
		maintainTicker: time.NewTicker(1500 * time.Millisecond),
		maxSize:        maxSize,
		stopCh:         make(chan struct{}),
		Status:         0xa,
		observers:      make([]observer.Observer, 0),
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

	gPoolMu.Lock()
	GPool = p
	gPoolMu.Unlock()

	return p, nil
}

// Stop gracefully shuts down the pool service loop.
func (p *Pool) Stop() {
	p.maintainTicker.Stop()
	close(p.stopCh)
}

// removeFromSlice removes an observer from the slice by swapping with the last element.
func removeFromSlice(observerList []observer.Observer, observerToRemove observer.Observer) []observer.Observer {
	observerListLength := len(observerList)
	for i, obs := range observerList {
		if observerToRemove.GetID() == obs.GetID() {
			observerList[observerListLength-1], observerList[i] = observerList[i], observerList[observerListLength-1]
			return observerList[:observerListLength-1]
		}
	}
	return observerList
}

// AddRawTransaction validates and adds a transaction to the pool.
// Returns an error if the transaction is rejected (duplicate, low gas, or pool full).
func (p *Pool) AddRawTransaction(tx *types.GTransaction) error {
	p.mu.Lock()

	if _, exists := p.memPool[tx.Hash()]; exists {
		p.mu.Unlock()
		return errors.New("transaction already in pool")
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
	poolTxAddedTotal.Inc()
	p.calcInfo()
	p.mu.Unlock()

	p.NotifyAll(tx)
	return nil
}

// Clear removes all transactions from the pool (for testing purposes).
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.memPool = make(map[common.Hash]*types.GTransaction)
	p.txHeap = make(TxHeap, 0, p.maxSize)
	p.heapItems = make(map[common.Hash]*txHeapItem, p.maxSize)
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
		Size:             len(hashes),
		Bytes:            txPoolSize,
		Usage:            0,
		UnbroadcastCount: len(hashes),
		MaxMempool:       p.maxSize,
		Mempoolminfee:    0,
		Hashes:           hashes,
		Txs:              cp,
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

// SendTransaction adds a transaction to the pool and returns its hash.
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

// NotifyAll delivers a transaction event to all registered observers.
// Called outside p.mu to prevent deadlocks when observers call back into the pool.
func (p *Pool) NotifyAll(tx *types.GTransaction) {
	p.observersMu.RLock()
	observers := make([]observer.Observer, len(p.observers))
	copy(observers, p.observers)
	p.observersMu.RUnlock()

	for _, obs := range observers {
		obs.Update(tx)
	}
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

// Register adds an observer to receive new transaction events.
func (p *Pool) Register(observer observer.Observer) {
	p.observersMu.Lock()
	defer p.observersMu.Unlock()
	pLogger.Infow("Register new pool observer", "observerID", observer.GetID())
	p.observers = append(p.observers, observer)
}

// RemoveFromPool deletes a transaction from the pool by hash.
func (p *Pool) RemoveFromPool(txHash common.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.memPool[txHash]; !ok {
		return errors.New("transaction not in mempool")
	}
	item := p.heapItems[txHash]
	heap.Remove(&p.txHeap, item.idx) // O(log n) - no full rebuild
	delete(p.heapItems, txHash)
	delete(p.memPool, txHash)
	poolTxRemovedTotal.Inc()
	p.calcInfo()
	return nil
}

// ServiceName returns the service identifier string.
func (p *Pool) ServiceName() string {
	return POOL_SERVICE_NAME
}

// UnRegister removes an observer from the notification list.
func (p *Pool) UnRegister(observer observer.Observer) {
	p.observersMu.Lock()
	defer p.observersMu.Unlock()
	p.observers = removeFromSlice(p.observers, observer)
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

	p.NotifyAll(stored)
}

// Exec dispatches a named method call on the pool.
func (p *Pool) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getInfo":
		return p.GetInfo()
	case "minGas":
		return p.GetMinimalGasValue()
	}
	return nil
}
