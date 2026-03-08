package miner

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/types"
	"github.com/cerera/gigea"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validator"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const MINER_ID = "CERERA_MINER:937"

var (
	ErrServiceRegistryNotFound = errors.New("service registry not found")
	ErrChainServiceNotFound    = errors.New("chain service not found")
	ErrPoolServiceNotFound     = errors.New("pool service not found")
	ErrLatestBlockNotFound     = errors.New("latest block not found")
	ErrInvalidBlock            = errors.New("invalid block")
	ErrBlockHeaderNil          = errors.New("block header is nil")
)

// minerLogger returns the miner logger, ensuring it's initialized after global logger
func minerLogger() *zap.SugaredLogger {
	return logger.Named("miner")
}

var (
	minerBlocksMinedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_blocks_mined_total",
		Help: "Total number of blocks successfully mined",
	})
	minerMiningAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_mining_attempts_total",
		Help: "Total number of mining attempts",
	})
	minerMiningErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_mining_errors_total",
		Help: "Total number of mining errors",
	})
	minerMiningDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_mining_duration_seconds",
		Help:    "Time spent mining a block in seconds",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	})
	minerPendingTxsInBlock = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_pending_txs_in_block",
		Help: "Number of pending transactions included in the last mined block",
	})
	minerStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_status",
		Help: "Miner status (0=stopped, 1=active)",
	})
	// Метрики проверки хэша
	minerHashValidationTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_validation_total",
		Help: "Total number of hash validations",
	})
	minerHashValidTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_valid_total",
		Help: "Total number of valid hashes",
	})
	minerHashInvalidTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_invalid_total",
		Help: "Total number of invalid hashes",
	})
	// Метрики поиска nonce
	minerNonceSearchAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_nonce_search_attempts_total",
		Help: "Total number of nonce search attempts",
	})
	minerNonceSearchAttempts = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_nonce_search_attempts",
		Help:    "Distribution of nonce search attempts per block",
		Buckets: []float64{1, 10, 100, 1000, 10000, 100000, 1000000},
	})
	minerNonceSearchDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_nonce_search_duration_seconds",
		Help:    "Time spent searching for valid nonce in seconds",
		Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
	})
	// Метрики difficulty и target
	minerCurrentDifficulty = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_current_difficulty",
		Help: "Current block difficulty",
	})
	minerCurrentTarget = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_current_target",
		Help: "Current target value (2^256 / difficulty)",
	})
	minerHashToTargetRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_hash_to_target_ratio",
		Help: "Ratio of block hash to target (for monitoring proximity to validity)",
	})
)

func init() {
	prometheus.MustRegister(
		minerBlocksMinedTotal,
		minerMiningAttemptsTotal,
		minerMiningErrorsTotal,
		minerMiningDurationSeconds,
		minerPendingTxsInBlock,
		minerStatus,
		minerHashValidationTotal,
		minerHashValidTotal,
		minerHashInvalidTotal,
		minerNonceSearchAttemptsTotal,
		minerNonceSearchAttempts,
		minerNonceSearchDurationSeconds,
		minerCurrentDifficulty,
		minerCurrentTarget,
		minerHashToTargetRatio,
	)
}

type Miner interface {
	GetID() string
	Start() error
	Stop()
	Status() byte
	Update(tx *types.GTransaction)
}

// HeightLockChecker interface for checking height locks from main package
type HeightLockChecker interface {
	TryLockHeight(height int) bool
	IsHeightLocked(height int) bool
	GetCancelChannel() <-chan struct{}
	GetLockedHeight() int
}

type miner struct {
	status byte
	config *config.Config
	// chain    *chain.Chain
	pool     pool.TxPool
	mining   bool
	stopChan chan struct{}

	miningHeight    int   // Height currently being mined
	miningCancelled int32 // 0 = not cancelled, 1 = cancelled (atomic access)

	// Package builder / miner separation.
	// packageBuilderLoop runs in its own goroutine and writes to cachedPkg;
	// miningLoop reads cachedPkg without ever blocking on the pool lock.
	cachedPkg   []*types.GTransaction
	cachedPkgMu sync.Mutex
	pkgReady    chan struct{} // cap=1: coalesced signal to rebuild package immediately
}

func (m *miner) GetID() string {
	return MINER_ID
}

