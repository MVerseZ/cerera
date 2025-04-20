package miner

import (
	"encoding/json"
	"fmt"
	"math/big"
	"runtime"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/randomx"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
	"github.com/prometheus/client_golang/prometheus"
)

type MinerObserver struct {
	pool.Observer
}

// getID implements pool.Observer.
func (mo MinerObserver) GetID() string {
	return fmt.Sprintf("OBSERVER_MINER_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// update implements pool.Observer.
func (mo MinerObserver) Update(tx *types.GTransaction) {
	fmt.Printf("Miner observer: \r\n\tReceived new transaction with hash %s\r\n", tx.Hash())
}

// PROTOTYPE STRUCTURE
type Miner struct {
	difficulty int64
	status     string

	HeaderTemplate *block.Header

	latest block.Block
	chain  *chain.Chain
	pool   *pool.Pool

	PreparedTransactions []*types.GTransaction

	TxChan    chan types.GTransaction
	BlockChan chan block.Block
	Quit      chan bool
}

var m *Miner
var xvm *randomx.RxVM

func Init() error {
	// init randomx vm
	var flags = []randomx.Flag{randomx.FlagDefault}
	var myCache, _ = randomx.AllocCache(flags...)
	var seed = []byte(big.NewInt(114167270716410).Bytes())
	randomx.InitCache(myCache, seed)
	// var dataset, _ = randomx.AllocDataset(flags...)
	var rxDs, _ = randomx.NewRxDataset(flags...)
	xvm, _ = randomx.NewRxVM(rxDs, flags...)
	// randomx.SetVMDataset(xvm, dataset)
	xvm.CalcHashFirst([]byte("FIRST"))
	return nil
}

func Run() {
	/*
		1. get latest block
		2. build tx tree from mempool
		3. prebuild block
		4. find hash
		5. final build block
	*/
	m := &Miner{
		chain:                chain.GetBlockChain(),
		pool:                 pool.Get(),
		status:               "ALLOC",
		PreparedTransactions: make([]*types.GTransaction, 0),
		TxChan:               make(chan types.GTransaction),
	}
	// fmt.Println("start miner")
	// var f = false
	// for {

	// }

	var minerMetric = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "block_found_counter",
			Help: "Counter of miner",
		},
	)
	prometheus.MustRegister(minerMetric)
	cTime := time.Now().Unix() // time for mine
	// tTime := time.Unix(0, 0).Unix()
	avgTime := time.Unix(0, 0).Unix()
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

	for {
		select {
		// case txs := <-m.pool.Listen:
		// 	for _, v := range txs {
		// 		fmt.Printf("tx in %s\r\n", v.Hash().Hex())
		// 	}
		// 	m.PreparedTransactions = append(m.PreparedTransactions, txs...)
		case incomingTransaction := <-m.TxChan:
			fmt.Println("tx in")
			m.PreparedTransactions = append(m.PreparedTransactions, &incomingTransaction)
		case <-m.BlockChan:
			fmt.Println("block in")
			latest = m.chain.GetLatestBlock() // Обновляем latest при получении нового блока
		case <-m.Quit:
			m.status = "STOP"
			return
		default:
			if m.latest.Hash != common.EmptyHash() {
				m.status = "RUN"
				templateBlock := block.NewBlockWithHeader(m.HeaderTemplate)
				cbTx := types.NewCoinBaseTransaction(
					m.HeaderTemplate.Nonce,                // nonce
					m.chain.GetCurrentChainOwnerAddress(), // current miner
					coinbase.RewardBlock(),                // reward
					100,                                   // gas
					types.FloatToBigInt(1_000_000.0),      // gas price
					[]byte("COINBASE_TX"),                 // data
				)
				txs := m.pool.GetPendingTransactions()

				templateBlock.Transactions = append(templateBlock.Transactions, *cbTx)
				templateBlock.Transactions = append(templateBlock.Transactions, txs...)
				for _, tx := range txs {
					templateBlock.Head.GasUsed += tx.Gas()
				}

				// 1000000 and 1 ~ avg 60 sec searching
				var h, f, sol = xvm.Search(
					templateBlock.ToBytes(),
					m.HeaderTemplate.Difficulty,
					1000000,
					100000,
					templateBlock.GetNonceBytes(),
				)
				if f {
					fmt.Printf(" \tfound!\r\n")
					minerMetric.Inc()
					cTime = time.Now().Unix() - cTime
					avgTime = cTime / int64(templateBlock.Head.Index)

					fmt.Printf(" \tavg time : %d\r\n", avgTime)
					fmt.Printf("\thash length from miner: %d\r\n\thash hex:%x\r\n", len(h), h)
					fmt.Printf("\twith nonce %d and solution %d HEX:%x\r\n", m.HeaderTemplate.Nonce, sol, sol)

					templateBlock.Hash = common.BytesToHash(h)
					templateBlock.SetNonceBytes(sol)
					// fill other fields
					templateBlock.Head.Timestamp = uint64(time.Now().UnixMilli())

					fmt.Printf("\tsize of new block is: %d\r\n", int(unsafe.Sizeof(templateBlock))+int(unsafe.Sizeof(templateBlock.Head)))
					bb, _ := json.Marshal(templateBlock)
					templateBlock.Head.Size = len(bb)
					fmt.Printf("\tfound hash:%s\r\n", common.BytesToHash(h))
					for _, ttx := range txs {
						m.pool.RemoveFromPool(ttx.Hash())
					}
					m.chain.UpdateChain(templateBlock)
					m.status = "FND"
					m.UpdateTemplate()
					latest = m.chain.GetLatestBlock() // Обновляем latest после нахождения блока
				} else {
					// Если поиск прерван
					m.HeaderTemplate.Nonce += 1
					// fmt.Printf("\tchange diff to %d\r\n", m.HeaderTemplate.Difficulty )
				}
				if m.HeaderTemplate.Nonce%100000 == 0 {
					fmt.Printf("\t now: %d\r\n", m.HeaderTemplate.Nonce)
				}
			} else {
				m.status = "NO_BLOCK"
				b := m.chain.GetLatestBlock()
				if m.latest.Hash != b.Hash {
					m.latest = *b
				}
				m.status = "RUN"
			}
		}
	}
}

