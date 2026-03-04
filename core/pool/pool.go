package pool

import (
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
	UnbroadCastCount int     // local pool transactions
	Hashes           []common.Hash
	Txs              []types.GTransaction
}

// for simplification of mining, pool sort transactions by gas*gasPrice with coefficient of payload
//
// @author gnupunk 28.02.2026
type Pool struct {
	Listen      chan []*types.GTransaction // outbound channel
	DataChannel chan []byte                // ws channel

	NewTxEventChannel chan *types.GTransaction
	maxSize           int
	memPool           map[common.Hash]types.GTransaction
	maintainTicker    *time.Ticker

	Status   byte
	Prepared []*types.GTransaction

	observers []observer.Observer

	Info MemPoolInfo

	mu sync.Mutex
}

var GPool TxPool

func Get() TxPool {
	return GPool
}

type TxPool interface {
	AddRawTransaction(tx *types.GTransaction)
	GetInfo() MemPoolInfo
	GetRawMemPool() []any
	GetPendingTransactions() []types.GTransaction
	GetTransaction(transactionHash common.Hash) *types.GTransaction
	Exec(method string, params []any) any
	QueueTransaction(tx *types.GTransaction)
	Register(observer observer.Observer)
	RemoveFromPool(txHash common.Hash) error
	ServiceName() string
}

func InitPool(maxSize int) (TxPool, error) {
	mPool := make(map[common.Hash]types.GTransaction)
	var p = &Pool{
		memPool:        mPool,
		maintainTicker: time.NewTicker(1500 * time.Millisecond),
		maxSize:        maxSize,

		Prepared: make([]*types.GTransaction, 0),

		Listen:      make(chan []*types.GTransaction),
		DataChannel: make(chan []byte),
		Status:      0xa,

		observers: make([]observer.Observer, 0),
	}
	pLogger.Infow("Init pool",
		"maxSize", p.maxSize,
	)
	p.Info = MemPoolInfo{
		Size:             0,
		Bytes:            0,
		Usage:            0,
		MaxMempool:       p.maxSize,
		UnbroadCastCount: 0,
		Hashes:           make([]common.Hash, 0),
		Txs:              make([]types.GTransaction, 0),
	}

	poolMaxSize.Set(float64(p.maxSize))
	poolSize.Set(0)
	poolBytes.Set(0)

	go p.PoolServiceLoop()
	GPool = p
	return GPool, nil
}

// remove observer from pool observers list, common method
func removeFromslice(observerList []observer.Observer, observerToRemove observer.Observer) []observer.Observer {
	observerListLength := len(observerList)
	for i, observer := range observerList {
		if observerToRemove.GetID() == observer.GetID() {
			observerList[observerListLength-1], observerList[i] = observerList[i], observerList[observerListLength-1]
			return observerList[:observerListLength-1]
		}
	}
	return observerList
}

// add transaction to pool
// tx validate by gas, size, etc..

func (p *Pool) AddRawTransaction(tx *types.GTransaction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if transaction already exists in memPool
	if _, exists := p.memPool[tx.Hash()]; exists {
		// Transaction already exists, skip or update
		return
	}
	// Check if transaction already exists in Prepared to avoid duplicates
	for _, preparedTx := range p.Prepared {
		if preparedTx.Hash() == tx.Hash() {
			// Transaction already in Prepared, skip
			return
		}
	}
	// For legacy transfers tx.Data() is a text message, not bytecode.
	// Use canonical minimum transfer gas; for contract transactions use PreCompile.
	var minGas uint64
	if tx.Type() == types.LegacyTxType {
		minGas = pallada.MinTransferGas
	} else {
		var err error
		minGas, err = pallada.NewVM(tx.Data(), nil).PreCompile(tx.Data())
		if err != nil {
			pLogger.Errorw("Error precompile transaction", "error", err)
			return
		}
	}
	if len(p.memPool) < p.maxSize && minGas <= tx.Gas() {
		p.memPool[tx.Hash()] = *tx
		p.Prepared = append(p.Prepared, tx)
		p.NotifyAll(tx)
		poolTxAddedTotal.Inc()
		// Recalculate Info instead of manual update to prevent memory leaks
		p.calcInfo()
	} else {
		poolTxRejectedTotal.Inc()
	}
}

// clear pool (for testing purposes)
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.memPool = nil
	p.memPool = make(map[common.Hash]types.GTransaction)
	p.calcInfo()
}

// calculate pool info (for internal use)
func (p *Pool) calcInfo() {

	var txPoolSize = 0
	var hashes = make([]common.Hash, 0)
	var cp = make([]types.GTransaction, 0)

	for _, k := range p.memPool {
		var txSize = k.Size()
		txPoolSize += int(txSize)
		hashes = append(hashes, k.Hash())
		cp = append(cp, k)
	}

	var result = MemPoolInfo{
		Size:             len(hashes),
		Bytes:            txPoolSize,
		Usage:            0, // Можно рассчитать если нужно
		UnbroadCastCount: len(hashes),
		MaxMempool:       p.maxSize,
		Mempoolminfee:    0, //int(p.minGas),
		Hashes:           hashes,
		Txs:              cp,
	}

	p.Info = result
	poolSize.Set(float64(result.Size))
	poolBytes.Set(float64(result.Bytes))
}

