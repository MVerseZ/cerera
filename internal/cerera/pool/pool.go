package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/common"

	"github.com/cerera/internal/cerera/types"
)

var p Pool

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
	minGas         uint64
	memPool        map[common.Hash]types.GTransaction
	maintainTicker *time.Ticker

	Status   byte
	Prepared []*types.GTransaction
	Executed []types.GTransaction

	observers []Observer

	mu sync.Mutex
}

func SendTransaction(tx types.GTransaction) (common.Hash, error) {
	if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
		p.memPool[tx.Hash()] = tx
		// p.memPool = append(p.memPool, tx)
		// network.BroadcastTx(tx)
	}
	fmt.Println(p.memPool)
	return tx.Hash(), nil
}

func InitPool(minGas uint64, maxSize int) (*Pool, error) {
	mPool := make(map[common.Hash]types.GTransaction)
	p = Pool{
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

		observers: make([]Observer, 0),
	}
	fmt.Printf("Init pool with parameters: \r\n\t MIN_GAS:%d\r\n\tMAX_SIZE:%d\r\n", p.minGas, p.maxSize)

	go p.PoolServiceLoop()
	return &p, nil
}

func (p *Pool) AddRawTransaction(tx *types.GTransaction) {

	// fmt.Printf("Catch tx with value: %s\r\n", tx.Value())
	var sLock = p.mu.TryLock()
	if sLock {
		if len(p.memPool) < p.maxSize && p.minGas <= tx.Gas() {
			p.memPool[tx.Hash()] = *tx
			p.Prepared = append(p.Prepared, tx)
			// p.memPool = append(p.memPool, *tx)
			// network.BroadcastTx(tx)
		}
	}
	p.mu.Unlock()
	p.NotifyAll(tx)
	// p.Listen <- []*types.GTransaction{tx}
	// p.DataChannel <- tx.Bytes()
	// fmt.Println(len(p.memPool))
}

func (p *Pool) GetInfo() MemPoolInfo {
	var txPoolSize = 0

	var usage uintptr
	var hashes = make([]common.Hash, 0)
	var cp = make([]types.GTransaction, 0)
	// p.mu.Lock()
	// defer p.mu.Unlock()
	for _, k := range p.memPool {
		var txSize = k.Size()
		txPoolSize += int(txSize)
		var txBts = unsafe.Sizeof(k)
		usage += txBts
		hashes = append(hashes, k.Hash())
		cp = append(cp, k)
	}
	// var txPoolSizeDiskUsage = (uint64)(unsafe.Sizeof(ch.pool))
	var result = MemPoolInfo{
		Size:             len(hashes),
		Bytes:            txPoolSize,
		Usage:            usage,
		UnbroadCastCount: txPoolSize,
		MaxMempool:       p.maxSize,
		Mempoolminfee:    int(p.minGas),
		Hashes:           hashes,
		Txs:              cp,
	}
	return result
}

func (p *Pool) GetRawMemPool() []interface{} {
	var result []interface{}

	for i := range p.memPool {
		var tx = p.memPool[i]
		result = append(result, tx.Hash())
	}
	return result
}

func (p *Pool) GetPendingTransactions() []types.GTransaction {
	// p.mu.Lock()
	// defer p.mu.Unlock()
	result := make([]types.GTransaction, 0)
	for _, k := range p.memPool {
		result = append(result, k)
	}
	return result
}

func (p *Pool) GetTransaction(transactionHash common.Hash) *types.GTransaction {
	for _, tx := range p.memPool {
		if tx.Hash() == transactionHash {
			return &tx
		}
	}
	return nil
}

func (p *Pool) UpdateTx(newTx types.GTransaction) {
	for _, tx := range p.memPool {
		if tx.Hash().String() == newTx.Hash().String() {
			fmt.Printf("Replace sign tx in pool: %s signed:%t\r\n", newTx.Hash(), newTx.IsSigned())
			p.AddRawTransaction(&newTx)
		}
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
	errc <- nil
}

func (p *Pool) RemoveFromPool(txHash common.Hash) error {

	_, ok := p.memPool[txHash]
	if !ok {
		return errors.New("no in mempool")
	}
	// fmt.Printf("Deleted: %s\r\n", tx.Hash())
	delete(p.memPool, txHash)
	return nil
}

func (p *Pool) Clear() {
	p.memPool = nil
	p.memPool = make(map[common.Hash]types.GTransaction)
}

func (p *Pool) GetMinimalGasValue() uint64 {
	return p.minGas
}

func (p *Pool) Register(observer Observer) {
	fmt.Printf("Register new pool observer: %s\r\n", observer.GetID())
	p.observers = append(p.observers, observer)
}

func (p *Pool) UnRegister(observer Observer) {
	p.observers = removeFromslice(p.observers, observer)
}

func removeFromslice(observerList []Observer, observerToRemove Observer) []Observer {
	observerListLength := len(observerList)
	for i, observer := range observerList {
		if observerToRemove.GetID() == observer.GetID() {
			observerList[observerListLength-1], observerList[i] = observerList[i], observerList[observerListLength-1]
			return observerList[:observerListLength-1]
		}
	}
	return observerList
}

func (p *Pool) NotifyAll(tx *types.GTransaction) {
	for _, observer := range p.observers {
		observer.Update(tx)
	}
}

func Get() *Pool {
	return &p
}
