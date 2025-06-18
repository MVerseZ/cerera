package miner

import (
	"encoding/json"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/randomx"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea/gigea"
	"github.com/prometheus/client_golang/prometheus"
)

type CMinerMetric struct {
	C              int //ticks
	FoundBlockCnt  int //found
	TxsApprovedCnt int //txs
	AllOpCnt       int //all
	mu             sync.Mutex
}

func (m *CMinerMetric) Update() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.C += 1
	m.AllOpCnt += 1
}
func (m *CMinerMetric) UpdateT() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.C = 0
	m.TxsApprovedCnt += 1
}
func (m *CMinerMetric) UpdateF() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AllOpCnt += 1
	m.FoundBlockCnt += 1
}

type Miner struct {
	// difficulty int64
	status       string
	minerMetrics CMinerMetric

	HeaderTemplate *block.Header

	// latest block.Block
	chain *chain.Chain
	pool  *pool.Pool

	PreparedTransactions []*types.GTransaction

	TxChan    chan *types.GTransaction
	BlockChan chan block.Block
	Quit      chan bool
}

var m *Miner
var xvm *randomx.RxVM

type MinerObserver struct {
	pool.Observer
	ch chan *types.GTransaction
}

// getID implements pool.Observer.
func (mo MinerObserver) GetID() string {
	return fmt.Sprintf("OBSERVER_MINER_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// update implements pool.Observer.
func (mo MinerObserver) Update(tx *types.GTransaction) {
	fmt.Printf("Miner observer: \r\n\tReceived new transaction with hash %s\r\n", tx.Hash())
	go func() { m.TxChan <- tx }()
	// mo.ch <- tx
}

func Init() error {
	// init randomx vm
	var flags = []randomx.Flag{randomx.FlagDefault, randomx.FlagArgon2AVX2}
	var myCache, _ = randomx.AllocCache(flags...)
	var seed = []byte(big.NewInt(114167270716410).Bytes())
	randomx.InitCache(myCache, seed)
	// var dataset, _ = randomx.AllocDataset(flags...)
	var rxDs, _ = randomx.NewRxDataset(flags...)
	xvm, _ = randomx.NewRxVM(rxDs, flags...)
	// randomx.SetVMDataset(xvm, dataset)
	xvm.CalcHashFirst([]byte("FIRST"))
	fmt.Printf("Miner init done with params:\t\r\n, %b\r\n", myCache)

	initMiner()
	return nil
}

func initMiner() {
	m = &Miner{
		chain:                chain.GetBlockChain(),
		pool:                 pool.Get(),
		status:               "ALLOC",
		PreparedTransactions: make([]*types.GTransaction, 0),
		TxChan:               make(chan *types.GTransaction),
		minerMetrics: CMinerMetric{
			C:              0,
			TxsApprovedCnt: 0,
			FoundBlockCnt:  0,
			AllOpCnt:       0,
		},
	}
}

func Run() {
	/*
		1. get latest block
		2. build tx tree from mempool
		3. prebuild block
		4. find hash
		5. final build block
	*/

	var minerMetric = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "block_found_counter",
			Help: "Counter of miner",
		},
	)
	prometheus.MustRegister(minerMetric)
	cTime := time.Now().Unix() // time for mine
	// tTime := time.Unix(0, 0).Unix()
	// avgTime := time.Unix(0, 0).Unix()
	latest := m.chain.GetLatestBlock()
	m.HeaderTemplate = &block.Header{
		Ctx:        17,
		Difficulty: latest.Head.Difficulty,
		Extra:      [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Height:     latest.Head.Height + 1,
		Index:      latest.Head.Index + 1,
		GasLimit:   250000,
		GasUsed:    1,
		ChainId:    m.chain.GetChainId(),
		Node:       m.chain.GetCurrentChainOwnerAddress(),
		Size:       0,
		V:          [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x1, 0x0},
		Nonce:      latest.Head.Nonce,
		PrevHash:   latest.Hash,
		Root:       latest.Head.Root,
	}
	minerObserver := MinerObserver{}
	m.pool.Register(minerObserver)
	m.status = "PREPARED"

	for {
		select {
		// case txs := <-m.pool.Listen:
		// 	for _, v := range txs {
		// 		fmt.Printf("tx in %s\r\n", v.Hash().Hex())
		// 	}
		// 	m.PreparedTransactions = append(m.PreparedTransactions, txs...)
		// case incomingTransaction := <-m.TxChan:
		// 	fmt.Println("tx in")
		// 	m.PreparedTransactions = append(m.PreparedTransactions, &incomingTransaction)
		// case <-m.BlockChan:
		// 	fmt.Println("block in")
		// 	if m.status == "FND" {
		// 		m.SetStatus("RUN")
		// 	}
		// 	latest = m.chain.GetLatestBlock() // Обновляем latest при получении нового блока
		case <-m.chain.OutBoundEvents:
			m.UpdateTemplate()
			m.SetStatus("RUN")
		case incomingTransaction := <-m.TxChan:
			m.SetStatus("REFRESH")
			m.minerMetrics.UpdateT()
			fmt.Printf("\tTx from miner observer: %s\r\n", incomingTransaction.Hash())
			m.PreparedTransactions = append(m.PreparedTransactions, incomingTransaction)
		case <-m.Quit:
			m.SetStatus("STOP")
			return
		default:
			// fmt.Printf("Miner status: %s\r\n\tC-%d || T-%d || A-%d || F-%d\r\n", m.status, m.minerMetrics.C, m.minerMetrics.T, m.minerMetrics.A, m.minerMetrics.F)
			if m.status == "FND" {
				m.UpdateTemplate()
			}
			if m.status == "PREPARED" {
				m.SetStatus("RUN")
			}
			if m.status == "REFRESH" {
				m.SetStatus("RUN")
			}
			if m.status == "RUN" {
				templateBlock := block.NewBlockWithHeader(m.HeaderTemplate)
				cbTx := types.NewCoinBaseTransaction(
					m.HeaderTemplate.Nonce,                // nonce
					m.chain.GetCurrentChainOwnerAddress(), // current miner
					coinbase.RewardBlock(),                // reward
					100,                                   // gas
					types.FloatToBigInt(1_000_000.0),      // gas price
					[]byte("COINBASE_TX"),                 // data
				)
				// txs := m.pool.GetPendingTransactions()

				templateBlock.Transactions = append(templateBlock.Transactions, *cbTx)
				// templateBlock.Transactions = append(templateBlock.Transactions, txs...)
				for _, tx := range m.PreparedTransactions {
					templateBlock.Transactions = append(templateBlock.Transactions, *tx)
					templateBlock.Head.GasUsed += tx.Gas()
				}
				templateBlock.Head.GasUsed += cbTx.Gas()

				// 1000000 and 1 ~ avg 60 sec searching
				var blockBytes, difficulty, maxTimes, jump, nonceBytes = templateBlock.ToBytes(), m.HeaderTemplate.Difficulty, uint64(100000000000), uint32(100000), templateBlock.GetNonceBytes()
				fmt.Printf(" \tsearching block... \r\n")
				fmt.Printf(" \t\twith params:\r\n\t\tlen bytes: %d\r\n\t\tdifficulty: %d\r\n\t\tnonce: %x\r\n\t\theight: %d\r\n", len(templateBlock.ToBytes()), m.HeaderTemplate.Difficulty, templateBlock.GetNonceBytes(), templateBlock.Head.Height)
				var h, f, sol = xvm.Search(blockBytes, difficulty, maxTimes, jump, nonceBytes)

				if f {
					m.minerMetrics.UpdateF()
					minerMetric.Inc()
					cTime = time.Now().Unix() - cTime

					// avgTime = cTime / int64(templateBlock.Head.Index)

					// fmt.Printf(" \tFound block! \r\n")
					// fmt.Printf(" \tavg time : %d\r\n", avgTime)
					// fmt.Printf("\thash length from miner: %d\r\n\thash hex:%x\r\n", len(h), h)
					// fmt.Printf("\twith nonce %d and solution %d HEX:%x\r\n", m.HeaderTemplate.Nonce, sol, sol)

					templateBlock.Hash = common.BytesToHash(h)
					templateBlock.SetNonceBytes(sol)
					// fill other fields
					templateBlock.Head.Timestamp = uint64(time.Now().UnixMilli())

					// fmt.Printf("\tsize of new block is: %d\r\n", int(unsafe.Sizeof(templateBlock))+int(unsafe.Sizeof(templateBlock.Head)))
					bb, _ := json.Marshal(templateBlock)
					templateBlock.Head.Size = len(bb)
					fmt.Printf("\tfound hash:%s\r\n", common.BytesToHash(h))
					m.SetStatus("FND")
					for _, ttx := range templateBlock.Transactions {
						m.pool.RemoveFromPool(ttx.Hash())
					}
					// m.chain.UpdateChain(templateBlock)
					// go func() {
					gigea.E.BlockFunnel <- templateBlock

					// }()
					// latest = m.chain.GetLatestBlock() // Обновляем latest после нахождения блока
				} else {
					// Если поиск прерван
					m.minerMetrics.Update()
					m.HeaderTemplate.Nonce += 1
					m.status = "PREPARED"
					// fmt.Printf("Miner status: %s\r\n\tC-%d || T-%d || A-%d || F-%d\r\n", m.status, m.minerMetrics.C, m.minerMetrics.T, m.minerMetrics.A, m.minerMetrics.F)
					// time.Sleep(1 * time.Nanosecond)
					// fmt.Printf("\tchange diff to %d\r\n", m.HeaderTemplate.Difficulty )
				}
				continue
			}
			time.Sleep(1 * time.Millisecond)

		}
	}
}

func (m *Miner) UpdateTemplate() {
	latest := m.chain.GetLatestBlock()
	if latest.Head.Height != m.HeaderTemplate.Height {
		m.HeaderTemplate = &block.Header{
			Ctx:        17,
			Difficulty: latest.Head.Difficulty,
			Extra:      [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			Height:     latest.Head.Height + 1,
			Index:      latest.Head.Index + 1,
			GasLimit:   250000,
			GasUsed:    1,
			ChainId:    m.chain.GetChainId(),
			Node:       m.chain.GetCurrentChainOwnerAddress(),
			Size:       0,
			V:          [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x1, 0x0},
			Nonce:      latest.Head.Nonce,
			PrevHash:   latest.Hash,
			Root:       latest.Head.Root,
		}
		m.PreparedTransactions = make([]*types.GTransaction, 0)
	}
}

func (m *Miner) Stop() {
	close(m.Quit)
}

func (m *Miner) SetStatus(status string) {
	fmt.Printf("Miner status old: %s::C-%d || T-%d || A-%d || F-%d\r\n", m.status, m.minerMetrics.C, m.minerMetrics.TxsApprovedCnt, m.minerMetrics.AllOpCnt, m.minerMetrics.FoundBlockCnt)
	m.status = status
	fmt.Printf("Miner status new: %s::C-%d || T-%d || A-%d || F-%d\r\n", m.status, m.minerMetrics.C, m.minerMetrics.TxsApprovedCnt, m.minerMetrics.AllOpCnt, m.minerMetrics.FoundBlockCnt)
}

func Start(latest *block.Block, chId *big.Int, difficulty uint64) {
	// var newBlock, f, _ = TryToFind(latest, chId, difficulty, 1000000, 1)
	// if f {
	// 	fmt.Println("found!")
	// 	fmt.Println(newBlock.Hash)
	// }
}

func Stop() {

}

func GetMiner() interface{} {
	return m
}

// calculates hash by xvm machine
func CalculateHash(b block.Block) common.Hash {
	var bhash = xvm.CalcHash(b.ToBytes())
	return common.BytesToHash(bhash)
}

func CalculateBlockHash(b block.Block) {

}

func MineBlock(latest *block.Block, addr types.Address) {
	head := &block.Header{
		Ctx:        latest.Header().Ctx,
		Difficulty: latest.Header().Difficulty,
		Extra:      [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Height:     latest.Header().Height + 1,
		Index:      latest.Header().Index + 1,
		Timestamp:  uint64(time.Now().UnixMilli()),
		ChainId:    latest.Header().ChainId,
		// PrevHash:      bc.info.Latest,
		Node:     addr,
		Root:     latest.Header().Root,
		GasLimit: latest.Head.GasLimit, // todo get gas limit dynamically
	}
	fmt.Println(head)
}

// https://github.com/ethereum/go-ethereum/blob/master/miner/payload_building.go#L208
