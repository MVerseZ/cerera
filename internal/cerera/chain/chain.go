package chain

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"

	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"

	"github.com/prometheus/client_golang/prometheus"
)

const CHAIN_SERVICE_NAME = "CHAIN_CERERA_001_1_7"

var clogger = logger.Named("chain")

// ChainMetrics encapsulates Prometheus metrics for the chain
type ChainMetrics struct {
	blocksTotal         prometheus.Counter
	txsTotal            prometheus.Counter
	gasTotal            prometheus.Counter
	avgBlockTimeSeconds prometheus.Gauge
	difficultyGauge     prometheus.Gauge
	blockchainSizeBytes prometheus.Gauge
	height              prometheus.Gauge
	blockSizeBytes      prometheus.Histogram
	blockGasUsed        prometheus.Histogram
}

// NewChainMetrics creates and registers new chain metrics
func NewChainMetrics() *ChainMetrics {
	metrics := &ChainMetrics{
		blocksTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "chain_blocks_total",
			Help: "Total number of blocks added to the chain",
		}),
		txsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "chain_txs_total",
			Help: "Total number of transactions applied to the chain",
		}),
		gasTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "chain_gas_total",
			Help: "Total gas consumed by executed transactions",
		}),
		avgBlockTimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chain_avg_block_time_seconds",
			Help: "Average time between blocks (seconds)",
		}),
		difficultyGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chain_difficulty",
			Help: "Current chain difficulty",
		}),
		blockchainSizeBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chain_blockchain_size_bytes",
			Help: "Total blockchain size in bytes (sum of block header sizes)",
		}),
		height: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "chain_height",
			Help: "Current chain height (latest block number)",
		}),
		blockSizeBytes: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "chain_block_size_bytes",
			Help:    "Size of blocks in bytes",
			Buckets: []float64{1024, 4096, 16384, 65536, 262144, 1048576},
		}),
		blockGasUsed: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "chain_block_gas_used",
			Help:    "Gas used per block",
			Buckets: []float64{1000, 5000, 10000, 50000, 100000, 500000, 1000000},
		}),
	}

	prometheus.MustRegister(
		metrics.blocksTotal,
		metrics.txsTotal,
		metrics.gasTotal,
		metrics.avgBlockTimeSeconds,
		metrics.difficultyGauge,
		metrics.blockchainSizeBytes,
		metrics.height,
		metrics.blockSizeBytes,
		metrics.blockGasUsed,
	)

	return metrics
}

// ChainStats holds chain statistics
type ChainStats struct {
	lastBlockTime int64
	blockCount    int64
	totalTime     int64
	avgTime       float64
}

// BlockStorage interface for block storage operations
type BlockStorage interface {
	Save(block *block.Block, chainPath string) error
	Load(chainPath string) ([]*block.Block, error)
	Init(block *block.Block, chainPath string) error
}

// fileBlockStorage implements BlockStorage using file system
type fileBlockStorage struct{}

func (s *fileBlockStorage) Save(blk *block.Block, chainPath string) error {
	return SaveToVaultWithPath(*blk, chainPath)
}

func (s *fileBlockStorage) Load(chainPath string) ([]*block.Block, error) {
	return SyncVaultWithPath(chainPath)
}

func (s *fileBlockStorage) Init(blk *block.Block, chainPath string) error {
	InitChainVaultWithPath(blk, chainPath)
	return nil
}

type BlockChainStatus struct {
	Total      int         `json:"total,omitempty"`
	ChainWork  int         `json:"chainWork,omitempty"`
	Latest     common.Hash `json:"latest,omitempty"`
	Size       int64       `json:"size,omitempty"`
	AvgTime    float64     `json:"avgTime,omitempty"`
	Txs        uint64      `json:"txs,omitempty"`
	Gas        float64     `json:"gas,omitempty"`
	GasPrice   float64     `json:"gasPrice,omitempty"`
	Difficulty uint64      `json:"difficulty,omitempty"`
}

type Chain struct {
	autoGen        bool
	chainId        int
	chainWork      *big.Int
	currentAddress types.Address
	currentBlock   *block.Block

	mu     sync.RWMutex
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
	stats      ChainStats

	// dependencies
	metrics   *ChainMetrics
	storage   BlockStorage
	chainPath string

	// height lock for sync/miner: prevents mining on height already received from network
	heightMu     sync.Mutex
	lockedHeight int
	cancelCh     chan struct{}
}

var (
	BLOCKTIMER = time.Duration(30 * time.Second)
)