func (m *miner) Start() error {
	m.status = 0x1
	m.mining = true
	m.stopChan = make(chan struct{})
	minerStatus.Set(1)

	minerLogger().Infow("[MINER] Starting miner", "id", m.GetID())

	// Получаем конфигурацию
	m.config = config.GenerageConfig()
	minerLogger().Infow("[MINER] Chain config loaded",
		"chain_id", m.config.Chain.ChainID,
		"type", m.config.Chain.Type)

	// Получаем доступ к сервисам через реестр
	registry, err := service.GetRegistry()
	if err != nil {
		minerLogger().Errorw("Failed to get service registry", "err", err)
		m.status = 0x0
		m.mining = false
		return fmt.Errorf("%w: %v", ErrServiceRegistryNotFound, err)
	}

	// Получаем цепочку
	_, ok := registry.GetService("chain")
	if !ok {
		minerLogger().Errorw("Chain service not found")
		m.status = 0x0
		m.mining = false
		return ErrChainServiceNotFound
	}
	minerLogger().Info("[MINER] Chain service connected")

	// Получаем пул транзакций
	_, ok = registry.GetService("pool")
	if !ok {
		minerLogger().Errorw("Pool service not found")
		m.status = 0x0
		m.mining = false
		return ErrPoolServiceNotFound
	}
	m.pool = pool.Get()
	minerLogger().Info("[MINER] Pool service connected")

	// Получаем последний блок
	lastBlockResult := service.ExecTyped("cerera.chain.getLatestBlock", nil)
	if lastBlockResult == nil {
		minerLogger().Errorw("Failed to get latest block: result is nil")
		m.status = 0x0
		m.mining = false
		return ErrLatestBlockNotFound
	}
	lastBlock, ok := lastBlockResult.(*block.Block)
	if !ok || lastBlock == nil || lastBlock.Head == nil {
		minerLogger().Errorw("Failed to get latest block: block is nil or invalid",
			"ok", ok,
			"block_nil", lastBlock == nil,
			"head_nil", lastBlock != nil && lastBlock.Head == nil)
		m.status = 0x0
		m.mining = false
		return ErrInvalidBlock
	}
	header := lastBlock.Header()
	if header == nil {
		minerLogger().Errorw("Failed to get latest block: header is nil")
		m.status = 0x0
		m.mining = false
		return ErrBlockHeaderNil
	}
	minerLogger().Infow("[MINER] Last block retrieved",
		"height", header.Height,
		"hash", lastBlock.GetHash())

	// packageBuilderLoop fetches the pool package independently so that
	// miningLoop never blocks on pool lock contention.
	go m.packageBuilderLoop()
	go m.miningLoop()
	return nil
}

func (m *miner) Stop() {
	m.status = 0x0
	m.mining = false
	minerStatus.Set(0)
	if m.stopChan != nil {
		close(m.stopChan)
	}
	minerLogger().Info("[MINER] Miner stopped")
}

func (m *miner) miningLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.mining {
				if !m.isConsensusStarted() {
					m.printConsensusStatus()
					minerLogger().Warnw("[MINER] Consensus not started, but attempting to mine block anyway - validator will handle consensus check")
				}
				// Read the package prepared by packageBuilderLoop.
				// This is a single mutex-protected pointer copy — never blocks on pool.
				pkg := m.getCachedPkg()
				m.mineBlock(pkg)
			}
		case <-m.stopChan:
			minerLogger().Info("[MINER] Mining loop stopped")
			return
		}
	}
}

// isConsensusStarted проверяет, начался ли консенсус
func (m *miner) isConsensusStarted() bool {
	registry, err := service.GetRegistry()
	if err != nil {
		return false
	}

	iceService, ok := registry.GetService("ice")
	if !ok {
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			return false
		}
	}

	result := iceService.Exec("isConsensusStarted", nil)
	if started, ok := result.(bool); ok {
		return started
	}

	return false
}

// printConsensusStatus выводит текущий статус консенсуса
func (m *miner) printConsensusStatus() {
	consensusInfo := gigea.GetConsensusInfo()
	minerLogger().Debugw("Consensus not started, skipping block creation",
		"status", consensusInfo["status"],
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"nonce", consensusInfo["nonce"],
		"address", consensusInfo["address"])
}

