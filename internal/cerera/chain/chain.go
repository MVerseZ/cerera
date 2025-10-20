package chain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/validator"

	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"
)

const CHAIN_SERVICE_NAME = "CHAIN_CERERA_001_1_7"

type BlockChainStatus struct {
	Total     int         `json:"total,omitempty"`
	ChainWork int         `json:"chainWork,omitempty"`
	Latest    common.Hash `json:"latest,omitempty"`
	Size      int64       `json:"size,omitempty"`
	AvgTime   float64     `json:"avgTime,omitempty"` // Renamed to AvgTime (exported)
	Txs       uint64      `json:"txs,omitempty"`
	Gas       float64     `json:"gas,omitempty"`
	GasPrice  float64     `json:"gasPrice,omitempty"`
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
	DataChannel    chan []byte
	OutBoundEvents chan []byte
	Size           int

	Difficulty uint64
	// stats
	lastBlockTime int64
	blockCount    int64
	totalTime     int64
	avgTime       float64
}

var (
	bch        Chain
	BLOCKTIMER = time.Duration(30 * time.Second)
)

func GetBlockChain() *Chain {
	return &bch
}

func InitBlockChain(cfg *config.Config) (*Chain, error) {

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
	}

	for _, v := range dataBlocks {
		list = append(list, v)
		stats.Total += 1
		stats.ChainWork += v.Head.Size
		stats.Latest = v.GetHash()
	}
	t, err = trie.NewTree(list)
	if err != nil {
		fmt.Printf("error trie validating: %s\r\n", err)
	}

	t.VerifyTree()

	//	0xb51551C31419695B703aD37a2c04A765AB9A6B4a183041354a6D392ce438Aec47eBb16495E84F18ef492B50f652342dE
	// Set current block safely
	var currentBlock *block.Block
	if len(dataBlocks) > 0 {
		currentBlock = dataBlocks[len(dataBlocks)-1]
	} else {
		currentBlock = genesisBlock
	}

	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   currentBlock,
		maintainTicker: time.NewTicker(time.Duration(5 * time.Minute)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
		t:              t,
		DataChannel:    make(chan []byte),
		OutBoundEvents: make(chan []byte),
		Size:           genesisBlock.Header().Size,
		Difficulty:     genesisBlock.Head.Difficulty,
		lastBlockTime:  time.Now().Unix(),
		blockCount:     0,
		totalTime:      0,
	}
	// genesisBlock.Head.Node = bch.currentAddress
	// go bch.BlockGenerator()
	go bch.Start()

	return &bch, nil
}

// Methods ordered alphabetically

func (bc *Chain) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getInfo":
		return bc.GetInfo()
	case "height":
		return bc.GetLatestBlock().Header().Height
	case "getBlockByIndex":
		return bc.GetBlockByNumber(int(params[0].(float64)))
	case "getBlock":
		return bc.GetBlock(common.HexToHash(params[0].(string)))
	case "getBlockHeader":
		return bc.GetBlockHeader(params[0].(string))
	}
	return nil
}

func (bc *Chain) GetBlock(blockHash common.Hash) *block.Block {
	for _, b := range bc.data {
		if b.GetHash().Compare(blockHash) == 0 {
			return b
		}
	}
	return &block.Block{}
}

func (bc *Chain) GetBlockByNumber(number int) *block.Block {
	for _, b := range bc.data {
		if b.Header().Index == uint64(number) {
			return b
		}
	}
	return &block.Block{}
}

func (bc *Chain) GetBlockHash(number int) common.Hash {
	for _, b := range bc.data {
		if b.Header().Index == uint64(number) {
			return b.GetHash()
		}
	}
	return common.EmptyHash()
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

func (bc *Chain) GetChainId() int {
	return bc.chainId
}

func (bc *Chain) GetCurrentChainOwnerAddress() types.Address {
	return bc.currentAddress
}

func (bc *Chain) GetInfo() BlockChainStatus {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Calculate total size
	var totalSize int
	var totalTxs uint64
	for _, b := range bc.data {
		totalSize += b.Header().Size
		totalTxs += uint64(len(b.Transactions))
	}

	// Update info struct with current values
	bc.info.Size = int64(totalSize)
	if len(bc.data) > 0 {
		bc.info.Latest = bc.data[len(bc.data)-1].GetHash()
	}
	bc.info.Total = len(bc.data)
	bc.info.ChainWork = int(bc.chainWork.Int64())
	bc.info.AvgTime = bc.avgTime
	bc.info.Txs = totalTxs

	return bc.info
}

func (bc *Chain) GetLatestBlock() *block.Block {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.currentBlock
}

func (bc *Chain) ServiceName() string {
	return CHAIN_SERVICE_NAME
}

func (bc *Chain) Start() {
	fmt.Printf("Chain started with: %d, chain owner: %s, total: %d\r\n", bc.chainId, bc.currentAddress, bc.info.Total)
	var errc chan error
	for errc == nil {
		select {
		case <-bc.maintainTicker.C:
			fmt.Println("Chain tick maintain")
			continue
		}
	}
	errc <- nil
}

/*
Update chain with new block
param:

	newBlock: new block for chain update
*/
func (bc *Chain) UpdateChain(newBlock *block.Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	// mined block -> simply approved
	currentTime := time.Now().Unix()
	// Calculate time since last block
	timeSinceLast := currentTime - bc.lastBlockTime

	// Update statistics
	bc.blockCount++
	bc.totalTime += timeSinceLast
	bc.lastBlockTime = currentTime

	// Calculate average
	if bc.blockCount > 0 {
		bc.avgTime = float64(bc.totalTime) / float64(bc.blockCount)
	}

	for _, v := range bc.data {
		v.Confirmations += 1
	}

	bc.data = append(bc.data, newBlock)
	bc.currentBlock = newBlock

	// execute block transactions
	var v = validator.Get()
	for _, btx := range newBlock.Transactions {
		v.ExecuteTransaction(btx)
	}
	// fill bc info with new latest block
	bc.info.Latest = newBlock.GetHash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	bc.info.AvgTime = bc.avgTime
	bc.info.Txs += uint64(len(newBlock.Transactions))
	for _, tx := range newBlock.Transactions {
		gas := tx.Gas()
		bc.info.Gas += gas
	}
}

// return lenght of array
func ValidateBlocks(blocks []*block.Block) (int, error) {
	// var vld = validator.Get()

	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}
	return len(blocks), nil
}
