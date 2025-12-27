package miner

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea"
	"github.com/prometheus/client_golang/prometheus"
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

var minerLog = logger.Named("miner")

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

type miner struct {
	status byte
	config *config.Config
	// chain    *chain.Chain
	pool     pool.TxPool
	mining   bool
	stopChan chan struct{}
}

func (m *miner) GetID() string {
	return MINER_ID
}

func (m *miner) Start() error {
	m.status = 0x1
	m.mining = true
	m.stopChan = make(chan struct{})
	minerStatus.Set(1)

	minerLog.Infow("Starting miner", "id", m.GetID())

	// Получаем конфигурацию
	m.config = config.GenerageConfig()
	minerLog.Infow("Chain config loaded",
		"chain_id", m.config.Chain.ChainID,
		"type", m.config.Chain.Type)

	// Получаем доступ к сервисам через реестр
	registry, err := service.GetRegistry()
	if err != nil {
		minerLog.Errorw("Failed to get service registry", "err", err)
		m.status = 0x0
		m.mining = false
		return fmt.Errorf("%w: %v", ErrServiceRegistryNotFound, err)
	}

	// Получаем цепочку
	_, ok := registry.GetService("chain")
	if !ok {
		minerLog.Errorw("Chain service not found")
		m.status = 0x0
		m.mining = false
		return ErrChainServiceNotFound
	}
	minerLog.Info("Chain service connected")

	// Получаем пул транзакций
	_, ok = registry.GetService("pool")
	if !ok {
		minerLog.Errorw("Pool service not found")
		m.status = 0x0
		m.mining = false
		return ErrPoolServiceNotFound
	}
	m.pool = pool.Get()
	minerLog.Info("Pool service connected")

	// Получаем последний блок
	lastBlockResult := service.ExecTyped("cerera.chain.getLatestBlock", nil)
	if lastBlockResult == nil {
		minerLog.Errorw("Failed to get latest block: result is nil")
		m.status = 0x0
		m.mining = false
		return ErrLatestBlockNotFound
	}
	lastBlock, ok := lastBlockResult.(*block.Block)
	if !ok || lastBlock == nil || lastBlock.Head == nil {
		minerLog.Errorw("Failed to get latest block: block is nil or invalid",
			"ok", ok,
			"block_nil", lastBlock == nil,
			"head_nil", lastBlock != nil && lastBlock.Head == nil)
		m.status = 0x0
		m.mining = false
		return ErrInvalidBlock
	}
	header := lastBlock.Header()
	if header == nil {
		minerLog.Errorw("Failed to get latest block: header is nil")
		m.status = 0x0
		m.mining = false
		return ErrBlockHeaderNil
	}
	minerLog.Infow("Last block retrieved",
		"height", header.Height,
		"hash", lastBlock.GetHash())

	// Запускаем цикл майнинга
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
	minerLog.Info("Miner stopped")
}

func (m *miner) miningLoop() {

	ticker := time.NewTicker(5 * time.Second) // Майним каждые 60 секунд
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.mining {
				// Проверяем статус консенсуса для логирования, но не блокируем создание блоков
				// Валидатор проверит консенсус перед добавлением блока в цепочку
				if !m.isConsensusStarted() {
					m.printConsensusStatus()
					minerLog.Warnw("Consensus not started, but attempting to mine block anyway - validator will handle consensus check")
				}
				m.mineBlock()
			}
		case <-m.stopChan:
			minerLog.Info("Mining loop stopped")
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
	minerLog.Debugw("Consensus not started, skipping block creation",
		"status", consensusInfo["status"],
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"nonce", consensusInfo["nonce"],
		"address", consensusInfo["address"])
}