// packageBuilderLoop runs in a dedicated goroutine and rebuilds the mining
// package independently of the nonce-search loop.
//
// Separation rationale: GetMiningPackage holds graph.mu (RLock) for an O(n)
// traversal + O(n log n) sort. Under high pool load this blocks AddTx/RemoveTx
// writers and, via write-lock priority in sync.RWMutex, also queues subsequent
// miner reads — starving the nonce-search loop of CPU time.
//
// By moving pool access here, the miner goroutine only does a single mutex read
// (getCachedPkg) per mining cycle instead of a potentially long pool traversal.
func (m *miner) packageBuilderLoop() {
	// Build an initial snapshot before the first mining tick fires.
	m.refreshPackage()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.pkgReady:
			// Coalesce bursts: many transactions arriving at once → one rebuild.
			for len(m.pkgReady) > 0 {
				<-m.pkgReady
			}
			m.refreshPackage()
		case <-ticker.C:
			m.refreshPackage()
		case <-m.stopChan:
			return
		}
	}
}

// refreshPackage calls GetMiningPackage and stores the result under cachedPkgMu.
// The mutex is held only for the pointer swap (nanoseconds), not for the pool
// traversal — so miningLoop is never blocked for more than a pointer copy.
func (m *miner) refreshPackage() {
	pkg := m.pool.GetMiningPackage(10_000)
	m.cachedPkgMu.Lock()
	m.cachedPkg = pkg
	m.cachedPkgMu.Unlock()
	minerLogger().Debugw("[MINER] Package refreshed", "txs", len(pkg))
}

// getCachedPkg returns the latest package prepared by packageBuilderLoop.
// Lock is held only for a slice-header copy — no pool traversal involved.
func (m *miner) getCachedPkg() []*types.GTransaction {
	m.cachedPkgMu.Lock()
	pkg := m.cachedPkg
	m.cachedPkgMu.Unlock()
	return pkg
}

// getHeightLockChecker retrieves the HeightLockChecker from the chain (registered in service registry).
func (m *miner) getHeightLockChecker() HeightLockChecker {
	registry, err := service.GetRegistry()
	if err != nil {
		return nil
	}
	chainSvc, ok := registry.GetService(chain.CHAIN_SERVICE_NAME)
	if !ok {
		return nil
	}
	if checker, ok := chainSvc.(HeightLockChecker); ok {
		return checker
	}
	return nil
}

