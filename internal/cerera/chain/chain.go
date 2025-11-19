package chain

import (
	"errors"
	"log"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"

	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"

	"github.com/prometheus/client_golang/prometheus"
)

const CHAIN_SERVICE_NAME = "CHAIN_CERERA_001_1_7"

// clogger is a dedicated console logger for chain
var clogger = log.New(os.Stdout, "[chain] ", log.LstdFlags|log.Lmicroseconds)

var (
	chainBlocksTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chain_blocks_total",
		Help: "Total number of blocks added to the chain",
	})
	chainTxsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chain_txs_total",
		Help: "Total number of transactions applied to the chain",
	})
	chainGasTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chain_gas_total",
		Help: "Total gas consumed by executed transactions",
	})
	chainAvgBlockTimeSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "chain_avg_block_time_seconds",
		Help: "Average time between blocks (seconds)",
	})
	chainDifficultyGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "chain_difficulty",
		Help: "Current chain difficulty",
	})
)

func init() {
	prometheus.MustRegister(
		chainBlocksTotal,
		chainTxsTotal,
		chainGasTotal,
		chainAvgBlockTimeSeconds,
		chainDifficultyGauge,
	)
}

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

	mu     sync.Mutex
	info   BlockChainStatus
	status byte

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

func Mold(cfg *config.Config) (*Chain, error) {

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

	// if cfg.IN_MEM {
	// 	clogger.Print("Using in-memory storage\n")
	// 	dataBlocks = append(dataBlocks, genesisBlock)
	// } else {
	clogger.Print("Using D5 storage\n")
	// Determine chain file path
	chainPath := cfg.Chain.Path
	if chainPath == "EMPTY" {
		chainPath = "./chain.dat"
		cfg.UpdateChainPath(chainPath)
	}
	// Check if chain file exists
	if _, err := os.Stat(chainPath); os.IsNotExist(err) {
		// File doesn't exist, create new chain with genesis block
		list = append(list, genesisBlock)
		stats.Total += 1
		stats.ChainWork += genesisBlock.Head.Size
		InitChainVaultWithPath(genesisBlock, chainPath)
	} else {
		// File exists, load blocks from file
		readBlocks, err := SyncVaultWithPath(chainPath)
		if err != nil {
			clogger.Printf("Failed to sync vault, using genesis block: %v", err)
			// Fallback to genesis if sync fails
			list = append(list, genesisBlock)
			stats.Total += 1
			stats.ChainWork += genesisBlock.Head.Size
		} else if len(readBlocks) > 0 {
			// Load blocks from file
			dataBlocks = readBlocks
			for _, blk := range readBlocks {
				list = append(list, blk)
				stats.Total += 1
				stats.ChainWork += blk.Head.Size
				stats.Latest = blk.GetHash()
			}
			clogger.Printf("Loaded %d blocks from chain file: %s", len(readBlocks), chainPath)
		} else {
			// File exists but is empty, use genesis block
			list = append(list, genesisBlock)
			stats.Total += 1
			stats.ChainWork += genesisBlock.Head.Size
			InitChainVaultWithPath(genesisBlock, chainPath)
		}
	}
	t, err = trie.NewTree(list)
	if err != nil {
		clogger.Printf("trie validation error: %v", err)
	}
	t.VerifyTree()

	//	0xb51551C31419695B703aD37a2c04A765AB9A6B4a183041354a6D392ce438Aec47eBb16495E84F18ef492B50f652342dE
	// Set current block safely
	var currentBlock *block.Block
	var chainSize int
	var chainDifficulty uint64
	if len(dataBlocks) > 0 {
		currentBlock = dataBlocks[len(dataBlocks)-1]
		if currentBlock != nil && currentBlock.Head != nil {
			if header := currentBlock.Header(); header != nil {
				chainSize = header.Size
			}
			chainDifficulty = currentBlock.Head.Difficulty
		} else {
			// Fallback to genesis if loaded block is invalid
			currentBlock = genesisBlock
			chainSize = genesisBlock.Header().Size
			chainDifficulty = genesisBlock.Head.Difficulty
		}
	} else {
		currentBlock = genesisBlock
		if header := genesisBlock.Header(); header != nil {
			chainSize = header.Size
		}
		chainDifficulty = genesisBlock.Head.Difficulty
	}

	// Ensure currentBlock is never nil
	if currentBlock == nil {
		currentBlock = genesisBlock
		chainSize = genesisBlock.Header().Size
		chainDifficulty = genesisBlock.Head.Difficulty
	}

	// Calculate chainWork from loaded blocks
	chainWorkValue := big.NewInt(int64(stats.ChainWork))
	if chainWorkValue.Cmp(big.NewInt(0)) == 0 {
		chainWorkValue = big.NewInt(1)
	}

	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      chainWorkValue,
		currentBlock:   currentBlock,
		maintainTicker: time.NewTicker(time.Duration(5 * time.Minute)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
		t:              t,
		DataChannel:    make(chan []byte),
		OutBoundEvents: make(chan []byte),
		Size:           chainSize,
		Difficulty:     chainDifficulty,
		lastBlockTime:  time.Now().Unix(),
		blockCount:     0,
		totalTime:      0,
		status:         0x0,
	}
	// genesisBlock.Head.Node = bch.currentAddress
	// go bch.BlockGenerator()
	go bch.Start()

	// Set initial gauges
	chainDifficultyGauge.Set(float64(chainDifficulty))
	chainAvgBlockTimeSeconds.Set(bch.avgTime)

	return &bch, nil
}

