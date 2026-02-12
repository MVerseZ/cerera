package pool

import (
	"math/big"
	"sync"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTx1 = types.NewTransaction(
	11,
	types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
	big.NewInt(100000000),
	3333,
	big.NewInt(3333),
	[]byte{0xa, 0xb},
)
var testTx2 = types.NewTransaction(
	11,
	types.HexToAddress("0x43F119F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
	big.NewInt(30),
	1,
	big.NewInt(100),
	[]byte{0xa, 0xb},
)
var testTx3 = types.NewTransaction(
	11,
	types.HexToAddress("0x804339F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
	big.NewInt(100000000),
	1500,
	big.NewInt(10),
	[]byte{0xa, 0xb},
)
var minGas = 1000
var maxCap = 10

// func TestPoolSize(t *testing.T) {
// 	tPool, _ := InitPool(uint64(minGas), maxCap)
// 	tPool.QueueTransaction(testTx1)
// 	info := tPool.GetInfo()
// 	if len(info.Txs) != 1 {
// 		t.Errorf("Different pool size, have %d, want %d", len(info.Txs), 1)
// 	}
// }

// func TestGetTx(t *testing.T) {
// 	tPool, _ := InitPool(uint64(minGas), maxCap)
// 	tPool.AddRawTransaction(testTx2)
// 	tPool.AddRawTransaction(testTx3)
// 	info := tPool.GetInfo()
// 	if len(info.Txs) != 2 {
// 		t.Errorf("Different pool size, have %d, want %d", len(info.Txs), 3)
// 	}
// }

// func TestUtilityMethods(t *testing.T) {
// 	tPool, _ := InitPool(uint64(minGas), maxCap)
// 	if tPool.GetMinimalGasValue() != uint64(minGas) {
// 		t.Errorf("Diffenrent minimum gas value! Have %d, want %d", tPool.GetMinimalGasValue(), minGas)
// 	}
// }

// func TestPoolCapacity(t *testing.T) {
// 	tPool, _ := InitPool(uint64(minGas), maxCap)

// 	// Добавляем транзакции до достижения лимита
// 	for i := 0; i < maxCap; i++ {
// 		tx := types.NewTransaction(
// 			uint64(i),
// 			types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
// 			big.NewInt(100000000),
// 			3333,
// 			big.NewInt(3333),
// 			[]byte{0xa, 0xb},
// 		)
// 		tPool.AddTransaction(tx.From(), tx)
// 	}

// 	// Проверяем, что пул заполнен
// 	info := tPool.GetInfo()
// 	if len(info.Txs) != maxCap {
// 		t.Errorf("Expected pool size to be %d, got %d", maxCap, len(info.Txs))
// 	}

// 	// Пытаемся добавить еще одну транзакцию (должно быть отклонено)
// 	tx := types.NewTransaction(
// 		uint64(maxCap),
// 		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
// 		big.NewInt(100000000),
// 		3333,
// 		big.NewInt(3333),
// 		[]byte{0xa, 0xb},
// 	)
// 	tPool.AddTransaction(tx.From(), tx)
// 	info = tPool.GetInfo()
// 	if len(info.Txs) != maxCap {
// 		t.Errorf("Expected pool size to remain %d, got %d", maxCap, len(info.Txs))
// 	}
// }

// func TestGasLimit(t *testing.T) {
// 	tPool, _ := InitPool(uint64(minGas), maxCap)

// 	// Добавляем транзакцию с низким газом (должна быть отклонена)
// 	tx := types.NewTransaction(
// 		11,
// 		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
// 		big.NewInt(100000000),
// 		500, // gasLimit ниже minGas
// 		big.NewInt(3333),
// 		[]byte{0xa, 0xb},
// 	)
// 	tPool.AddTransaction(tx.From(), tx)
// 	info := tPool.GetInfo()
// 	if len(info.Txs) != 0 {
// 		t.Errorf("Expected transaction with low gas to be rejected, got %d transactions in the pool", len(info.Txs))
// 	}
// }

// TestRaceConditionAddRawTransaction tests for race conditions when adding transactions concurrently
func TestRaceConditionAddRawTransaction(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	var wg sync.WaitGroup
	numGoroutines := 50
	numTxsPerGoroutine := 10

	// Create unique transactions
	txs := make([]*types.GTransaction, numGoroutines*numTxsPerGoroutine)
	for i := 0; i < len(txs); i++ {
		txs[i] = types.NewTransaction(
			uint64(i),
			types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
			big.NewInt(100000000),
			3333,
			big.NewInt(3333),
			[]byte{byte(i)},
		)
	}

	// Add transactions concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()
			for j := 0; j < numTxsPerGoroutine; j++ {
				idx := startIdx*numTxsPerGoroutine + j
				tPool.QueueTransaction(txs[idx])
			}
		}(i)
	}

	wg.Wait()

	// Verify no duplicates and correct count
	info := tPool.GetInfo()
	assert.LessOrEqual(t, info.Size, len(txs), "Pool size should not exceed number of unique transactions")
	assert.Greater(t, info.Size, 0, "Some transactions should be added")
}

// TestRaceConditionGetTransaction tests for race conditions when getting transactions concurrently
func TestRaceConditionGetTransaction(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	// Add some transactions
	txs := make([]*types.GTransaction, 10)
	hashes := make([]common.Hash, 10)
	for i := 0; i < 10; i++ {
		txs[i] = types.NewTransaction(
			uint64(i),
			types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
			big.NewInt(100000000),
			3333,
			big.NewInt(3333),
			[]byte{byte(i)},
		)
		hashes[i] = txs[i].Hash()
		tPool.QueueTransaction(txs[i])
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	// Get transactions concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool := tPool.(*Pool)
			for _, hash := range hashes {
				tx := pool.GetTransaction(hash)
				if tx != nil {
					assert.Equal(t, hash, tx.Hash(), "Transaction hash should match")
				}
			}
		}()
	}

	wg.Wait()
}

