package pool

import (
	"math/big"
	"testing"

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

func TestTransactionConsuming(t *testing.T) {
	tPool := InitPool(uint64(minGas), maxCap)
	tPool.AddTransaction(testTx1.From(), testTx1)
	info := tPool.GetInfo()
	if len(info.Txs) != 1 {
		t.Errorf("Different pool size, have %d, want %d", len(info.Txs), 1)
	}
	tPool.Clear()
	info = tPool.GetInfo()
	if info.Size != 0 {
		t.Errorf("Different pool size, have %d, want %d", info.Size, 0)
	}
}

func TestGetTx(t *testing.T) {
	tPool := InitPool(uint64(minGas), maxCap)
	tPool.AddTransaction(testTx1.From(), testTx1)
	tPool.AddRawTransaction(testTx2)
	tPool.AddRawTransaction(testTx3)
	info := tPool.GetInfo()
	if len(info.Txs) != 2 {
		t.Errorf("Different pool size, have %d, want %d", len(info.Txs), 3)
	}
}

func TestUtilityMethods(t *testing.T) {
	tPool := InitPool(uint64(minGas), maxCap)
	if tPool.GetMinimalGasValue() != uint64(minGas) {
		t.Errorf("Diffenrent minimum gas value! Have %d, want %d", tPool.GetMinimalGasValue(), minGas)
	}
}