// mineBlock assembles and mines one block using the pre-built pkgTxs snapshot.
// pkgTxs is supplied by miningLoop from the cache maintained by packageBuilderLoop,
// so this function never touches the pool lock — the nonce-search hot-loop is
// fully isolated from pool write contention.
func (m *miner) mineBlock(pkgTxs []*types.GTransaction) {
	startTime := time.Now()
	minerMiningAttemptsTotal.Inc()
	atomic.StoreInt32(&m.miningCancelled, 0)

	// Получаем последний блок
	latestBlockResult := service.ExecTyped("cerera.chain.getLatestBlock", nil)
	if latestBlockResult == nil {
		minerLogger().Warnw("No last block found, skipping mining cycle")
		minerMiningErrorsTotal.Inc()
		return
	}
	latestBlock, ok := latestBlockResult.(*block.Block)
	if !ok || latestBlock == nil || latestBlock.Head == nil {
		minerLogger().Warnw("Invalid last block, skipping mining cycle",
			"ok", ok,
			"block_nil", latestBlock == nil,
			"head_nil", latestBlock != nil && latestBlock.Head == nil)
		minerMiningErrorsTotal.Inc()
		return
	}
	header := latestBlock.Header()
	if header == nil {
		minerLogger().Warnw("Last block header is nil, skipping mining cycle")
		minerMiningErrorsTotal.Inc()
		return
	}

	// Determine the height we're mining for
	targetHeight := header.Height + 1
	m.miningHeight = targetHeight

	// Check if this height is already locked (another node has already mined a block)
	heightLock := m.getHeightLockChecker()
	if heightLock != nil {
		if heightLock.IsHeightLocked(targetHeight) {
			minerLogger().Infow("Height already locked by another node, skipping mining",
				"targetHeight", targetHeight,
				"lockedHeight", heightLock.GetLockedHeight())
			return
		}
	}

	// pkgTxs is a snapshot built by packageBuilderLoop — no pool lock needed here.
	pendingTxs := make([]types.GTransaction, 0, len(pkgTxs))
	for _, tx := range pkgTxs {
		if tx != nil {
			pendingTxs = append(pendingTxs, *tx)
		}
	}
	minerPendingTxsInBlock.Set(float64(len(pendingTxs)))
	minerLogger().Debugw("Mining block", "pending_txs", len(pendingTxs), "height", targetHeight)

	// Создаем новый блок
	newBlock := m.createNewBlock(latestBlock, pendingTxs)
	if newBlock == nil {
		minerLogger().Errorw("Failed to create new block")
		minerMiningErrorsTotal.Inc()
		return
	}

	// Выполняем майнинг (поиск nonce)
	if err := m.performMining(newBlock); err != nil {
		if atomic.LoadInt32(&m.miningCancelled) != 0 {
			minerLogger().Infow("Mining cancelled - received block from network",
				"height", targetHeight)
			return
		}
		minerLogger().Errorw("Mining failed", "err", err)
		minerMiningErrorsTotal.Inc()
		return
	}

	// Check if mining was cancelled while we were mining
	if atomic.LoadInt32(&m.miningCancelled) != 0 {
		minerLogger().Infow("Mining cancelled after nonce found - block already received from network",
			"height", targetHeight)
		return
	}

	// Final check: make sure height is still not locked before proposing
	if heightLock != nil && heightLock.IsHeightLocked(targetHeight) {
		minerLogger().Infow("Height locked during mining, discarding mined block",
			"height", targetHeight,
			"lockedHeight", heightLock.GetLockedHeight())
		return
	}

	// Добавляем блок в цепочку через validator
	var validator = validator.Get()
	validator.ProposeBlock(newBlock)
	minerBlocksMinedTotal.Inc()
	duration := time.Since(startTime).Seconds()
	minerMiningDurationSeconds.Observe(duration)
	txCount := len(newBlock.Transactions)
	minerLogger().Infow("Block mined and proposed",
		"height", newBlock.Header().Height,
		"hash", newBlock.GetHash(),
		"txs", txCount,
		"duration_seconds", duration)
	minerLogger().Infow("New block: transactions count", "count", txCount, "height", newBlock.Header().Height)

	// Broadcast the block to other nodes
	m.broadcastBlock(newBlock)

	// Очищаем пул от обработанных транзакций
	m.clearProcessedTransactions(newBlock.Transactions)
}

func (m *miner) createNewBlock(lastBlock *block.Block, transactions []types.GTransaction) *block.Block {
	if lastBlock == nil || lastBlock.Head == nil {
		minerLogger().Errorw("createNewBlock: lastBlock is nil or invalid")
		return nil
	}

	lastHeader := lastBlock.Header()
	if lastHeader == nil {
		minerLogger().Errorw("createNewBlock: lastBlock header is nil")
		return nil
	}

	// Создаем заголовок нового блока
	newHeader := &block.Header{
		Ctx:        lastHeader.Ctx,
		Difficulty: lastHeader.Difficulty,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     lastHeader.Height + 1,
		Index:      lastHeader.Index + 1,
		GasLimit:   lastHeader.GasLimit,
		GasUsed:    0, // Будет рассчитано после обработки транзакций
		ChainId:    m.config.Chain.ChainID,
		Node:       m.config.NetCfg.ADDR, // Адрес майнера
		Size:       0,                    // Будет рассчитано после создания блока
		Timestamp:  uint64(time.Now().UnixMilli()),
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      lastHeader.Nonce, // Будет установлен при майнинге
		PrevHash:   lastBlock.GetHash(),
		Root:       common.EmptyRootHash, // Будет рассчитан
	}

	// Создаем новый блок
	newBlock := block.NewBlock(newHeader)

	// Фильтруем транзакции по GasLimit: добавляем только те, которые помещаются в лимит
	var totalGasUsed uint64
	var selectedTxs []types.GTransaction

	for _, tx := range transactions {
		txType := tx.Type()
		txGas := tx.Gas()

		// Coinbase-транзакции из пула пропускаем: свежий coinbase создаётся ниже,
		// иначе в блок попадут два coinbase.
		if txType == types.CoinbaseTxType {
			continue
		}

		// Faucet-транзакции не учитываются в GasUsed, но добавляются в блок.
		if txType == types.FaucetTxType {
			selectedTxs = append(selectedTxs, tx)
			continue
		}

		// Для обычных транзакций проверяем GasLimit.
		// Используем continue (не break): транзакции отсортированы по fee rate,
		// а не по газу, поэтому меньшая tx после крупной может ещё поместиться.
		if totalGasUsed+txGas > newHeader.GasLimit {
			minerLogger().Debugw("Transaction exceeds gas limit, skipping",
				"tx_hash", tx.Hash(),
				"tx_gas", txGas,
				"current_gas_used", totalGasUsed,
				"gas_limit", newHeader.GasLimit)
			continue
		}
		selectedTxs = append(selectedTxs, tx)
		totalGasUsed += txGas
	}

	// Добавляем отобранные транзакции в блок
	newBlock.Transactions = selectedTxs

	// Добавляем coinbase транзакцию (она не учитывается в GasUsed)
	coinbaaseTransaction := coinbase.CreateCoinBaseTransation(lastHeader.Nonce, m.config.NetCfg.ADDR)
	newBlock.Transactions = append(newBlock.Transactions, coinbaaseTransaction)

	// Рассчитываем размер блока
	blockBytes := newBlock.ToBytes()
	newBlock.Head.Size = len(blockBytes)

	// Устанавливаем использованный газ (только для обычных транзакций, без coinbase и faucet)
	newBlock.Head.GasUsed = uint64(totalGasUsed)

	minerLogger().Debugw("Block created",
		"height", newHeader.Height,
		"gas_used", totalGasUsed,
		"gas_limit", newHeader.GasLimit,
		"txs_count", len(selectedTxs),
		"total_txs_in_block", len(newBlock.Transactions))

	return newBlock
}

