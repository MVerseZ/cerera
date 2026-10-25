package pool

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
)

func TestAddRawTransaction_RejectedByValidator(t *testing.T) {
	p, err := InitPool(100)
	if err != nil {
		t.Fatal(err)
	}
	pool := p.(*Pool)
	pool.SetTxValidator(func(*types.GTransaction) bool { return false })

	to := types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002")
	tx := types.NewTransaction(1, to, big.NewInt(1), 1000, big.NewInt(1), nil)
	if err := pool.AddRawTransaction(tx); err == nil {
		t.Fatal("expected validator rejection")
	}
}
