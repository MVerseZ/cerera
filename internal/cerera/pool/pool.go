package pool

import (
	"errors"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/observer"
	"github.com/cerera/internal/cerera/types"
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
)

func init() {
	prometheus.MustRegister(
		poolSize,
		poolBytes,
		poolTxAddedTotal,
		poolTxRemovedTotal,
		poolTxRejectedTotal,
		poolMaxSize,
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

	maxSize        int
	minGas         float64
	memPool        map[common.Hash]types.GTransaction
	maintainTicker *time.Ticker

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
	GetInfo() MemPoolInfo
	GetRawMemPool() []interface{}
	GetPendingTransactions() []types.GTransaction
	Exec(method string, params []interface{}) interface{}
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

	// fmt.Printf("Catch tx with value: %s\r\n", tx.Value())

	// if p.Info.Bytes+int(tx.Size()) > p.maxSize {
	// 	return
	// } else {
	var sLock = p.mu.TryLock()
	if sLock {
		defer p.mu.Unlock()
		if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
			p.memPool[tx.Hash()] = *tx
			p.Prepared = append(p.Prepared, tx)
			p.NotifyAll(tx)
			// p.memPool = append(p.memPool, *tx)
			// network.BroadcastTx(tx)
			p.Info.Bytes += int(tx.Size())
			p.Info.Size++
			p.Info.Usage += uintptr(tx.Size())
			p.Info.Hashes = append(p.Info.Hashes, tx.Hash())
			p.Info.Txs = append(p.Info.Txs, *tx)
			p.Info.UnbroadCastCount++
			poolTxAddedTotal.Inc()
			poolSize.Set(float64(p.Info.Size))
			poolBytes.Set(float64(p.Info.Bytes))
		} else {
			poolTxRejectedTotal.Inc()
		}
	}
	// }
	// p.Listen <- []*types.GTransaction{tx}
	// p.DataChannel <- tx.Bytes()
	// fmt.Println(len(p.memPool))
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
	for _, tx := range p.memPool {
		if tx.Hash() == transactionHash {
			return &tx
		}
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
	// fmt.Printf("Deleted: %s\r\n", tx.Hash())
	delete(p.memPool, txHash)
	poolTxRemovedTotal.Inc()
	// Update metrics
	p.calcInfo()
	return nil
}

func (p *Pool) SendTransaction(tx types.GTransaction) (common.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
		p.memPool[tx.Hash()] = tx
		// p.memPool = append(p.memPool, tx)
		// network.BroadcastTx(tx)
	} else {
		poolTxRejectedTotal.Inc()
		pLogger.Warnw("Transaction rejected",
			"hash", tx.Hash(),
			"poolSize", len(p.memPool),
			"maxSize", p.maxSize,
			"gas", tx.Gas(),
			"minGas", p.minGas,
		)
	}
	return tx.Hash(), nil
}

func (p *Pool) ServiceName() string {
	return POOL_SERVICE_NAME
}

func (p *Pool) UnRegister(observer observer.Observer) {
	p.observers = removeFromslice(p.observers, observer)
}

func (p *Pool) UpdateTx(newTx types.GTransaction) {
	for _, tx := range p.memPool {
		if tx.Hash().String() == newTx.Hash().String() {
			pLogger.Infow("Replace sign tx in pool",
				"hash", newTx.Hash(),
				"signed", newTx.IsSigned(),
			)
			p.AddRawTransaction(&newTx)
		}
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