func (m *miner) performMining(block *block.Block) error {
	// Get cancel channel for this mining operation
	var cancelChan <-chan struct{}
	heightLock := m.getHeightLockChecker()
	if heightLock != nil {
		cancelChan = heightLock.GetCancelChannel()
	}

	// Обновляем метрику difficulty
	minerCurrentDifficulty.Set(float64(block.Header().Difficulty))

	// Рассчитываем хеш блока
	blockHash, err := block.CalculateHash()
	if err != nil {
		minerLogger().Errorw("Error calculating block hash", "err", err, "height", block.Header().Height)
		return fmt.Errorf("failed to calculate block hash: %w", err)
	}

	// Защита от деления на ноль
	if block.Header().Difficulty == 0 {
		return fmt.Errorf("difficulty cannot be zero")
	}
	target := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(int64(block.Header().Difficulty)))

	// Обновляем метрику target (используем логарифм для больших чисел)
	targetLog := 256*math.Log2(2) - math.Log2(float64(block.Header().Difficulty))
	minerCurrentTarget.Set(targetLog)

	block.Hash = common.BytesToHash(blockHash)
	blockHashInt := new(big.Int).SetBytes(blockHash)

	// Вычисляем отношение хэша к target для мониторинга
	hashBitLen := blockHashInt.BitLen()
	hashLog := float64(hashBitLen)
	ratio := hashLog - targetLog
	minerHashToTargetRatio.Set(ratio)

	// Проверяем начальный хэш
	minerHashValidationTotal.Inc()
	if blockHashInt.Cmp(target) < 0 {
		minerHashValidTotal.Inc()
	} else {
		minerHashInvalidTotal.Inc()
	}

	// Начинаем отслеживание поиска nonce
	nonceSearchStartTime := time.Now()
	nonceSearchAttempts := uint64(0)
	checkCancelInterval := uint64(100) // Check cancel channel every 100 iterations

	// Цикл поиска валидного nonce: продолжаем пока хеш >= target (невалидный)
	for blockHashInt.Cmp(target) >= 0 {
		// Check for cancellation periodically
		if cancelChan != nil && nonceSearchAttempts%checkCancelInterval == 0 {
		select {
		case <-cancelChan:
			atomic.StoreInt32(&m.miningCancelled, 1)
			minerLogger().Infow("Mining cancelled by external block",
				"height", block.Header().Height,
				"attempts", nonceSearchAttempts)
			return fmt.Errorf("mining cancelled: received block from network")
		default:
			// Continue mining
		}
		}

		// Also check if height is now locked
		if heightLock != nil && nonceSearchAttempts%checkCancelInterval == 0 {
			if heightLock.IsHeightLocked(block.Header().Height) {
				atomic.StoreInt32(&m.miningCancelled, 1)
				minerLogger().Infow("Mining cancelled - height now locked",
					"height", block.Header().Height,
					"lockedHeight", heightLock.GetLockedHeight())
				return fmt.Errorf("mining cancelled: height locked")
			}
		}

		// Increment nonce directly on the header pointer (block.Header() returns
		// block.Head itself, not a copy, so assigning back would be a no-op).
		// Detect uint64 overflow: if nonce wraps to 0 we have exhausted the space.
		next, carry := bits.Add64(block.Head.Nonce, 1, 0)
		if carry != 0 {
			return fmt.Errorf("nonce overflow: exhausted all 2^64 nonce values without finding valid hash")
		}
		block.Head.Nonce = next
		newBlockHash, _ := block.CalculateHash()
		blockHash = newBlockHash
		blockHashInt = new(big.Int).SetBytes(newBlockHash)

		// Обновляем метрики поиска nonce
		nonceSearchAttempts++
		minerNonceSearchAttemptsTotal.Inc()

		// Обновляем отношение хэша к target (используем логарифм через bitLen)
		hashBitLen = blockHashInt.BitLen()
		hashLog = float64(hashBitLen)
		ratio = hashLog - targetLog
		minerHashToTargetRatio.Set(ratio)

		// Проверяем хэш на каждой итерации
		minerHashValidationTotal.Inc()
		if blockHashInt.Cmp(target) < 0 {
			minerHashValidTotal.Inc()
		} else {
			minerHashInvalidTotal.Inc()
		}
	}

	// Final cancellation check before returning success
	if cancelChan != nil {
		select {
		case <-cancelChan:
			atomic.StoreInt32(&m.miningCancelled, 1)
			return fmt.Errorf("mining cancelled at completion")
		default:
		}
	}

	// Обновляем метрики после нахождения валидного nonce
	nonceSearchDuration := time.Since(nonceSearchStartTime).Seconds()
	minerNonceSearchAttempts.Observe(float64(nonceSearchAttempts))
	minerNonceSearchDurationSeconds.Observe(nonceSearchDuration)

	// Обновляем block.Hash после нахождения валидного хеша
	block.Hash = common.BytesToHash(blockHash)

	minerLogger().Infow("Block mined",
		"height", block.Header().Height,
		"nonce", block.Header().Nonce,
		"hash", fmt.Sprintf("%x", blockHash))

	return nil
}

