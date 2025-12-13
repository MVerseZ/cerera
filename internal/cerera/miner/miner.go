package miner

import (
	"errors"
	"fmt"
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
	// Получаем последний блок
	latestBlockResult := service.ExecTyped("cerera.chain.getLatestBlock", nil)
	if latestBlockResult == nil {
		minerLog.Warnw("No last block found, skipping mining cycle")
		return
	}
	latestBlock, ok := latestBlockResult.(*block.Block)
	if !ok || latestBlock == nil || latestBlock.Head == nil {
		minerLog.Warnw("Invalid last block, skipping mining cycle",
			"ok", ok,
			"block_nil", latestBlock == nil,
			"head_nil", latestBlock != nil && latestBlock.Head == nil)
		return
	}
	header := latestBlock.Header()
	if header == nil {
		minerLog.Warnw("Last block header is nil, skipping mining cycle")
		return
	}

	// Получаем транзакции из пула
	pendingTxs := m.pool.GetPendingTransactions()
	minerLog.Debugw("Mining block", "pending_txs", len(pendingTxs), "height", header.Height+1)

	// Создаем новый блок
	newBlock := m.createNewBlock(latestBlock, pendingTxs)
	if newBlock == nil {
		minerLog.Errorw("Failed to create new block")
		return
	}

	// Выполняем майнинг (поиск nonce)
	if err := m.performMining(newBlock); err != nil {
		minerLog.Errorw("Mining failed", "err", err)
		return
	}

	// Добавляем блок в цепочку через validator
	var validator = validator.Get()
	validator.ProposeBlock(newBlock)
	minerLog.Infow("Block mined and proposed",
		"height", newBlock.Header().Height,
		"hash", newBlock.GetHash(),
		"txs", len(newBlock.Transactions))

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
		Nonce:      0, // Будет установлен при майнинге
		PrevHash:   lastBlock.GetHash(),
		Root:       common.EmptyHash(), // Будет рассчитан
	}

	// Создаем новый блок
	newBlock := block.NewBlock(newHeader)
	coinbaaseTransaction := coinbase.CreateCoinBaseTransation(lastHeader.Nonce, m.config.NetCfg.ADDR)
	newBlock.Transactions = append(transactions, coinbaaseTransaction)

	// Рассчитываем размер блока
	blockBytes := newBlock.ToBytes()
	newBlock.Head.Size = len(blockBytes)

	// Рассчитываем использованный газ
	var totalGasUsed float64
	for _, tx := range newBlock.Transactions {
		totalGasUsed += tx.Gas()
	}
	newBlock.Head.GasUsed = uint64(totalGasUsed)

	return newBlock
}

func (m *miner) performMining(block *block.Block) error {
	// Простой майнинг - просто устанавливаем случайный nonce
	block.Header().Nonce = uint64(time.Now().UnixNano() % 1000000)

	// Рассчитываем хеш блока
	blockHash, err := block.CalculateHash()
	if err != nil {
		minerLog.Errorw("Error calculating block hash", "err", err, "height", block.Header().Height)
		return fmt.Errorf("failed to calculate block hash: %w", err)
	}

	block.Hash = common.BytesToHash(blockHash)
	minerLog.Debugw("Block mined",
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
	return &miner{status: 0x0}, nil
}