// get pool info (for internal use)
func (p *Pool) GetInfo() MemPoolInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calcInfo()
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
		0x55,       // SSTORE   (save new receiver balance)
		0x00,       // STOP
	}
	gasUnits, err := pallada.NewVM(transferBytecode, nil).PreCompile(transferBytecode)
	if err != nil {
		pLogger.Errorw("Error precompile transfer bytecode", "error", err)
		return 0
	}
	// gasUnits DUST → CER
	return float64(gasUnits) / 1_000_000
}

// get pending transactions (for internal use)
func (p *Pool) GetPendingTransactions() []types.GTransaction {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		poolGetPendingDurationSeconds.Observe(duration)
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]types.GTransaction, 0)
	for _, k := range p.memPool {
		result = append(result, k)
	}
	return result
}

// get raw mempool (hashes of transactions in mempool)
func (p *Pool) GetRawMemPool() []any {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		poolGetPendingDurationSeconds.Observe(duration)
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	var result []any
	for i := range p.memPool {
		var tx = p.memPool[i]
		result = append(result, tx.Hash())
	}

	return result
}

// add transaction to pool queue (wrapper for AddRawTransaction)
func (p *Pool) QueueTransaction(tx *types.GTransaction) {
	p.AddRawTransaction(tx)
}

// get transaction by hash (for internal use)
func (p *Pool) GetTransaction(transactionHash common.Hash) *types.GTransaction {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Use direct access by hash instead of iteration
	if tx, ok := p.memPool[transactionHash]; ok {
		return &tx
	}
	return nil
}

// notify all observers (for internal use)
func (p *Pool) NotifyAll(tx *types.GTransaction) {
	for _, observer := range p.observers {
		observer.Update(tx)
	}
}

// pool service loop (for internal use)
func (p *Pool) PoolServiceLoop() {
	var errc chan error
	for errc == nil {
		select {
		case <-p.maintainTicker.C:
			// fmt.Printf("Pool maintain loop\r\n")
			// p.mu.Lock()
		}
	}
}

// register new observer (for internal use)
func (p *Pool) Register(observer observer.Observer) {
	pLogger.Infow("Register new pool observer", "observerID", observer.GetID())
	p.observers = append(p.observers, observer)
}

func (p *Pool) RemoveFromPool(txHash common.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.memPool[txHash]
	if !ok {
		return errors.New("no in mempool")
	}
	delete(p.memPool, txHash)
	// Исключаем также из Prepared, чтобы не было рассинхрона и утечки
	for i := 0; i < len(p.Prepared); i++ {
		if p.Prepared[i].Hash() == txHash {
			p.Prepared = append(p.Prepared[:i], p.Prepared[i+1:]...)
			break
		}
	}
	poolTxRemovedTotal.Inc()
	p.calcInfo()
	return nil
}

// send transaction to pool (for internal use)
func (p *Pool) SendTransaction(tx types.GTransaction) (common.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.memPool) < p.maxSize { //&& p.minGas <= tx.Gas() {
		p.memPool[tx.Hash()] = tx
		poolTxAddedTotal.Inc()
		// Update metrics
		p.calcInfo()
	} else {
		poolTxRejectedTotal.Inc()
		pLogger.Warnw("Transaction rejected",
			"hash", tx.Hash(),
			"poolSize", len(p.memPool),
			"maxSize", p.maxSize,
			"gas", tx.Gas(),
			//"minGas", p.minGas,
		)
		return tx.Hash(), errors.New("transaction rejected: pool full or gas too low")
	}
	return tx.Hash(), nil
}

// get service name (for internal use)
func (p *Pool) ServiceName() string {
	return POOL_SERVICE_NAME
}

// unregister observer (for internal use)
func (p *Pool) UnRegister(observer observer.Observer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.observers = removeFromslice(p.observers, observer)
}

// update transaction in pool, change gas or message (NOT VALUE, if we send 1,1 -> we will receive 1,1).
func (p *Pool) UpdateTx(newTx types.GTransaction) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Use direct access by hash instead of iteration
	if _, exists := p.memPool[newTx.Hash()]; exists {
		pLogger.Infow("Replace sign tx in pool",
			"hash", newTx.Hash(),
			"signed", newTx.IsSigned(),
		)
		p.memPool[newTx.Hash()] = newTx
		// Update Prepared list if transaction is there
		for i, preparedTx := range p.Prepared {
			if preparedTx.Hash() == newTx.Hash() {
				p.Prepared[i] = &newTx
				break
			}
		}
		p.calcInfo()
		// Notify observers
		p.NotifyAll(&newTx)
	}
}

// execute method (for internal use)
func (p *Pool) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getInfo":
		return p.GetInfo()
	case "minGas":
		return p.GetMinimalGasValue()
	}
	return nil
}