func (m *miner) mineBlock() {
	startTime := time.Now()
	minerMiningAttemptsTotal.Inc()

	// Получаем последний блок
	latestBlockResult := service.ExecTyped("cerera.chain.getLatestBlock", nil)
	if latestBlockResult == nil {
		minerLog.Warnw("No last block found, skipping mining cycle")
		minerMiningErrorsTotal.Inc()
		return
	}
	latestBlock, ok := latestBlockResult.(*block.Block)
	if !ok || latestBlock == nil || latestBlock.Head == nil {
		minerLog.Warnw("Invalid last block, skipping mining cycle",
			"ok", ok,
			"block_nil", latestBlock == nil,
			"head_nil", latestBlock != nil && latestBlock.Head == nil)
		minerMiningErrorsTotal.Inc()
		return
	}
	header := latestBlock.Header()
	if header == nil {
		minerLog.Warnw("Last block header is nil, skipping mining cycle")
		minerMiningErrorsTotal.Inc()
		return
	}

	// Получаем транзакции из пула
	pendingTxs := m.pool.GetPendingTransactions()
	minerPendingTxsInBlock.Set(float64(len(pendingTxs)))
	minerLog.Debugw("Mining block", "pending_txs", len(pendingTxs), "height", header.Height+1)

	// Создаем новый блок
	newBlock := m.createNewBlock(latestBlock, pendingTxs)
	if newBlock == nil {
		minerLog.Errorw("Failed to create new block")
		minerMiningErrorsTotal.Inc()
		return
	}

	// Выполняем майнинг (поиск nonce)
	if err := m.performMining(newBlock); err != nil {
		minerLog.Errorw("Mining failed", "err", err)
		minerMiningErrorsTotal.Inc()
		return
	}

	// Добавляем блок в цепочку через validator
	var validator = validator.Get()
	validator.ProposeBlock(newBlock)
	minerBlocksMinedTotal.Inc()
	duration := time.Since(startTime).Seconds()
	minerMiningDurationSeconds.Observe(duration)
	minerLog.Infow("Block mined and proposed",
		"height", newBlock.Header().Height,
		"hash", newBlock.GetHash(),
		"txs", len(newBlock.Transactions),
		"duration_seconds", duration)

	// Очищаем пул от обработанных транзакций
	m.clearProcessedTransactions(newBlock.Transactions)
}

