package chain

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/miner"

	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/gigea/gigea"
)

type BlockChainStatus struct {
	Total     int         `json:"total,omitempty"`
	ChainWork int         `json:"chainWork,omitempty"`
	Latest    common.Hash `json:"latest,omitempty"`
	Size      int64       `json:"size,omitempty"`
}

type Chain struct {
	autoGen        bool
	chainId        *big.Int
	chainWork      *big.Int
	currentAddress types.Address
	currentBlock   *block.Block
	// rootHash       common.Hash

	mu   sync.Mutex
	info BlockChainStatus
	data []block.Block
	t    *trie.MerkleTree

	// tickers
	maintainTicker *time.Ticker
	blockTicker    *time.Ticker
	DataChannel    chan []byte
	Size           int

	Difficulty uint64
}

var (
	bch        Chain
	BLOCKTIMER = time.Duration(30 * time.Second)
)

func GetBlockChain() *Chain {
	return &bch
}

func InitBlockChain(cfg *config.Config) { //Chain {

	var (
		t         *trie.MerkleTree
		chainWork = 0
		total     = 0
	)

	stats := BlockChainStatus{
		Total:     total,
		ChainWork: chainWork,
		Size:      0,
	}

	var err error

	genesisBlock := block.Genesis(cfg.Chain.ChainID)
	genesisBlock.Hash = miner.CalculateHash(&genesisBlock)

	dataBlocks := make([]block.Block, 0)
	var list []trie.Content

	if cfg.IN_MEM {
		dataBlocks = append(dataBlocks, genesisBlock)
	} else {
		if cfg.Chain.Path == "EMPTY" {
			cfg.UpdateChainPath("./chain.dat")
			list = append(list, genesisBlock)
			stats.Total += 1
			stats.ChainWork += genesisBlock.Head.Size
			InitChainVault(genesisBlock)
		}

		switch stat := gigea.C.Status; stat {
		case 1:
			// local mode
			var readBlock, err = SyncVault()
			if err != nil {
				panic(err)
			}

			dataBlocks = append(dataBlocks, readBlock...)
			// validate added blocks
			lastCorrect, errorBlock := ValidateBlocks(dataBlocks)
			if errorBlock != nil {
				log.Printf("ERROR BLOCK! %s\r\n", errorBlock)
			}
			dataBlocks = dataBlocks[:lastCorrect]
			fmt.Printf("Start chain from %d block\r\n", lastCorrect)

		case 2:
			// network mode
			panic("gigea.blockchain.status")
		}
	}

	for _, v := range dataBlocks {
		list = append(list, v)
		stats.Total += 1
		stats.ChainWork += v.Head.Size
		stats.Latest = v.GetHash()
		// 		bc.info.Total = bc.info.Total + 1
		// bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	}
	t, err = trie.NewTree(list)
	if err != nil {
		fmt.Printf("error trie validating: %s\r\n", err)
	}

	t.VerifyTree()

	//	0xb51551C31419695B703aD37a2c04A765AB9A6B4a183041354a6D392ce438Aec47eBb16495E84F18ef492B50f652342dE
	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   &dataBlocks[len(dataBlocks)-1],
		blockTicker:    time.NewTicker(BLOCKTIMER),
		maintainTicker: time.NewTicker(time.Duration(5 * time.Minute)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
		t:              t,
		DataChannel:    make(chan []byte),
		Size:           genesisBlock.Header().Size,
		Difficulty:     genesisBlock.Head.Difficulty,
	}
	// genesisBlock.Head.Node = bch.currentAddress
	// go bch.BlockGenerator()
	go bch.Start()

	// return bch
}

func (bc *Chain) GetInfo() interface{} {
	var totalSize = 0
	for _, b := range bc.data {
		totalSize += b.Header().Size
	}
	bc.info.Size = int64(totalSize)
	bc.info.Total = len(bc.data)
	bc.info.Latest = bc.data[len(bc.data)-1].GetHash()

	return bc.info
}

func (bc *Chain) GetLatestBlock() *block.Block {
	return bc.currentBlock
}

func (bc *Chain) GetBlockHash(number int) common.Hash {
	for _, b := range bc.data {
		if b.Header().Index == uint64(number) {
			return b.GetHash()
		}
	}
	return common.EmptyHash()
}

func (bc *Chain) GetBlockByNumber(number int) *block.Block {
	for _, b := range bc.data {
		if b.Header().Index == uint64(number) {
			return &b
		}
	}
	return &block.Block{}
}

func (bc *Chain) GetBlock(blockHash common.Hash) *block.Block {
	for _, b := range bc.data {
		if b.GetHash().Compare(blockHash) == 0 {
			return &b
		}
	}
	return &block.Block{}
}

func (bc *Chain) GetBlockHeader(blockHash string) *block.Header {
	var bHash = common.HexToHash(blockHash)
	for _, b := range bc.data {
		if b.GetHash().Compare(bHash) == 0 {
			return b.Header()
		}
	}
	return &block.Header{}
}

// func (bc *Chain) BlockGenerator() {
// 	for {
// 		select {
// 		case <-bc.blockTicker.C:
// 			var latest = bc.GetLatestBlock()
// 			if bc.autoGen {
// 				bc.TryAutoGen(latest)
// 			}
// 		case <-bc.maintainTicker.C:
// 			continue
// 		}
// 	}
// }

func (bc *Chain) Start() {
	var p = pool.Get()
	var v = validator.Get()
	var errc chan error
	// if bc.autoGen {
	// 	bc.Mine(bc.GetLatestBlock())
	// }
	// if bc.autoGen {
	// 	var latest = bc.GetLatestBlock()
	// 	go bc.Mine(latest)
	// }
	for errc == nil {
		select {
		case newBlock := <-gigea.E.BlockPipe:
			fmt.Printf("Approved block!! : %s\r\n", newBlock.GetHash())
			fmt.Printf("SKIPPED : %s\r\n", newBlock.GetHash())
			for _, tx := range newBlock.Transactions {
				// fmt.Printf("Tx: %s\r\n", tx.Hash())
				p.RemoveFromPool(tx.Hash())
				v.ExecuteTransaction(tx)
			}
			bc.mu.TryLock()
			bc.info.Latest = newBlock.GetHash()
			bc.info.Total = bc.info.Total + 1
			bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
			// 	err := SaveToVault(*newBlock)
			bc.Size += newBlock.Header().Size
			bc.data = append(bc.data, newBlock)
			bc.currentBlock = &newBlock
			bc.mu.Unlock()
		case <-bc.maintainTicker.C:
			fmt.Println("tick maintain")
			continue
		}
	}
	errc <- nil
}

func (bc *Chain) Mine(latest *block.Block) {
	// var vld = validator.Get()
	// var pool = pool.Get()
	// time.Sleep(10 * time.Second)

	fmt.Println("MINE ON CHAIN")

	// head := &block.Header{
	// 	Ctx:        latest.Header().Ctx,
	// 	Difficulty: latest.Head.Difficulty,
	// 	Extra:      []byte("OP_AUTO_GEN_BLOCK_DAT"),
	// 	Height:     latest.Header().Height + 1,
	// 	Index:      latest.Header().Index + 1,
	// 	Timestamp:  uint64(time.Now().UnixMilli()),
	// 	Number:     bc.chainId,
	// 	PrevHash:   bc.info.Latest,
	// 	Node:       bc.currentAddress,
	// 	GasLimit:   latest.Head.GasLimit, // todo get gas limit dynamically
	// }
	// // cpy version, should store elsewhere
	// head.V = latest.Head.V
	// newBlock := block.NewBlockWithHeader(head)
	// newBlock.Nonce = latest.Nonce

	// var finalSize = block.CalculateSize(*newBlock)
	// newBlock.Head.Size = finalSize
	// newBlock.Head.GasUsed += uint64(finalSize)
	// gigea.E.BlockFunnel <- newBlock
	// fmt.Printf("Block unconfirmed: %s\r\n", newBlock.GetHash())

	// OLD CODE
	// TODO refactor
	// if len(pool.Prepared) > 0 {
	// 	for _, tx := range pool.Prepared {
	// 		if vld.ValidateTransaction(tx, tx.From()) {
	// 			newBlock.Transactions = append(newBlock.Transactions, *tx)
	// 			newBlock.Head.GasUsed += tx.Gas()
	// 			// newBlock.SetTransaction(tx)
	// 		}
	// 	}
	// }

	// var nodeFees = int(finalSize)

	// bc.DataChannel <- newBlock.ToBytes()

	// if vld.ValidateBlock(*newBlock) {
	// 	bc.t.Add(newBlock)
	// 	var t, err = bc.t.VerifyTree()
	// 	if err != nil || !t {
	// 		log.Printf("Verifying trie error: %s\r\n", err)
	// 	} else {
	// 		bc.info.Latest = newBlock.Hash()
	// 		bc.info.Total = bc.info.Total + 1
	// 		bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	// 		bc.currentBlock = newBlock
	// 		err := SaveToVault(*newBlock)
	// 		if err == nil {
	// 			var rewardAddress = newBlock.Head.Node
	// 			fmt.Printf("Reward to: %s, hash: %s\r\n", rewardAddress, newBlock.Hash())
	// 			bc.data = append(bc.data, *newBlock)
	// 			vld.Reward(rewardAddress)
	// 		}
	// 	}
	// 	// clear array with included txs
	// 	pool.Prepared = nil
	// } else {

	// return
	// }
}

// change block generation time
// val multiply by milliseconds (ms)
func (bc *Chain) ChangeBlockInterval(val int) {
	bc.blockTicker.Reset(time.Duration(time.Duration(val) * time.Millisecond))
}

func (bc *Chain) UpdateChain(newBlock *block.Block) {
	fmt.Printf("Current index: %d with hash: %s\r\n",
		bc.currentBlock.Head.ChainId, bc.currentBlock.GetHash())
	fmt.Printf("Incoming index: %d with hash: %s\r\n", newBlock.Head.Index, newBlock.GetHash())

	if newBlock.Head.ChainId.Cmp(big.NewInt(0)) == 0 {
		// replace all
		ClearVault()
		bc.data = nil
	}
	bc.data = append(bc.data, *newBlock)
	fmt.Printf("Update index: %d with hash: %s\r\n", newBlock.Head.Index, newBlock.GetHash())

	// bc.currentBlock = newBlock
	err := SaveToVault(*newBlock)
	if err == nil {
		var rewardAddress = newBlock.Head.Node
		fmt.Printf("Reward to: %s\r\n", rewardAddress)
	}
	bc.info.Latest = newBlock.GetHash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
}

func (bc *Chain) Idle() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.autoGen = false
}

