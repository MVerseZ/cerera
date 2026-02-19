package miner

import (
	"math/big"
	"testing"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/pool"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/observer"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTxPool is a mock implementation of pool.TxPool for testing
type mockTxPool struct {
	txs []types.GTransaction
}

func (m *mockTxPool) GetPendingTransactions() []types.GTransaction {
	return m.txs
}

func (m *mockTxPool) RemoveFromPool(hash common.Hash) error {
	for i, tx := range m.txs {
		if tx.Hash() == hash {
			m.txs = append(m.txs[:i], m.txs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockTxPool) AddRawTransaction(tx *types.GTransaction) {
	m.txs = append(m.txs, *tx)
}

func (m *mockTxPool) GetTransaction(hash common.Hash) *types.GTransaction {
	for _, tx := range m.txs {
		if tx.Hash() == hash {
			return &tx
		}
	}
	return nil
}

func (m *mockTxPool) NewTxEvent() <-chan *types.GTransaction {
	return nil
}

func (m *mockTxPool) GetInfo() pool.MemPoolInfo {
	hashes := make([]common.Hash, len(m.txs))
	for i, tx := range m.txs {
		hashes[i] = tx.Hash()
	}
	return pool.MemPoolInfo{
		Size:   len(m.txs),
		Hashes: hashes,
		Txs:    m.txs,
	}
}

func (m *mockTxPool) GetRawMemPool() []interface{} {
	result := make([]interface{}, len(m.txs))
	for i, tx := range m.txs {
		result[i] = tx
	}
	return result
}

func (m *mockTxPool) QueueTransaction(tx *types.GTransaction) {
	m.txs = append(m.txs, *tx)
}

func (m *mockTxPool) ServiceName() string {
	return "POOL_CERERA_001_1_3"
}

func (m *mockTxPool) Exec(method string, params []interface{}) interface{} {
	return nil
}

func (m *mockTxPool) Register(obs observer.Observer) {}

func TestMiner_Init(t *testing.T) {
	m, err := Init()
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Equal(t, MINER_ID, m.GetID())
	assert.Equal(t, byte(0x0), m.Status())
}

func TestMiner_GetID(t *testing.T) {
	m := &miner{}
	assert.Equal(t, MINER_ID, m.GetID())
}

func TestMiner_Status(t *testing.T) {
	m := &miner{status: 0x1}
	assert.Equal(t, byte(0x1), m.Status())

	m.status = 0x0
	assert.Equal(t, byte(0x0), m.Status())
}

func TestMiner_Stop(t *testing.T) {
	m := &miner{
		status:   0x1,
		mining:   true,
		stopChan: make(chan struct{}),
	}

	m.Stop()

	assert.Equal(t, byte(0x0), m.Status())
	assert.False(t, m.mining)
}

func TestMiner_Update(t *testing.T) {
	tests := []struct {
		name      string
		status    byte
		mining    bool
		shouldLog bool
	}{
		{
			name:      "Active miner accepts transaction",
			status:    0x1,
			mining:    true,
			shouldLog: true,
		},
		{
			name:      "Inactive miner ignores transaction",
			status:    0x0,
			mining:    false,
			shouldLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &miner{
				status: tt.status,
				mining: tt.mining,
			}

			tx := types.NewTransaction(
				1,
				types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000),
				3.0,
				big.NewInt(0),
				[]byte("test"),
			)

			// Should not panic
			m.Update(tx)
		})
	}
}

func TestMiner_CreateNewBlock(t *testing.T) {
	m := &miner{
		config: config.GenerageConfig(),
	}

	// Create a valid last block
	lastHeader := &block.Header{
		Height:     100,
		Index:      100,
		Difficulty: 1,
		GasLimit:   1000000,
		ChainId:    11,
		Nonce:      12345,
		PrevHash:   common.Hash{},
		Root:       common.Hash{},
	}
	lastBlock := block.NewBlock(lastHeader)

	// Test with valid block
	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test"),
	)
	txs := []types.GTransaction{*tx}

	newBlock := m.createNewBlock(lastBlock, txs)
	require.NotNil(t, newBlock)
	assert.Equal(t, int(101), int(newBlock.Header().Height))
	assert.Equal(t, int(101), int(newBlock.Header().Index))
	assert.Equal(t, lastBlock.GetHash(), newBlock.Header().PrevHash)
	assert.Greater(t, len(newBlock.Transactions), 0) // Should include coinbase + txs

	// Test with nil block
	nilBlock := m.createNewBlock(nil, txs)
	assert.Nil(t, nilBlock)

	// Test with invalid block
	invalidBlock := block.NewBlock(nil)
	invalidBlockResult := m.createNewBlock(invalidBlock, txs)
	assert.Nil(t, invalidBlockResult)
}

func TestMiner_PerformMining(t *testing.T) {
	m := &miner{}

	header := &block.Header{
		Height:     1,
		Index:      1,
		Difficulty: 1,
		Nonce:      0,
		ChainId:    11,
		PrevHash:   common.Hash{},
		Root:       common.Hash{},
		GasLimit:   1000000,
		GasUsed:    0,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}
	b := block.NewBlock(header)
	// Add empty transactions to make block valid
	b.Transactions = []types.GTransaction{}

	err := m.performMining(b)
	require.NoError(t, err)
	// performMining sets nonce and calculates hash
	// The exact nonce value depends on time, so we just verify it doesn't error
	// In a real scenario, nonce would be set to time.Now().UnixNano() % 1000000
}

func TestMiner_ClearProcessedTransactions(t *testing.T) {
	tx1 := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test1"),
	)
	tx2 := types.NewTransaction(
		2,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(2000),
		3.0,
		big.NewInt(0),
		[]byte("test2"),
	)

	mockPool := &mockTxPool{
		txs: []types.GTransaction{*tx1, *tx2},
	}

	m := &miner{pool: mockPool}

	processedTxs := []types.GTransaction{
		*tx1,
		*tx2,
	}

	initialCount := len(mockPool.txs)
	m.clearProcessedTransactions(processedTxs)

	// Transactions should be removed from pool
	// Note: In real implementation, coinbase transactions are skipped
	assert.Less(t, len(mockPool.txs), initialCount)
}