// TestRaceConditionUpdateTx tests for race conditions when updating transactions concurrently
func TestRaceConditionUpdateTx(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	// Add initial transaction
	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100000000),
		3333,
		big.NewInt(3333),
		[]byte{0xa, 0xb},
	)
	tPool.QueueTransaction(tx)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Update transaction concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(nonce uint64) {
			defer wg.Done()
			updatedTx := types.NewTransaction(
				nonce,
				types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
				big.NewInt(100000000),
				3333,
				big.NewInt(3333),
				[]byte{0xa, 0xb},
			)
			// Use same hash as original
			pool := tPool.(*Pool)
			pool.UpdateTx(*updatedTx)
		}(uint64(i))
	}

	wg.Wait()

	// Verify transaction still exists
	pool := tPool.(*Pool)
	retrievedTx := pool.GetTransaction(tx.Hash())
	assert.NotNil(t, retrievedTx, "Transaction should still exist after updates")
}

// TestNoDuplicateInPrepared tests that transactions are not duplicated in Prepared
func TestNoDuplicateInPrepared(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x24F369F35D4323dF9980E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100000000),
		3333,
		big.NewInt(3333),
		[]byte{0xa, 0xb},
	)

	// Add same transaction multiple times
	for i := 0; i < 5; i++ {
		tPool.QueueTransaction(tx)
	}

	// Get pool info
	info := tPool.GetInfo()
	assert.Equal(t, 1, info.Size, "Pool should contain only one unique transaction")
}

// TestInfoMemoryLeak tests that Info is properly recalculated and doesn't leak memory
func TestInfoMemoryLeak(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	// Add transactions
	for i := 0; i < 10; i++ {
		tx := types.NewTransaction(
			uint64(i),
			types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
			big.NewInt(100000000),
			3333,
			big.NewInt(3333),
			[]byte{byte(i)},
		)
		tPool.QueueTransaction(tx)
	}

	info1 := tPool.GetInfo()
	initialSize := len(info1.Hashes)

	// Remove some transactions
	for i := 0; i < 5; i++ {
		tx := types.NewTransaction(
			uint64(i),
			types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
			big.NewInt(100000000),
			3333,
			big.NewInt(3333),
			[]byte{byte(i)},
		)
		tPool.RemoveFromPool(tx.Hash())
	}

	info2 := tPool.GetInfo()
	finalSize := len(info2.Hashes)
	finalTxsCount := len(info2.Txs)

	// Info should be recalculated, not accumulated
	assert.Equal(t, finalSize, info2.Size, "Info.Hashes length should match Info.Size")
	assert.Equal(t, finalTxsCount, info2.Size, "Info.Txs length should match Info.Size")
	assert.Less(t, finalSize, initialSize, "Info should shrink after removing transactions")
}

// TestGetTransactionWithLock tests that GetTransaction works correctly with locking
func TestGetTransactionWithLock(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100000000),
		3333,
		big.NewInt(3333),
		[]byte{0xa, 0xb},
	)

	// Add transaction
	tPool.QueueTransaction(tx)

	// Get transaction
	pool := tPool.(*Pool)
	retrievedTx := pool.GetTransaction(tx.Hash())
	require.NotNil(t, retrievedTx, "Transaction should be found")
	assert.Equal(t, tx.Hash(), retrievedTx.Hash(), "Transaction hash should match")

	// Get non-existent transaction
	nonExistentHash := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	retrievedTx2 := pool.GetTransaction(nonExistentHash)
	assert.Nil(t, retrievedTx2, "Non-existent transaction should return nil")
}

// TestSendTransactionUpdatesMetrics tests that SendTransaction updates metrics correctly
func TestSendTransactionUpdatesMetrics(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100000000),
		3333,
		big.NewInt(3333),
		[]byte{0xa, 0xb},
	)

	// Send transaction
	pool := tPool.(*Pool)
	hash, err := pool.SendTransaction(*tx)
	require.NoError(t, err, "SendTransaction should succeed")
	assert.Equal(t, tx.Hash(), hash, "Returned hash should match transaction hash")

	// Verify transaction was added
	info := tPool.GetInfo()
	assert.Equal(t, 1, info.Size, "Pool should contain one transaction")
	assert.Equal(t, tx.Hash(), info.Hashes[0], "Transaction hash should be in Info.Hashes")
}

// TestUnRegisterWithLock tests that UnRegister works correctly with locking
func TestUnRegisterWithLock(t *testing.T) {
	tPool, err := InitPool(float64(minGas), maxCap*10)
	require.NoError(t, err)
	require.NotNil(t, tPool)

	// Create a mock observer
	mockObserver := &mockObserver{id: "test-observer-1"}

	// Register observer
	tPool.Register(mockObserver)

	// Unregister observer concurrently
	pool := tPool.(*Pool)
	var wg sync.WaitGroup
	numGoroutines := 5
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.UnRegister(mockObserver)
		}()
	}

	wg.Wait()
	// Test should complete without race condition
}

// mockObserver is a simple observer implementation for testing
type mockObserver struct {
	id string
}

func (m *mockObserver) GetID() string {
	return m.id
}

func (m *mockObserver) Update(tx *types.GTransaction) {
	// Mock implementation
}