func Mold(cfg *config.Config) (*Chain, error) {
	metrics := NewChainMetrics()
	storage := &fileBlockStorage{}

	genesisBlock, err := loadGenesisBlock(cfg.Chain.ChainID)
	if err != nil {
		return nil, err
	}

	chainPath := determineChainPath(cfg)
	if chainPath == "EMPTY" {
		chainPath = "./chain.dat"
		cfg.UpdateChainPath(chainPath)
	}

	dataBlocks, stats, err := loadBlocksFromStorage(cfg, genesisBlock, chainPath, storage)
	if err != nil {
		return nil, err
	}

	t, err := initializeTrie(dataBlocks)
	if err != nil {
		clogger.Warnw("trie validation error", "err", err)
	}

	currentBlock, chainSize, chainDifficulty := determineCurrentBlock(dataBlocks, genesisBlock)

	chainWorkValue := big.NewInt(int64(stats.ChainWork))
	if chainWorkValue.Cmp(big.NewInt(0)) == 0 {
		chainWorkValue = big.NewInt(1)
	}

	initialTotalSize := calculateInitialTotalSize(dataBlocks)

	chain := &Chain{
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
		stats: ChainStats{
			lastBlockTime: time.Now().Unix(),
			blockCount:    0,
			totalTime:     0,
			avgTime:       0,
		},
		metrics:   metrics,
		storage:   storage,
		chainPath: chainPath,
		cancelCh:  make(chan struct{}),
	}

	chain.info.Size = int64(initialTotalSize)
	go chain.Start()

	// Set initial gauges
	metrics.difficultyGauge.Set(float64(chainDifficulty))
	metrics.avgBlockTimeSeconds.Set(chain.stats.avgTime)
	if currentBlock != nil && currentBlock.Head != nil {
		metrics.height.Set(float64(currentBlock.Header().Height))
	}
	metrics.blockchainSizeBytes.Set(float64(initialTotalSize))

	return chain, nil
}

func loadGenesisBlock(chainID int) (*block.Block, error) {
	genesisHead := block.GenesisHead(chainID)
	genesisBlock := block.NewBlockWithHeaderAndHash(genesisHead)
	genesisBlock.UpdateHash()
	return genesisBlock, nil
}

func determineChainPath(cfg *config.Config) string {
	if cfg.IN_MEM {
		return ""
	}
	return cfg.Chain.Path
}

func loadBlocksFromStorage(cfg *config.Config, genesisBlock *block.Block, chainPath string, storage BlockStorage) ([]*block.Block, BlockChainStatus, error) {
	stats := BlockChainStatus{
		Total:     0,
		ChainWork: 0,
		Size:      0,
	}

	if cfg.IN_MEM {
		clogger.Info("Using in-memory storage")
		stats.Total = 1
		stats.ChainWork = genesisBlock.Head.Size
		return []*block.Block{genesisBlock}, stats, nil
	}

	clogger.Info("Using D5 storage")

	if _, err := os.Stat(chainPath); os.IsNotExist(err) {
		stats.Total = 1
		stats.ChainWork = genesisBlock.Head.Size
		if err := storage.Init(genesisBlock, chainPath); err != nil {
			return nil, stats, err
		}
		return []*block.Block{genesisBlock}, stats, nil
	}

	readBlocks, err := storage.Load(chainPath)
	if err != nil {
		clogger.Warnw("Failed to sync vault, using genesis block", "err", err)
		stats.Total = 1
		stats.ChainWork = genesisBlock.Head.Size
		return []*block.Block{genesisBlock}, stats, nil
	}

	if len(readBlocks) == 0 {
		stats.Total = 1
		stats.ChainWork = genesisBlock.Head.Size
		if err := storage.Init(genesisBlock, chainPath); err != nil {
			return nil, stats, err
		}
		return []*block.Block{genesisBlock}, stats, nil
	}

	dataBlocks := readBlocks
	for _, blk := range readBlocks {
		stats.Total++
		stats.ChainWork += blk.Head.Size
		stats.Latest = blk.GetHash()
	}

	clogger.Infow("Loaded blocks from chain file", "count", len(readBlocks), "path", chainPath)
	return dataBlocks, stats, nil
}

func initializeTrie(blocks []*block.Block) (*trie.MerkleTree, error) {
	var list []trie.Content
	for _, blk := range blocks {
		list = append(list, blk)
	}
	t, err := trie.NewTree(list)
	if err != nil {
		return nil, err
	}
	t.VerifyTree()
	return t, nil
}