// broadcastBlock sends the mined block to other nodes via Ice
func (m *miner) broadcastBlock(newBlock *block.Block) {
	registry, err := service.GetRegistry()
	if err != nil {
		minerLogger().Warnw("Failed to get registry for broadcast", "err", err)
		return
	}

	iceService, ok := registry.GetService("ice")
	if !ok {
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			minerLogger().Warnw("Ice service not found for block broadcast")
			return
		}
	}

	result := iceService.Exec("broadcastBlock", []interface{}{newBlock})
	if err, ok := result.(error); ok && err != nil {
		minerLogger().Warnw("Failed to broadcast block", "err", err, "height", newBlock.Header().Height)
	} else {
		minerLogger().Infow("Block broadcasted to network", "height", newBlock.Header().Height, "hash", newBlock.GetHash())
	}
}

func (m *miner) clearProcessedTransactions(processedTxs []types.GTransaction) {
	for _, tx := range processedTxs {
		if tx.Type() == types.CoinbaseTxType {
			continue
		}
		err := m.pool.RemoveFromPool(tx.Hash())
		if err != nil {
			minerLogger().Warnw("Error removing transaction from pool",
				"tx_hash", tx.Hash(),
				"err", err)
		}
	}
}

func (m *miner) Status() byte {
	return m.status
}

func (m *miner) Update(tx *types.GTransaction) {
	if m.status != 0x1 || !m.mining {
		minerLogger().Debugw("Miner not active, ignoring transaction update",
			"status", m.status,
			"mining", m.mining,
			"tx_hash", tx.Hash())
		return
	}

	// Signal packageBuilderLoop to rebuild the cached package immediately.
	// Non-blocking: if a rebuild is already queued, the signal is coalesced.
	select {
	case m.pkgReady <- struct{}{}:
	default:
	}
	minerLogger().Debugw("Signalled package builder to refresh",
		"tx_hash", tx.Hash())
}

func Init() (Miner, error) {
	m := &miner{
		status:   0x0,
		pkgReady: make(chan struct{}, 1),
	}
	minerStatus.Set(0)
	return m, nil
}
