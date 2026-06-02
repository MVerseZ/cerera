package storage

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cerera/core/types"
)

func testTx(nonce byte) *types.GTransaction {
	b := make([]byte, 32)
	b[31] = nonce
	to := types.BytesToAddress(b)
	tx, err := types.CreateTransaction(uint64(nonce), to, 1.0, 21000, "m")
	if err != nil {
		panic(err)
	}
	return tx
}

func TestTxTable_Add_Get_UpdateIndex(t *testing.T) {
	InitTxTable()
	tab := GetTxTable()

	tx := testTx(1)
	h := tx.Hash()

	if tab.Get(h) != -1 {
		t.Fatalf("unknown tx want -1, got %d", tab.Get(h))
	}

	tab.Add(tx)
	if g := tab.Get(h); g != -1 {
		t.Fatalf("pending want -1, got %d", g)
	}

	tab.UpdateIndex(tx, 42)
	if g := tab.Get(h); g != 42 {
		t.Fatalf("after update want 42, got %d", g)
	}

	// Second Add same hash must not reset block index
	tab.Add(tx)
	if g := tab.Get(h); g != 42 {
		t.Fatalf("duplicate Add should keep index 42, got %d", g)
	}
}

func TestTxTable_UpdateIndex_without_Add(t *testing.T) {
	InitTxTable()
	tab := GetTxTable()
	tx := testTx(2)
	tab.UpdateIndex(tx, 99)
	if g := tab.Get(tx.Hash()); g != -1 {
		t.Fatalf("UpdateIndex without Add should not insert, got %d", g)
	}
}

func TestTxTable_Add_nil(t *testing.T) {
	InitTxTable()
	tab := GetTxTable()
	tab.Add(nil)
}

func TestTxTable_concurrent(t *testing.T) {
	InitTxTable()
	tab := GetTxTable()
	txs := make([]*types.GTransaction, 50)
	hashes := make([][32]byte, 50)
	to := types.BytesToAddress(make([]byte, 32))
	for i := 0; i < 50; i++ {
		var err error
		txs[i], err = types.CreateTransaction(uint64(i), to, 1.0, 21000, fmt.Sprintf("x-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		hashes[i] = txs[i].Hash()
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tab.Add(txs[i])
			_ = tab.Get(hashes[i])
			tab.UpdateIndex(txs[i], i)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 50; i++ {
		if tab.Get(hashes[i]) != i {
			t.Fatalf("tx %d: want block index %d, got %d", i, i, tab.Get(hashes[i]))
		}
	}
}