func (bc *Chain) Resume() {
	bc.mu.Lock()
	bc.autoGen = true
	bc.mu.Unlock()
	for {
		select {
		case <-bc.blockTicker.C:
			// var latest = bc.GetLatestBlock()
			// if bc.autoGen {
			// bc.TryAutoGen(latest)
			// gigea.E.Mine()
			// }
			continue
		case <-bc.maintainTicker.C:
			continue
		}
	}
}

// return lenght of array
func ValidateBlocks(blocks []block.Block) (int, error) {
	// var vld = validator.Get()

	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}

	// for i, blk := range blocks {
	// 	// check version chain
	// 	if vld.GetVersion() != blocks[i].Head.V {
	// 		return i, errors.New("wrong chain version")
	// 	}
	// 	if blocks[i].Head.GasUsed > blocks[i].Head.GasLimit {
	// 		return i, errors.New("wrong gas data in block")
	// 	}
	// 	// Проверка целостности цепочки блоков
	// 	if i > 0 {
	// 		prevBlock := blocks[i-1]
	// 		//log.Printf("%d-%d: %s - %s\r\n", i-1, i, blk.Head.PrevHash, prevBlock.Hash())
	// 		if blk.Head.PrevHash.String() != prevBlock.Hash().String() {
	// 			return i - 1, fmt.Errorf("block %d has invalid previous hash", i)
	// 		}
	// 	}
	// }

	return len(blocks), nil
}
