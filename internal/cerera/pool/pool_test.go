package pool

import (
	"math/big"

	"github.com/cerera/internal/cerera/types"
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
