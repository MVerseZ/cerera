package miner

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/observer"
	"github.com/cerera/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCtx = context.Background()

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

func (m *mockTxPool) GetRawMemPool() []any {
	result := make([]any, len(m.txs))
	for i, tx := range m.txs {
		result[i] = tx
	}
	return result
}

func (m *mockTxPool) QueueTransaction(tx *types.GTransaction) {
	m.txs = append(m.txs, *tx)
}

func (m *mockTxPool) ServiceName() string { return "POOL_CERERA_001_1_3" }

func (m *mockTxPool) Register(observer.Observer) {}

func (m *mockTxPool) RegisterSource(observer.Source) {}

func (m *mockTxPool) RequeueTransactions([]types.GTransaction) {}

func (m *mockTxPool) SetTxValidator(func(*types.GTransaction) bool) {}

func (m *mockTxPool) Methods() map[string]service.RPCHandler { return nil }

func (m *mockTxPool) GetMiningPackage(n int) []*types.GTransaction {
	return m.GetTopN(n)
}

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

func testMinerAddr() types.Address {
	return types.HexToAddress("0x34435Dd4078275B01D76A456FF9599fc07e61B90")
}

func testMinerConfig() *config.Config {
	return &config.Config{
		IN_MEM: true,
		Chain:  config.ChainConfig{ChainID: 11, Path: "EMPTY"},
		NetCfg: config.NetworkConfig{ADDR: testMinerAddr()},
	}
}

func testTx(nonce uint64, gas uint64, data string) *types.GTransaction {
	return types.NewTransaction(
		nonce,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		gas,
		big.NewInt(1),
		[]byte(data),
	)
}

func blockTask(prev *block.Block, txs ...types.GTransaction) *Task {
	ptrs := make([]*types.GTransaction, len(txs))
	for i := range txs {
		ptrs[i] = &txs[i]
	}
	return &Task{id: testMinerAddr(), chain: 11, prev: prev, txs: ptrs}
}

func TestMiner_Init(t *testing.T) {
	m, err := Init(testCtx, testMinerConfig())
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
		name   string
		status byte
		mining bool
	}{
		{name: "Active miner accepts transaction", status: 0x1, mining: true},
		{name: "Inactive miner ignores transaction", status: 0x0, mining: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &miner{
				status: tt.status,
				mining: tt.mining,
			}
			assert.NotPanics(t, func() {
				m.Update(testTx(1, 21000, "test"))
			})
		})
	}
}

func TestMiner_CreateNewBlock(t *testing.T) {
	w := NewWorker()

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
	tx := testTx(1, 21000, "test")

	newBlock := w.createNewBlock(blockTask(lastBlock, *tx))
	require.NotNil(t, newBlock)
	assert.Equal(t, 101, newBlock.Header().Height)
	assert.Equal(t, uint64(101), newBlock.Header().Index)
	assert.Equal(t, lastBlock.GetHash(), newBlock.Header().PrevHash)
	assert.Greater(t, len(newBlock.Transactions), 0)

	assert.Nil(t, w.createNewBlock(nil))
	assert.Nil(t, w.createNewBlock(&Task{id: testMinerAddr(), prev: nil}))

	invalidBlock := block.NewBlock(nil)
	assert.Nil(t, w.createNewBlock(blockTask(invalidBlock, *tx)))
}

func TestMiner_ClearProcessedTransactions(t *testing.T) {
	tx1 := testTx(1, 21000, "test1")
	tx2 := testTx(2, 21000, "test2")
	mockPool := &mockTxPool{txs: []types.GTransaction{*tx1, *tx2}}
	m := &miner{pool: mockPool}

	initialCount := len(mockPool.txs)
	m.clearProcessedTransactions([]types.GTransaction{*tx1, *tx2})
	assert.Less(t, len(mockPool.txs), initialCount)
}

func TestMiner_Start_AndStop(t *testing.T) {
	cfg := testMinerConfig()
	bc, err := chain.Mold(cfg)
	require.NoError(t, err)

	m, err := Init(testCtx, cfg)
	require.NoError(t, err)
	m.SetChain(bc)
	m.SetPool(&mockTxPool{})

	require.NoError(t, m.Start())
	assert.Equal(t, byte(0x1), m.Status())
	assert.True(t, m.mining)

	time.Sleep(50 * time.Millisecond)
	m.Stop()
	assert.Equal(t, byte(0x0), m.Status())
	assert.False(t, m.mining)
}

func TestMockTxPool_GetTopN_Empty(t *testing.T) {
	mp := &mockTxPool{}
	assert.Nil(t, mp.GetTopN(5), "GetTopN on empty mock must return nil")
}

func TestMockTxPool_GetTopN_NGreaterThanSize(t *testing.T) {
	tx1 := testTx(1, 3, "a")
	tx2 := testTx(2, 3, "b")
	mp := &mockTxPool{txs: []types.GTransaction{*tx1, *tx2}}

	top := mp.GetTopN(100)
	require.Len(t, top, 2)
	assert.Equal(t, tx1.Hash(), top[0].Hash())
	assert.Equal(t, tx2.Hash(), top[1].Hash())
}

func TestMockTxPool_GetTopN_ExactN(t *testing.T) {
	txs := make([]types.GTransaction, 5)
	for i := range txs {
		txs[i] = *testTx(uint64(i+1), 3, "x")
	}
	mp := &mockTxPool{txs: txs}
	require.Len(t, mp.GetTopN(3), 3)
}