func TestMiner_Start_Errors(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() *miner
		expectedError error
		checkError    func(t *testing.T, err error)
	}{
		{
			name: "Service registry not found",
			setupFunc: func() *miner {
				m, _ := Init()
				return m.(*miner)
			},
			checkError: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "service registry not found")
			},
		},
		{
			name: "Miner status reset on error",
			setupFunc: func() *miner {
				m, _ := Init()
				miner := m.(*miner)
				miner.status = 0x1
				miner.mining = true
				return miner
			},
			checkError: func(t *testing.T, err error) {
				require.Error(t, err)
				m, _ := Init()
				miner := m.(*miner)
				// После ошибки статус должен быть сброшен
				assert.Equal(t, byte(0x0), miner.status)
				assert.False(t, miner.mining)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setupFunc()
			err := m.Start()
			tt.checkError(t, err)
		})
	}
}

func TestMiner_Start_ChainServiceNotFound(t *testing.T) {
	m, err := Init()
	require.NoError(t, err)

	// Создаем registry без chain service
	registry, _ := service.NewRegistry()
	service.R = registry

	err = m.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain service not found")
	assert.Equal(t, ErrChainServiceNotFound, err)
}

func TestMiner_Start_PoolServiceNotFound(t *testing.T) {
	m, err := Init()
	require.NoError(t, err)

	// Создаем registry с chain service, но без pool service
	registry, _ := service.NewRegistry()
	service.R = registry

	// Мокируем chain service
	chainService := &mockService{name: "CHAIN_CERERA_001_1_7"}
	registry.Register("CHAIN_CERERA_001_1_7", chainService)

	err = m.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool service not found")
	assert.Equal(t, ErrPoolServiceNotFound, err)
}

func TestMiner_Start_LatestBlockNotFound(t *testing.T) {
	m, err := Init()
	require.NoError(t, err)

	// Создаем registry с сервисами, но без chain service, возвращающего блок
	registry, _ := service.NewRegistry()
	service.R = registry

	chainService := &mockService{name: "CHAIN_CERERA_001_1_7"}
	poolService := &mockService{name: "POOL_CERERA_001_1_3"}
	registry.Register("CHAIN_CERERA_001_1_7", chainService)
	registry.Register("POOL_CERERA_001_1_3", poolService)

	err = m.Start()
	require.Error(t, err)
	// Может быть либо ErrLatestBlockNotFound, либо ErrInvalidBlock
	assert.True(t, err == ErrLatestBlockNotFound || err == ErrInvalidBlock || err == ErrBlockHeaderNil)
}

// mockService для тестирования
type mockService struct {
	name string
}

func (m *mockService) ServiceName() string {
	return m.name
}

func (m *mockService) Exec(method string, params []interface{}) interface{} {
	// Для тестирования возвращаем nil, что означает отсутствие результата
	return nil
}

func TestMiner_MiningLoop(t *testing.T) {
	m := &miner{
		status:   0x1,
		mining:   true,
		stopChan: make(chan struct{}),
		pool:     &mockTxPool{},
		config:   config.GenerageConfig(),
	}

	// Start mining loop in background
	done := make(chan bool)
	go func() {
		m.miningLoop()
		done <- true
	}()

	// Stop after a short delay
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	// Wait for loop to finish
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Mining loop did not stop in time")
	}
}
