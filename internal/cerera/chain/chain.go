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
	chainId        int
	chainWork      *big.Int
	currentAddress types.Address
	currentBlock   *block.Block
	// rootHash       common.Hash

	mu   sync.Mutex
	info BlockChainStatus
	data []*block.Block
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

func InitBlockChain(cfg *config.Config) error {

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

	genesisHead := block.GenesisHead(cfg.Chain.ChainID)
	genesisBlock := block.NewBlockWithHeaderAndHash(genesisHead)
	genesisBlock.UpdateHash()
	// genesisBlock.Hash = miner.CalculateHash(&genesisBlock)

	dataBlocks := make([]*block.Block, 0)
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
		currentBlock:   dataBlocks[len(dataBlocks)-1],
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

	return nil
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
			return b
		}
	}
	return &block.Block{}
}

func (bc *Chain) GetBlock(blockHash common.Hash) *block.Block {
	for _, b := range bc.data {
		if b.GetHash().Compare(blockHash) == 0 {
			return b
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
	fmt.Printf("Chain started with: %d, chain owner: %s, total: %d\r\n", bc.chainId, bc.currentAddress, bc.info.Total)
	var p = pool.Get()
	var v = validator.Get()
	var errc chan error
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
			bc.data = append(bc.data, &newBlock)
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
	fmt.Println("MINE ON CHAIN")
}

// change block generation time
// val multiply by milliseconds (ms)
func (bc *Chain) ChangeBlockInterval(val int) {
	bc.blockTicker.Reset(time.Duration(time.Duration(val) * time.Millisecond))
}

func (bc *Chain) UpdateChain(newBlock *block.Block) {
	fmt.Printf("Current index: %d with hash: %s\r\n",
		bc.currentBlock.Head.Index, bc.currentBlock.GetHash())
	fmt.Printf("Incoming index: %d with hash: %s\r\n", newBlock.Head.Index, newBlock.GetHash())

	// if newBlock.Head.ChainId.Cmp(big.NewInt(0)) == 0 {
	// 	// replace all
	// 	ClearVault()
	// 	bc.data = nil
	// }
	bc.DataChannel <- newBlock.ToBytes()

	bc.data = append(bc.data, newBlock)
	bc.currentBlock = newBlock
	fmt.Printf("Update index: %d with hash: %s\r\n", newBlock.Head.Index, newBlock.GetHash())

	// bc.currentBlock = newBlock
	err := SaveToVault(*newBlock)
	if err == nil {
		var rewardAddress = newBlock.Head.Node
		fmt.Printf("Reward to: %s\r\n", rewardAddress)
	}

	var v = validator.Get()
	for _, btx := range newBlock.Transactions {
		v.ExecuteTransaction(btx)
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
func (bc *Chain) GetChainId() int {
	return bc.chainId
}
func (bc *Chain) GetCurrentChainOwnerAddress() types.Address {
	return bc.currentAddress
}

// return lenght of array
func ValidateBlocks(blocks []*block.Block) (int, error) {
	// var vld = validator.Get()

	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}
	return len(blocks), nil
}
