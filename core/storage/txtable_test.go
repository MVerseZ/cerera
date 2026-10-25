package storage

import (
	"testing"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func testTx(t *testing.T) *types.GTransaction {
	t.Helper()
	privateKey, err := types.GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount: %v", err)
	}
	from := types.PubkeyToAddress(&privateKey.PublicKey)
	to := from
	tx, err := types.CreateUnbroadcastTransaction(1, to, 1.0, 21000, common.FloatToBigInt(1), "test")
	if err != nil {
		t.Fatalf("CreateUnbroadcastTransaction: %v", err)
	}
	return tx
}

func TestTxTableAddGetUpdateReset(t *testing.T) {
	tab := NewTxTable()
	tx := testTx(t)
	tab.Add(tx)
	if tab.Get(tx.Hash()) != -1 {
		t.Fatalf("expected pending index -1")
	}
	tab.UpdateIndex(tx, 3)
	if tab.Get(tx.Hash()) != 3 {
		t.Fatalf("expected index 3")
	}
	tab.Reset()
	if tab.Get(tx.Hash()) != -1 {
		t.Fatalf("expected missing tx after reset")
	}
}

func TestTxTableNilTx(t *testing.T) {
	tab := NewTxTable()
	tab.Add(nil)
	tab.UpdateIndex(nil, 1)
	if tab.Get(common.Hash{}) != -1 {
		t.Fatal("expected -1 for unknown hash")
	}
}