func determineCurrentBlock(dataBlocks []*block.Block, genesisBlock *block.Block) (*block.Block, int, uint64) {
	if len(dataBlocks) == 0 {
		if header := genesisBlock.Header(); header != nil {
			return genesisBlock, header.Size, genesisBlock.Head.Difficulty
		}
		return genesisBlock, 0, genesisBlock.Head.Difficulty
	}

	currentBlock := dataBlocks[len(dataBlocks)-1]
	if currentBlock == nil || currentBlock.Head == nil {
		if header := genesisBlock.Header(); header != nil {
			return genesisBlock, header.Size, genesisBlock.Head.Difficulty
		}
		return genesisBlock, 0, genesisBlock.Head.Difficulty
	}

	if header := currentBlock.Header(); header != nil {
		return currentBlock, header.Size, currentBlock.Head.Difficulty
	}
	return currentBlock, 0, currentBlock.Head.Difficulty
}

func calculateInitialTotalSize(dataBlocks []*block.Block) int {
	var totalSize int
	for _, blk := range dataBlocks {
		if blk != nil {
			if header := blk.Header(); header != nil {
				totalSize += header.Size
			}
		}
	}
	return totalSize
}

// Service methods
// Methods ordered alphabetically

func (bc *Chain) Exec(method string, params []interface{}) interface{} {
	if method == "" {
		return nil
	}

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
		if len(params) == 0 {
			return nil
		}
		index, ok := params[0].(float64)
		if !ok {
			return nil
		}
		return bc.GetBlockByNumber(int(index))
	case "getBlock":
		if len(params) == 0 {
			return nil
		}
		hashStr, ok := params[0].(string)
		if !ok {
			return nil
		}
		return bc.GetBlock(common.HexToHash(hashStr))
	case "getBlockHeader":
		if len(params) == 0 {
			return nil
		}
		hashStr, ok := params[0].(string)
		if !ok {
			return nil
		}
		return bc.GetBlockHeader(hashStr)
	case "getLatestBlock":
		return bc.GetLatestBlock()
	}
	return nil
}

func (bc *Chain) GetBlock(blockHash common.Hash) *block.Block {
	if blockHash.Compare(common.EmptyHash()) == 0 {
		return nil
	}

	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, b := range bc.data {
		if b != nil && b.GetHash().Compare(blockHash) == 0 {
			return b
		}
	}
	return nil
}

func (bc *Chain) GetBlockByNumber(number int) *block.Block {
	if number < 0 {
		return nil
	}

	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, b := range bc.data {
		if b != nil && b.Header() != nil && b.Header().Index == uint64(number) {
			return b
		}
	}
	return nil
}

func (bc *Chain) GetBlockHash(number int) common.Hash {
	if number < 0 {
		return common.EmptyHash()
	}

	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, b := range bc.data {
		if b != nil && b.Header() != nil && b.Header().Index == uint64(number) {
			return b.GetHash()
		}
	}
	return common.EmptyHash()
}

func (bc *Chain) GetBlockHeader(blockHash string) *block.Header {
	if blockHash == "" {
		return nil
	}

	bHash := common.HexToHash(blockHash)
	block := bc.GetBlock(bHash)
	if block == nil {
		return nil
	}
	return block.Header()
}

func (bc *Chain) GetChainId() int {
	return bc.chainId
}

func (bc *Chain) GetCurrentChainOwnerAddress() types.Address {
	return bc.currentAddress
}

func (bc *Chain) GetInfo() BlockChainStatus {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return BlockChainStatus{
		Size:       bc.info.Size,
		Latest:     bc.info.Latest,
		Total:      bc.info.Total,
		ChainWork:  bc.info.ChainWork,
		AvgTime:    bc.info.AvgTime,
		Txs:        bc.info.Txs,
		Gas:        bc.info.Gas,
		GasPrice:   bc.info.GasPrice,
		Difficulty: bc.info.Difficulty,
	}
}

func (bc *Chain) GetLatestBlock() *block.Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.currentBlock == nil {
		return nil
	}
	return bc.currentBlock
}

func (bc *Chain) ServiceName() string {
	return CHAIN_SERVICE_NAME
}

func (bc *Chain) Start() {
	clogger.Infow("chain started", "chainId", bc.chainId, "owner", bc.currentAddress, "total", bc.info.Total)
	for {
		<-bc.maintainTicker.C
		clogger.Debug("chain tick maintain")
	}
}