func (m *Miner) UpdateTemplate() {
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
}

func (m *Miner) Stop() {
	close(m.Quit)
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

// calculates hash by xvm machine
func CalculateHash(b block.Block) common.Hash {
	var bhash = xvm.CalcHash(b.ToBytes())
	return common.BytesToHash(bhash)
}

// func TryToFind(prevBlock *block.Block, chainId *big.Int, difficulty uint64, maxTimes uint64, jump uint32) (block.Block, bool, []byte) {

// 	// block consume txs from pool or event-call from pool to blockchain/miner/validator/other ???
// 	var find = false
// 	var sol []byte = []byte{0x0, 0x0, 0x0, 0x0}
// 	var maxNonce = difficulty
// 	var newHeight = prevBlock.Header().Height + 1
// 	var newIndex = prevBlock.Header().Index + 1
// 	fmt.Printf("curr nonce: %d\r\n", prevBlock.Header().Nonce)
// 	fmt.Printf("max nonce: %d\r\n", maxNonce)
// 	fmt.Printf("target difficulty: %d\r\n", difficulty)

// 	for nonce := prevBlock.Header().Nonce; nonce < maxNonce; nonce++ {
// 		head := &block.Header{
// 			Ctx:        prevBlock.Header().Ctx,
// 			Difficulty: difficulty,
// 			Extra:      []byte(""),
// 			Height:     newHeight,
// 			Index:      newIndex,
// 			Timestamp:  uint64(time.Now().UnixMilli()),
// 			ChainId:    chainId,
// 			PrevHash:   prevBlock.GetHash(),
// 			// Node:       bc.currentAddress,
// 			GasLimit: prevBlock.Head.GasLimit, // todo get gas limit dynamically
// 			Nonce:    nonce,
// 		}
// 		var preBlock = block.NewBlockWithHeader(head)
// 		h, f, sol := xvm.Search(preBlock.ToBytes(), difficulty, 1, 1, preBlock.GetNonceBytes())
// 		if f {
// 			preBlock.Hash = common.BytesToHash(h)
// 			return *preBlock, f, sol
// 		}
// 	}
// 	return block.EmptyBlock, find, sol
// }

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