func TestMiner_CreateNewBlock_GasLimitEnforcement(t *testing.T) {
	w := NewWorker()
	lastHeader := &block.Header{
		Height:     10,
		Index:      10,
		Difficulty: 1,
		GasLimit:   100,
		ChainId:    11,
	}
	lastBlock := block.NewBlock(lastHeader)

	tx1 := types.NewTransaction(1, testMinerAddr(), big.NewInt(1), 60, big.NewInt(1), []byte("a"))
	tx2 := types.NewTransaction(2, testMinerAddr(), big.NewInt(1), 60, big.NewInt(1), []byte("b"))
	tx3 := types.NewTransaction(3, testMinerAddr(), big.NewInt(1), 10, big.NewInt(1), []byte("c"))

	newBlock := w.createNewBlock(blockTask(lastBlock, *tx1, *tx2, *tx3))
	require.NotNil(t, newBlock)

	txHashes := make(map[common.Hash]struct{}, len(newBlock.Transactions))
	for _, tx := range newBlock.Transactions {
		txHashes[tx.Hash()] = struct{}{}
	}
	assert.Contains(t, txHashes, tx1.Hash())
	assert.NotContains(t, txHashes, tx2.Hash())
	assert.Contains(t, txHashes, tx3.Hash())
	assert.Equal(t, uint64(70), newBlock.Head.GasUsed)
}

func TestMiner_CreateNewBlock_EmptyTransactions(t *testing.T) {
	w := NewWorker()
	lastHeader := &block.Header{Height: 5, Index: 5, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)

	newBlock := w.createNewBlock(blockTask(lastBlock))
	require.NotNil(t, newBlock)
	require.Len(t, newBlock.Transactions, 1)
	assert.Equal(t, uint8(types.CoinbaseTxType), newBlock.Transactions[0].Type())
}

func TestMiner_CreateNewBlock_CoinbaseTxSkippedFromPool(t *testing.T) {
	w := NewWorker()
	lastHeader := &block.Header{Height: 1, Index: 1, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)

	staleCoinbaseTx := coinbase.CreateCoinBaseTransation(0, testMinerAddr())
	regularTx := types.NewTransaction(1, testMinerAddr(), big.NewInt(100), 3, big.NewInt(1), []byte("pay"))

	newBlock := w.createNewBlock(blockTask(lastBlock, staleCoinbaseTx, *regularTx))
	require.NotNil(t, newBlock)

	var coinbaseCount int
	found := false
	for _, tx := range newBlock.Transactions {
		if tx.Type() == uint8(types.CoinbaseTxType) {
			coinbaseCount++
		}
		if tx.Hash() == regularTx.Hash() {
			found = true
		}
	}
	assert.Equal(t, 1, coinbaseCount)
	assert.True(t, found)
	assert.Equal(t, uint64(3), newBlock.Head.GasUsed)
}

func TestMiner_CreateNewBlock_HeightAndIndexIncrement(t *testing.T) {
	w := NewWorker()
	for _, h := range []int{0, 1, 99, 1000} {
		lastHeader := &block.Header{Height: h, Index: uint64(h), Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
		lastBlock := block.NewBlock(lastHeader)
		newBlock := w.createNewBlock(blockTask(lastBlock))
		require.NotNil(t, newBlock)
		assert.Equal(t, h+1, newBlock.Header().Height)
		assert.Equal(t, uint64(h+1), newBlock.Header().Index)
	}
}

func TestMiner_CreateNewBlock_PrevHashLinksToParent(t *testing.T) {
	w := NewWorker()
	lastHeader := &block.Header{Height: 42, Index: 42, Difficulty: 1, GasLimit: 1_000_000, ChainId: 11}
	lastBlock := block.NewBlock(lastHeader)
	expectedPrevHash := lastBlock.GetHash()

	newBlock := w.createNewBlock(blockTask(lastBlock))
	require.NotNil(t, newBlock)
	assert.Equal(t, expectedPrevHash, newBlock.Header().PrevHash)
}

func TestMiner_ClearProcessedTransactions_SkipsCoinbase(t *testing.T) {
	regularTx := testTx(1, 3, "pay")
	cbTx := coinbase.CreateCoinBaseTransation(0, testMinerAddr())
	mp := &mockTxPool{txs: []types.GTransaction{*regularTx}}
	m := &miner{pool: mp}

	m.clearProcessedTransactions([]types.GTransaction{*regularTx, cbTx})
	assert.Len(t, mp.txs, 0)
}

func TestMiner_ClearProcessedTransactions_NonExistentTxNoError(t *testing.T) {
	tx := testTx(99, 3, "ghost")
	m := &miner{pool: &mockTxPool{}}
	assert.NotPanics(t, func() {
		m.clearProcessedTransactions([]types.GTransaction{*tx})
	})
}

func TestMiner_Stop_NilStopChan(t *testing.T) {
	m := &miner{status: 0x1, mining: true, stopChan: nil}
	assert.NotPanics(t, m.Stop)
	assert.Equal(t, byte(0x0), m.status)
	assert.False(t, m.mining)
}

func TestMiner_Stop_Idempotent(t *testing.T) {
	m := &miner{status: 0x1, mining: true, stopChan: make(chan struct{})}
	m.Stop()
	assert.Equal(t, byte(0x0), m.status)
}