// TryLockHeight locks the given height (e.g. when block is added from sync/consensus).
// Returns false if this height is already locked. Used to prevent miner from competing.
func (bc *Chain) TryLockHeight(height int) bool {
	bc.heightMu.Lock()
	defer bc.heightMu.Unlock()
	if bc.lockedHeight >= height {
		return false
	}
	oldCh := bc.cancelCh
	bc.cancelCh = make(chan struct{})
	bc.lockedHeight = height
	if oldCh != nil {
		close(oldCh)
	}
	return true
}

// IsHeightLocked returns whether the given height is locked (block received from network).
func (bc *Chain) IsHeightLocked(height int) bool {
	bc.heightMu.Lock()
	defer bc.heightMu.Unlock()
	return bc.lockedHeight >= height
}

// GetLockedHeight returns the current locked height.
func (bc *Chain) GetLockedHeight() int {
	bc.heightMu.Lock()
	defer bc.heightMu.Unlock()
	return bc.lockedHeight
}

// GetCancelChannel returns a channel that is closed when a new block height is locked (miner should cancel).
func (bc *Chain) GetCancelChannel() <-chan struct{} {
	bc.heightMu.Lock()
	defer bc.heightMu.Unlock()
	return bc.cancelCh
}

/*
Update chain with new block
param:

	newBlock: new block for chain update
*/
func (bc *Chain) UpdateChain(newBlock *block.Block) error {
	if newBlock == nil {
		return errors.New("newBlock cannot be nil")
	}
	if newBlock.Head == nil {
		return errors.New("newBlock.Head cannot be nil")
	}

	height := newBlock.Header().Height
	if height <= 0 {
		// genesis or invalid; allow without lock
	} else if !bc.TryLockHeight(height) {
		return fmt.Errorf("height %d already locked", height)
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()

	currentTime := time.Now().Unix()
	timeSinceLast := currentTime - bc.stats.lastBlockTime

	bc.stats.blockCount++
	bc.stats.totalTime += timeSinceLast
	bc.stats.lastBlockTime = currentTime

	if bc.stats.blockCount > 0 {
		bc.stats.avgTime = float64(bc.stats.totalTime) / float64(bc.stats.blockCount)
	}

	for _, v := range bc.data {
		if v != nil {
			v.Confirmations += 1
		}
	}

	bc.data = append(bc.data, newBlock)
	bc.currentBlock = newBlock

	bc.info.Latest = newBlock.GetHash()
	bc.info.Total++
	bc.info.ChainWork += newBlock.Head.Size
	bc.info.AvgTime = bc.stats.avgTime
	bc.info.Txs += uint64(len(newBlock.Transactions))

	var blockGas float64
	for _, tx := range newBlock.Transactions {
		if txType := tx.Type(); txType != types.CoinbaseTxType && txType != types.FaucetTxType {
			gas := tx.Gas()
			bc.info.Gas += gas
			blockGas += gas
		}
	}

	header := newBlock.Header()
	if header != nil {
		bc.info.Size += int64(header.Size)
	}

	bc.metrics.blocksTotal.Inc()
	bc.metrics.txsTotal.Add(float64(len(newBlock.Transactions)))
	bc.metrics.gasTotal.Add(blockGas)
	bc.metrics.avgBlockTimeSeconds.Set(bc.stats.avgTime)
	bc.metrics.difficultyGauge.Set(float64(bc.Difficulty))

	if header != nil {
		bc.metrics.height.Set(float64(header.Height))
		blockSizeBytes := len(newBlock.ToBytes())
		bc.metrics.blockSizeBytes.Observe(float64(blockSizeBytes))
		bc.metrics.blockchainSizeBytes.Set(float64(bc.info.Size))
		bc.metrics.blockGasUsed.Observe(float64(newBlock.Head.GasUsed))
	}

	// Persist block only when a storage backend and path are configured.
	// In in-memory mode (cfg.IN_MEM), chainPath is empty and persistence is skipped.
	if bc.storage != nil && bc.chainPath != "" {
		if err := bc.storage.Save(newBlock, bc.chainPath); err != nil {
			return err
		}
	}

	return nil
}

// return lenght of array
func ValidateBlocks(blocks []*block.Block) (int, error) {
	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}
	return len(blocks), nil
}

func (bc *Chain) GetChainConfigStatus() byte {
	return bc.status
}

func (bc *Chain) GetData() []*block.Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.data
}

func (bc *Chain) SetChainConfigStatus(status byte) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.status = status
}