func (m *miner) createNewBlock(lastBlock *block.Block, transactions []types.GTransaction) *block.Block {
	if lastBlock == nil || lastBlock.Head == nil {
		minerLog.Errorw("createNewBlock: lastBlock is nil or invalid")
		return nil
	}

	lastHeader := lastBlock.Header()
	if lastHeader == nil {
		minerLog.Errorw("createNewBlock: lastBlock header is nil")
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
		Root:       common.EmptyHash(), // Будет рассчитан
	}

	// Создаем новый блок
	newBlock := block.NewBlock(newHeader)

	// Фильтруем транзакции по GasLimit: добавляем только те, которые помещаются в лимит
	var totalGasUsed float64
	var selectedTxs []types.GTransaction

	for _, tx := range transactions {
		txType := tx.Type()
		txGas := tx.Gas()

		// Coinbase и faucet транзакции не учитываются в GasUsed, но добавляются в блок
		if txType == types.CoinbaseTxType || txType == types.FaucetTxType {
			selectedTxs = append(selectedTxs, tx)
			continue
		}

		// Для обычных транзакций проверяем GasLimit
		// Проверяем, не превысит ли добавление этой транзакции GasLimit
		if float64(totalGasUsed)+txGas <= float64(newHeader.GasLimit) {
			selectedTxs = append(selectedTxs, tx)
			totalGasUsed += txGas
		} else {
			// Транзакция не помещается в блок по лимиту газа
			minerLog.Debugw("Transaction exceeds gas limit, skipping",
				"tx_hash", tx.Hash(),
				"tx_gas", txGas,
				"current_gas_used", totalGasUsed,
				"gas_limit", newHeader.GasLimit)
			break // Остальные транзакции тоже не поместятся
		}
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

	minerLog.Debugw("Block created",
		"height", newHeader.Height,
		"gas_used", totalGasUsed,
		"gas_limit", newHeader.GasLimit,
		"txs_count", len(selectedTxs),
		"total_txs_in_block", len(newBlock.Transactions))

	return newBlock
}

func (m *miner) performMining(block *block.Block) error {
	// Простой майнинг - просто устанавливаем случайный nonce

	// Обновляем метрику difficulty
	minerCurrentDifficulty.Set(float64(block.Header().Difficulty))

	// Рассчитываем хеш блока
	blockHash, err := block.CalculateHash()
	if err != nil {
		minerLog.Errorw("Error calculating block hash", "err", err, "height", block.Header().Height)
		return fmt.Errorf("failed to calculate block hash: %w", err)
	}
	// fmt.Printf(">>>>>>>>>>Current block height: %d\r\n", block.Header().Height)
	// fmt.Printf(">>>>>>>>>>Difficulty: %d\r\n", block.Header().Difficulty)
	// fmt.Printf(">>>>>>>>>>Nonce: %d\r\n", block.Header().Nonce)

	// Защита от деления на ноль
	if block.Header().Difficulty == 0 {
		return fmt.Errorf("difficulty cannot be zero")
	}
	target := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(int64(block.Header().Difficulty)))

	// Обновляем метрику target (используем логарифм для больших чисел)
	// target = 2^256 / difficulty, поэтому log(target) = 256*log(2) - log(difficulty)
	targetLog := 256*math.Log2(2) - math.Log2(float64(block.Header().Difficulty))
	minerCurrentTarget.Set(targetLog)

	block.Hash = common.BytesToHash(blockHash)
	blockHashInt := new(big.Int).SetBytes(blockHash)

	// Вычисляем отношение хэша к target для мониторинга (используем логарифм через big.Float)
	// Для больших чисел используем приближение: log2(hash) ≈ bitLen(hash)
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

	// fmt.Printf(">>>>>>>>>>[BLKINT]: %d\r\n", blockHashInt)
	// fmt.Printf(">>>>>>>>>>[TRGINT]: %d\r\n", target)
	// fmt.Printf(">>>>>>>>>>[BLKINTCMP]: %d\r\n", blockHashInt.Cmp(target))

	// Начинаем отслеживание поиска nonce
	nonceSearchStartTime := time.Now()
	nonceSearchAttempts := uint64(0)

	// Цикл поиска валидного nonce: продолжаем пока хеш >= target (невалидный)
	for blockHashInt.Cmp(target) >= 0 {
		newHeader := block.Header()
		newHeader.Nonce += 1
		block.Head = newHeader
		block.UpdateNonce()
		newBlockHash, _ := block.CalculateHash()
		// if err != nil {
		// 	minerLog.Errorw("Error calculating block hash in mining loop", "err", err, "height", block.Header().Height, "nonce", newHeader.Nonce)
		// 	return fmt.Errorf("failed to calculate block hash in mining loop: %w", err)
		// }
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
		minerHashValidationTotal.Inc() // WARN TODO ALARM LABEL CRTUTICAL
		if blockHashInt.Cmp(target) < 0 {
			minerHashValidTotal.Inc()
		} else {
			minerHashInvalidTotal.Inc()
		}

		// fmt.Printf(">>>>>>>>>>Current block height: %d\r\n", block.Header().Height)
		// fmt.Printf(">>>>>>>>>>Difficulty: %d\r\n", block.Header().Difficulty)
		// fmt.Printf(">>>>>>>>>>Nonce: %d\r\n", newHeader.Nonce)
		// fmt.Printf(">>>>>>>>>>[BLKINT]: %d\r\n", blockHashInt)
		// fmt.Printf(">>>>>>>>>>[TRGINT]: %d\r\n", target)
		// fmt.Printf(">>>>>>>>>>[BLKINTCMP]: %d\r\n", blockHashInt.Cmp(target))
	}

	// Обновляем метрики после нахождения валидного nonce
	nonceSearchDuration := time.Since(nonceSearchStartTime).Seconds()
	minerNonceSearchAttempts.Observe(float64(nonceSearchAttempts))
	minerNonceSearchDurationSeconds.Observe(nonceSearchDuration)

	// Обновляем block.Hash после нахождения валидного хеша
	block.Hash = common.BytesToHash(blockHash)

	// if blockHashInt.Cmp(target) >= 0 {
	// 	fmt.Printf("Target is less than block hash\r\n")
	// 	return fmt.Errorf("block hash does not meet difficulty requirement")
	// }
	// Используем обновленный blockHash после цикла майнинга
	// fmt.Printf(">>>>>>>>>>Block mined: %x\r\n", blockHash)
	// fmt.Printf(">>>>>>>>>>Height: %d\r\n", block.Header().Height)
	minerLog.Infow("Block mined",
		"height", block.Header().Height,
		"nonce", block.Header().Nonce,
		"hash", fmt.Sprintf("%x", blockHash))

	return nil
}

func (m *miner) clearProcessedTransactions(processedTxs []types.GTransaction) {
	for _, tx := range processedTxs {
		if tx.Type() == types.CoinbaseTxType {
			continue
		}
		err := m.pool.RemoveFromPool(tx.Hash())
		if err != nil {
			minerLog.Warnw("Error removing transaction from pool",
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
		minerLog.Debugw("Miner not active, ignoring transaction update",
			"status", m.status,
			"mining", m.mining,
			"tx_hash", tx.Hash())
		return
	}

	minerLog.Debugw("Received transaction update, will be included in next mining cycle",
		"tx_hash", tx.Hash())
	// Транзакция будет включена в следующий блок при следующем майнинге
}

func Init() (Miner, error) {
	m := &miner{status: 0x0}
	minerStatus.Set(0) // Initialize status metric
	return m, nil
}
