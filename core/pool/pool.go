package pool

import (
	"errors"
	"sync"
	"time"

	"github.com/cerera/core/types"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/observer"
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

type Pool struct {
	Funnel      chan []*types.GTransaction // input funnel
	Listen      chan []*types.GTransaction // outbound channel
	DataChannel chan []byte                // ws channel

	NewTxEventChannel chan *types.GTransaction
	maxSize           int
	minGas            float64
	memPool           map[common.Hash]types.GTransaction
	maintainTicker    *time.Ticker

	Status   byte
	Prepared []*types.GTransaction
	Executed []types.GTransaction

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

func InitPool(minGas float64, maxSize int) (TxPool, error) {
	mPool := make(map[common.Hash]types.GTransaction)
	var p = &Pool{
		memPool:        mPool,
		maintainTicker: time.NewTicker(1500 * time.Millisecond),
		maxSize:        maxSize,
		minGas:         minGas,

		Prepared: make([]*types.GTransaction, 0),
		Executed: make([]types.GTransaction, 0),

		Funnel:      make(chan []*types.GTransaction),
		Listen:      make(chan []*types.GTransaction),
		DataChannel: make(chan []byte),
		Status:      0xa,

		observers: make([]observer.Observer, 0),
	}
	pLogger.Infow("Init pool",
		"minGas", p.minGas,
		"minGasBI", types.FloatToBigInt(p.minGas),
		"maxSize", p.maxSize,
	)
	p.Info = MemPoolInfo{
		Size:             0,
		Bytes:            0,
		Usage:            0,
		MaxMempool:       p.maxSize,
		Mempoolminfee:    int(p.minGas),
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

	if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
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

func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.memPool = nil
	p.memPool = make(map[common.Hash]types.GTransaction)
	p.calcInfo()
}

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
		Mempoolminfee:    int(p.minGas),
		Hashes:           hashes,
		Txs:              cp,
	}

	p.Info = result
	poolSize.Set(float64(result.Size))
	poolBytes.Set(float64(result.Bytes))
}

func (p *Pool) GetInfo() MemPoolInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calcInfo()
	return p.Info
}

func (p *Pool) GetMinimalGasValue() float64 {
	return p.minGas
}

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

func (p *Pool) GetRawMemPool() []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	var result []interface{}
	for i := range p.memPool {
		var tx = p.memPool[i]
		result = append(result, tx.Hash())
	}

	return result
}

func (p *Pool) QueueTransaction(tx *types.GTransaction) {
	p.AddRawTransaction(tx)
}

func (p *Pool) GetTransaction(transactionHash common.Hash) *types.GTransaction {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Use direct access by hash instead of iteration
	if tx, ok := p.memPool[transactionHash]; ok {
		return &tx
	}
	return nil
}

func (p *Pool) NotifyAll(tx *types.GTransaction) {
	for _, observer := range p.observers {
		observer.Update(tx)
	}
}

func (p *Pool) PoolServiceLoop() {
	var errc chan error
	for errc == nil {
		select {
		case <-p.maintainTicker.C:
			// fmt.Printf("Pool maintain loop\r\n")
			// p.mu.Lock()
			// if p.Prepared == nil {
			// 	p.Prepared = make([]*types.GTransaction, 0)
			// }
			// for _, tx := range p.memPool {
			// 	var r, s, v = tx.RawSignatureValues()
			// 	// fmt.Printf("%s to %s - signed %t \r\n", tx.Hash(), tx.To(), tx.IsSigned())
			// 	// if tx signed - add it to block
			// 	if big.NewInt(0).Cmp(r) != 0 && big.NewInt(0).Cmp(s) != 0 && big.NewInt(0).Cmp(v) != 0 {
			// 		p.Prepared = append(p.Prepared, &tx)
			// 	}
			// 	for _, preparedTx := range p.Prepared {
			// 		delete(p.memPool, preparedTx.Hash())
			// 	}
			// }
			// p.mu.Unlock()
			// fmt.Printf("Prepared for block txs count: %d\r\n", len(p.Prepared))
			// fmt.Printf("Executed txs count: %d\r\n", len(p.Executed))
			// fmt.Printf("Current pool size: %d\r\n", len(p.memPool))
		case txs := <-p.Funnel:
			// NEED TO REWRITE 13/04/25 gnupunk
			// fmt.Printf("Funnel data arrive\r\n")
			var txPoolSize = 0
			for _, k := range p.memPool {
				var txSize = k.Size()
				txPoolSize += int(txSize)
			}
			for _, tx := range txs {
				if txPoolSize+int(tx.Size()) < p.maxSize {
					p.AddRawTransaction(tx)
				} else {
					break
				}
			}
			// go func() { p.Listen <- txs }()

			// case newBlock := <-gigea.E.BlockPipe:
			// 	fmt.Println("POOL")
			// 	for _, tx := range newBlock.Transactions {
			// 		fmt.Println(tx.Hash())
			// 		delete(p.memPool, tx.Hash())
			// 	}
		}
	}
}

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

func (p *Pool) SendTransaction(tx types.GTransaction) (common.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
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
			"minGas", p.minGas,
		)
		return tx.Hash(), errors.New("transaction rejected: pool full or gas too low")
	}
	return tx.Hash(), nil
}

func (p *Pool) ServiceName() string {
	return POOL_SERVICE_NAME
}

func (p *Pool) UnRegister(observer observer.Observer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.observers = removeFromslice(p.observers, observer)
}

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

func (p *Pool) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getInfo":
		return p.GetInfo()
	case "minGas":
		return p.GetMinimalGasValue()
	}
	return nil
}