// Methods ordered alphabetically

func (bc *Chain) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getInfo":
		return bc.GetInfo()
	case "height":
		latestBlock := bc.GetLatestBlock()
		if latestBlock == nil || latestBlock.Head == nil {
			return 0
		}
		return latestBlock.Header().Height
	case "getBlockByIndex":
		return bc.GetBlockByNumber(int(params[0].(float64)))
	case "getBlock":
		return bc.GetBlock(common.HexToHash(params[0].(string)))
	case "getBlockHeader":
		return bc.GetBlockHeader(params[0].(string))
	case "getLatestBlock":
		return bc.GetLatestBlock()
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
		if b != nil {
			if header := b.Header(); header != nil {
				totalSize += header.Size
			}
		}
		for _, tx := range b.Transactions {
			if txType := tx.Type(); txType != types.CoinbaseTxType && txType != types.FaucetTxType {
				totalTxs += 1
			}
		}
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
	// Ensure currentBlock is never nil
	if bc.currentBlock == nil {
		// This should not happen, but if it does, return nil
		// The caller should handle this case
		return nil
	}
	return bc.currentBlock
}

func (bc *Chain) ServiceName() string {
	return CHAIN_SERVICE_NAME
}

func (bc *Chain) Start() {
	clogger.Printf("chain started: chainId=%d, owner=%s, total=%d", bc.chainId, bc.currentAddress, bc.info.Total)
	for {
		<-bc.maintainTicker.C
		clogger.Println("chain tick maintain")
	}
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
	// var v = validator.Get()
	// for _, btx := range newBlock.Transactions {
	// 	// v.ExecuteTransaction(btx)
	// 	// v.UpdateTxTree(&btx, int(newBlock.Header().Index))
	// }

	// fill bc info with new latest block
	bc.info.Latest = newBlock.GetHash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	bc.info.AvgTime = bc.avgTime
	bc.info.Txs += uint64(len(newBlock.Transactions))
	for _, tx := range newBlock.Transactions {
		if txType := tx.Type(); txType != types.CoinbaseTxType && txType != types.FaucetTxType {
			gas := tx.Gas()
			bc.info.Gas += gas
		}
	}

	// Update Prometheus metrics
	chainBlocksTotal.Inc()
	chainTxsTotal.Add(float64(len(newBlock.Transactions)))
	var blockGas float64
	for _, tx := range newBlock.Transactions {
		blockGas += tx.Gas()
	}
	chainGasTotal.Add(blockGas)
	chainAvgBlockTimeSeconds.Set(bc.avgTime)
	chainDifficultyGauge.Set(float64(bc.Difficulty))
	SaveToVault(*newBlock)
}

// return lenght of array
func ValidateBlocks(blocks []*block.Block) (int, error) {
	// var vld = validator.Get()

	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}
	return len(blocks), nil
}

func (bc *Chain) GetChainConfigStatus() byte {
	return bc.status
}

func (bc *Chain) GetData() []*block.Block {
	return bc.data
}

func (bc *Chain) SetChainConfigStatus(status byte) {
	bc.status = status
}
