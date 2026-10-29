package pool

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
)

func TestAddLocalRawTransaction_Broadcasts(t *testing.T) {
	p, err := InitPool(100)
	if err != nil {
		t.Fatalf("InitPool: %v", err)
	}
	pool := p.(*Pool)
	pool.SetTxValidator(func(*types.GTransaction) bool { return true })

	var broadcasted int
	pool.SetTxBroadcaster(func(*types.GTransaction) {
		broadcasted++
	})

	tx, err := types.CreateUnbroadcastTransaction(1, types.HexToAddress("0x2"), 1, 21000, big.NewInt(1), "test")
	if err != nil {
		t.Fatalf("CreateUnbroadcastTransaction: %v", err)
	}

	if err := pool.AddRawTransaction(tx); err != nil {
		t.Fatalf("AddRawTransaction: %v", err)
	}
	if broadcasted != 0 {
		t.Fatalf("AddRawTransaction must not broadcast, got %d", broadcasted)
	}

	tx2, err := types.CreateUnbroadcastTransaction(2, types.HexToAddress("0x2"), 1, 21000, big.NewInt(1), "test2")
	if err != nil {
		t.Fatalf("CreateUnbroadcastTransaction: %v", err)
	}
	if err := pool.AddLocalRawTransaction(tx2); err != nil {
		t.Fatalf("AddLocalRawTransaction: %v", err)
	}
	if broadcasted != 1 {
		t.Fatalf("AddLocalRawTransaction should broadcast once, got %d", broadcasted)
	}
}
