package miner

import (
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

const MINER_ID = "CERERA_MINER:937"

type Miner interface {
	GetID() string
	Start()
	Stop()
	Status() byte
	Update(tx *types.GTransaction)
}

type miner struct {
	status   byte
	config   *config.Config
	chain    *chain.Chain
	pool     pool.TxPool
	mining   bool
	stopChan chan struct{}
}

func (m *miner) GetID() string {
	return MINER_ID
}

func (m *miner) Start() {
	m.status = 0x1
	m.mining = true
	m.stopChan = make(chan struct{})

	fmt.Printf("[MINER] Starting miner with ID: %s\n", m.GetID())

	// Получаем конфигурацию
	m.config = config.GenerageConfig()
	fmt.Printf("[MINER] Chain config loaded: ChainID=%d, Type=%s\n",
		m.config.Chain.ChainID, m.config.Chain.Type)

	// Получаем доступ к сервисам через реестр
	registry, err := service.GetRegistry()
	if err != nil {
		fmt.Printf("[MINER] Error getting service registry: %v\n", err)
		return
	}

	// Получаем цепочку
	_, ok := registry.GetService("chain")
	if !ok {
		fmt.Printf("[MINER] Chain service not found\n")
		return
	}
	m.chain = chain.GetBlockChain()
	fmt.Printf("[MINER] Chain service connected\n")

	// Получаем пул транзакций
	_, ok = registry.GetService("pool")
	if !ok {
		fmt.Printf("[MINER] Pool service not found\n")
		return
	}
	m.pool = pool.Get()
	fmt.Printf("[MINER] Pool service connected\n")

	// Получаем последний блок
	lastBlock := m.chain.GetLatestBlock()
	fmt.Printf("[MINER] Last block: Height=%d, Hash=%s\n",
		lastBlock.Header().Height, lastBlock.GetHash())

	// Запускаем цикл майнинга
	go m.miningLoop()
}

func (m *miner) Stop() {
	m.status = 0x0
	m.mining = false
	if m.stopChan != nil {
		close(m.stopChan)
	}
	fmt.Printf("[MINER] Stopped\n")
}

func (m *miner) miningLoop() {
	ticker := time.NewTicker(3 * time.Second) // Майним каждые 7 секунд
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.mining {
				m.mineBlock()
			}
		case <-m.stopChan:
			fmt.Printf("[MINER] Mining loop stopped\n")
			return
		}
	}
}

func (m *miner) mineBlock() {
	// fmt.Printf("[MINER] Starting scheduled block mining...\n")

	// Получаем последний блок
	lastBlock := m.chain.GetLatestBlock()
	if lastBlock == nil {
		fmt.Printf("[MINER] No last block found\n")
		return
	}

	// Получаем транзакции из пула
	pendingTxs := m.pool.GetPendingTransactions()
	// fmt.Printf("[MINER] Found %d pending transactions\n", len(pendingTxs))

	// Создаем новый блок
	newBlock := m.createNewBlock(lastBlock, pendingTxs)
	if newBlock == nil {
		fmt.Printf("[MINER] Failed to create new block\n")
		return
	}

	// Выполняем майнинг (поиск nonce)
	m.performMining(newBlock)

	// Добавляем блок в цепочку
	m.chain.UpdateChain(newBlock)

	// Очищаем пул от обработанных транзакций
	m.clearProcessedTransactions(newBlock.Transactions)
}

func (m *miner) createNewBlock(lastBlock *block.Block, transactions []types.GTransaction) *block.Block {
	// Создаем заголовок нового блока
	newHeader := &block.Header{
		Ctx:        lastBlock.Header().Ctx,
		Difficulty: lastBlock.Header().Difficulty,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     lastBlock.Header().Height + 1,
		Index:      lastBlock.Header().Index + 1,
		GasLimit:   lastBlock.Header().GasLimit,
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
	coinbaaseTransaction := coinbase.CreateCoinBaseTransation(lastBlock.Header().Nonce, m.config.NetCfg.ADDR)
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

func (m *miner) performMining(block *block.Block) {
	// fmt.Printf("[MINER] Performing simplified mining for block Height=%d\n", block.Header().Height)

	// Простой майнинг - просто устанавливаем случайный nonce
	block.Header().Nonce = uint64(time.Now().UnixNano() % 1000000)

	// Рассчитываем хеш блока
	blockHash, err := block.CalculateHash()
	if err != nil {
		fmt.Printf("[MINER] Error calculating block hash: %v\n", err)
		return
	}

	block.Hash = common.BytesToHash(blockHash)
	// fmt.Printf("[MINER] Block mined with nonce: %d, hash: %x\n", block.Header().Nonce, blockHash)
}

func (m *miner) clearProcessedTransactions(processedTxs []types.GTransaction) {
	for _, tx := range processedTxs {
		if tx.Type() == types.CoinbaseTxType {
			continue
		}
		err := m.pool.RemoveFromPool(tx.Hash())
		if err != nil {
			fmt.Printf("[MINER] Error removing transaction from pool: %v\n", err)
		}
	}
}

func (m *miner) Status() byte {
	return m.status
}

func (m *miner) Update(tx *types.GTransaction) {
	if m.status != 0x1 || !m.mining {
		fmt.Printf("[MINER] Miner not active, ignoring transaction update\n")
		return
	}

	// fmt.Printf("[MINER] Received transaction update: %s (will be included in next mining cycle)\n", tx.Hash())

	// Не рестартуем майнинг - ждем следующий цикл через 7 секунд
	// Транзакция будет включена в следующий блок при следующем майнинге
}

func Init() (Miner, error) {
	return &miner{status: 0x0}, nil
}
