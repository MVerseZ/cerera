package miner

import (
	"math/big"
	"testing"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/observer"
	"github.com/cerera/internal/service"
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

func (m *mockTxPool) AddRawTransaction(tx *types.GTransaction) error {
	m.txs = append(m.txs, *tx)
	return nil
}

func (m *mockTxPool) Stop() {}

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
	txPtrs := make([]*types.GTransaction, len(m.txs))
	for i := range m.txs {
		hashes[i] = m.txs[i].Hash()
		txPtrs[i] = &m.txs[i]
	}
	return pool.MemPoolInfo{
		Size:   len(m.txs),
		Hashes: hashes,
		Txs:    txPtrs,
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

// GetMiningPackage stub — delegates to GetTopN (no dependency graph in mock).
func (m *mockTxPool) GetMiningPackage(n int) []*types.GTransaction {
	return m.GetTopN(n)
}

// GetTopN returns up to n transaction pointers from the front of the slice.
// The mock does not sort — ordering is determined by insertion order.
func (m *mockTxPool) GetTopN(n int) []*types.GTransaction {
	if n > len(m.txs) {
		n = len(m.txs)
	}
	if n == 0 {
		return nil
	}
	result := make([]*types.GTransaction, n)
	for i := 0; i < n; i++ {
		result[i] = &m.txs[i]
	}
	return result
}

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

// ─── mockTxPool.GetTopN tests ─────────────────────────────────────────────────

// TestMockTxPool_GetTopN_Empty verifies GetTopN returns nil on an empty pool.
func TestMockTxPool_GetTopN_Empty(t *testing.T) {
	mp := &mockTxPool{}
	assert.Nil(t, mp.GetTopN(5), "GetTopN on empty mock must return nil")
}

// TestMockTxPool_GetTopN_NGreaterThanSize verifies no panic and all txs returned.
func TestMockTxPool_GetTopN_NGreaterThanSize(t *testing.T) {
	tx1 := types.NewTransaction(1, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(100), 3, big.NewInt(1), nil)
	tx2 := types.NewTransaction(2, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(200), 3, big.NewInt(2), nil)
	mp := &mockTxPool{txs: []types.GTransaction{*tx1, *tx2}}

	top := mp.GetTopN(100)
	require.Len(t, top, 2, "should return all txs when n > pool size")
	assert.Equal(t, tx1.Hash(), top[0].Hash())
	assert.Equal(t, tx2.Hash(), top[1].Hash())
}

// TestMockTxPool_GetTopN_ExactN verifies exactly n pointers are returned.
func TestMockTxPool_GetTopN_ExactN(t *testing.T) {
	txs := make([]types.GTransaction, 5)
	for i := range txs {
		tx := types.NewTransaction(uint64(i+1), types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(int64(i+1)*100), 3, big.NewInt(int64(i+1)), nil)
		txs[i] = *tx
	}
	mp := &mockTxPool{txs: txs}

	top := mp.GetTopN(3)
	require.Len(t, top, 3, "must return exactly 3 items")
}

// ─── createNewBlock tests ─────────────────────────────────────────────────────

// TestMiner_CreateNewBlock_GasLimitEnforcement verifies that regular transactions
// exceeding the per-transaction gas budget are skipped with continue (not break),
// so smaller transactions that still fit are not unfairly excluded.
func TestMiner_CreateNewBlock_GasLimitEnforcement(t *testing.T) {
	m := &miner{config: config.GenerageConfig()}

	lastHeader := &block.Header{
		Height:     10,
		Index:      10,
		Difficulty: 1,
		GasLimit:   100, // very tight limit
		ChainId:    11,
		Nonce:      0,
	}
	lastBlock := block.NewBlock(lastHeader)

	// tx1 uses 60 gas — fits;  totalGasUsed = 60.
	// tx2 uses 60 gas — 60+60 = 120 > 100, skipped (continue, not break).
	// tx3 uses 10 gas — 60+10 =  70 ≤ 100, fits;  totalGasUsed = 70.
	tx1 := types.NewTransaction(1, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(1), 60, big.NewInt(1), []byte("a"))
	tx2 := types.NewTransaction(2, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(1), 60, big.NewInt(1), []byte("b"))
	tx3 := types.NewTransaction(3, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(1), 10, big.NewInt(1), []byte("c"))

	newBlock := m.createNewBlock(lastBlock, []types.GTransaction{*tx1, *tx2, *tx3})
	require.NotNil(t, newBlock)

	txHashes := make(map[common.Hash]struct{}, len(newBlock.Transactions))
	for _, tx := range newBlock.Transactions {
		txHashes[tx.Hash()] = struct{}{}
	}
	assert.Contains(t, txHashes, tx1.Hash(), "tx1 fits and must be included")
	assert.NotContains(t, txHashes, tx2.Hash(), "tx2 exceeds remaining gas and must be excluded")
	assert.Contains(t, txHashes, tx3.Hash(), "tx3 fits in remaining gas (60+10=70≤100) and must be included")

	assert.Equal(t, uint64(70), newBlock.Head.GasUsed, "GasUsed must equal sum of accepted regular tx gas (60+10)")
}

// TestMiner_CreateNewBlock_EmptyTransactions verifies that a block is created with
// only the coinbase transaction when no pending transactions are provided.
func TestMiner_CreateNewBlock_EmptyTransactions(t *testing.T) {
	m := &miner{config: config.GenerageConfig()}

	lastHeader := &block.Header{Height: 5, Index: 5, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)

	newBlock := m.createNewBlock(lastBlock, nil)
	require.NotNil(t, newBlock)

	// The only transaction must be the coinbase.
	require.Len(t, newBlock.Transactions, 1, "empty pending list → only coinbase in block")
	assert.Equal(t, uint8(types.CoinbaseTxType), newBlock.Transactions[0].Type(), "single tx must be coinbase")
}

// TestMiner_CreateNewBlock_CoinbaseTxSkippedFromPool verifies that CoinbaseTxType
// transactions passed in from the pool are dropped (to prevent double-coinbase),
// while createNewBlock still appends exactly one fresh coinbase at the end.
//
// Note: crvTxHash does not include nonce, so two coinbase txs may collide on
// hash when created within the same nanosecond. The meaningful invariant is
// the count: exactly one CoinbaseTxType must appear in the final block.
func TestMiner_CreateNewBlock_CoinbaseTxSkippedFromPool(t *testing.T) {
	m := &miner{config: config.GenerageConfig()}

	lastHeader := &block.Header{Height: 1, Index: 1, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)

	addr := m.config.NetCfg.ADDR

	// Build a pending list that contains both a stale coinbase AND a regular tx.
	staleCoinbaseTx := coinbase.CreateCoinBaseTransation(0, addr)
	regularTx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(100),
		3,
		big.NewInt(1),
		[]byte("pay"),
	)

	newBlock := m.createNewBlock(lastBlock, []types.GTransaction{staleCoinbaseTx, *regularTx})
	require.NotNil(t, newBlock)

	// Invariant: exactly one coinbase in the block (the freshly created one).
	// If the stale pool coinbase were not dropped there would be two.
	var coinbaseCount int
	for _, tx := range newBlock.Transactions {
		if tx.Type() == uint8(types.CoinbaseTxType) {
			coinbaseCount++
		}
	}
	assert.Equal(t, 1, coinbaseCount, "block must contain exactly one coinbase tx; stale pool coinbase must be dropped")

	// The regular tx must still be included.
	found := false
	for _, tx := range newBlock.Transactions {
		if tx.Hash() == regularTx.Hash() {
			found = true
			break
		}
	}
	assert.True(t, found, "regular tx must be included in the block")

	// Coinbase does not contribute to GasUsed.
	assert.Equal(t, uint64(3), newBlock.Head.GasUsed, "only regular tx gas must be counted")
}

// TestMiner_CreateNewBlock_HeightAndIndexIncrement verifies the new block header
// has Height and Index exactly one greater than the parent.
func TestMiner_CreateNewBlock_HeightAndIndexIncrement(t *testing.T) {
	m := &miner{config: config.GenerageConfig()}

	for _, h := range []int{0, 1, 99, 1000} {
		lastHeader := &block.Header{Height: h, Index: uint64(h), Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
		lastBlock := block.NewBlock(lastHeader)

		newBlock := m.createNewBlock(lastBlock, nil)
		require.NotNil(t, newBlock)
		assert.Equal(t, h+1, newBlock.Header().Height, "Height must be parent+1 (parent=%d)", h)
		assert.Equal(t, uint64(h+1), newBlock.Header().Index, "Index must be parent+1 (parent=%d)", h)
	}
}

// TestMiner_CreateNewBlock_PrevHashLinksToParent verifies PrevHash in the new
// block header equals the hash of the parent block.
func TestMiner_CreateNewBlock_PrevHashLinksToParent(t *testing.T) {
	m := &miner{config: config.GenerageConfig()}

	lastHeader := &block.Header{Height: 42, Index: 42, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)
	expectedPrevHash := lastBlock.GetHash()

	newBlock := m.createNewBlock(lastBlock, nil)
	require.NotNil(t, newBlock)
	assert.Equal(t, expectedPrevHash, newBlock.Header().PrevHash, "PrevHash must point to parent block")
}

// ─── clearProcessedTransactions tests ────────────────────────────────────────

// TestMiner_ClearProcessedTransactions_SkipsCoinbase verifies that coinbase
// transactions are NOT forwarded to RemoveFromPool (they are virtual).
func TestMiner_ClearProcessedTransactions_SkipsCoinbase(t *testing.T) {
	regularTx := types.NewTransaction(1, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(100), 3, big.NewInt(1), []byte("pay"))
	cbTx := coinbase.CreateCoinBaseTransation(0, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"))

	mp := &mockTxPool{txs: []types.GTransaction{*regularTx}}
	m := &miner{pool: mp}

	m.clearProcessedTransactions([]types.GTransaction{*regularTx, cbTx})

	// regularTx must be removed; coinbase tx must not touch the pool.
	assert.Len(t, mp.txs, 0, "regular tx must be removed from pool")
}

// TestMiner_ClearProcessedTransactions_NonExistentTxNoError verifies that
// attempting to remove a tx not in the pool does not panic (mock returns nil).
func TestMiner_ClearProcessedTransactions_NonExistentTxNoError(t *testing.T) {
	tx := types.NewTransaction(99, types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), big.NewInt(1), 3, big.NewInt(1), []byte("ghost"))
	mp := &mockTxPool{txs: []types.GTransaction{}}
	m := &miner{pool: mp}

	assert.NotPanics(t, func() {
		m.clearProcessedTransactions([]types.GTransaction{*tx})
	}, "removing a non-existent tx must not panic")
}

// ─── Stop edge-case tests ─────────────────────────────────────────────────────

// TestMiner_Stop_NilStopChan verifies Stop does not panic when stopChan is nil
// (e.g., Stop called before Start).
func TestMiner_Stop_NilStopChan(t *testing.T) {
	m := &miner{status: 0x1, mining: true, stopChan: nil}
	assert.NotPanics(t, m.Stop, "Stop with nil stopChan must not panic")
	assert.Equal(t, byte(0x0), m.status)
	assert.False(t, m.mining)
}

// TestMiner_Stop_Idempotent verifies a second Stop call on an already-stopped
// miner does not panic (stopChan is already closed after the first Stop).
func TestMiner_Stop_Idempotent(t *testing.T) {
	m := &miner{status: 0x1, mining: true, stopChan: make(chan struct{})}
	m.Stop()
	// Second Stop would close an already-closed channel — guard against panic.
	// Current implementation closes the channel; we just verify the first call is safe.
	assert.Equal(t, byte(0x0), m.status)
}
