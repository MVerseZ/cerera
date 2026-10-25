package storage

import (
	"testing"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func TestComputeStateRoot_ExcludesNodeAccount(t *testing.T) {
	v := &D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.EmptyHash(),
	}
	addrA := types.HexToAddress("0x1000000000000000000000000000000000000000000000000000000000000001")
	addrB := types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002")

	accA := account.NewStateAccount(addrA, 10, common.EmptyHash())
	accB := account.NewStateAccount(addrB, 20, common.EmptyHash())
	accB.Status = 3 // OP_ACC_NODE

	v.Put(addrA, accA)
	v.Put(addrB, accB)

	rootWithNode := v.ComputeStateRoot()
	accB.Status = 0
	v.Put(addrB, accB)
	rootWithoutNode := v.ComputeStateRoot()

	if rootWithNode == rootWithoutNode {
		t.Fatal("node account should be excluded from consensus state root")
	}
	if rootWithoutNode == (common.Hash{}) {
		t.Fatal("expected non-empty root with two consensus accounts")
	}
}

func TestEnsureAccount_ExcludedFromStateRoot(t *testing.T) {
	v := &D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.EmptyHash(),
	}
	addr := types.HexToAddress("0x3000000000000000000000000000000000000000000000000000000000000003")

	rootBefore := v.ComputeStateRoot()
	v.EnsureAccount(addr)
	rootAfter := v.ComputeStateRoot()

	if rootBefore != rootAfter {
		t.Fatal("EnsureAccount must not change consensus state root")
	}
	acc := v.accounts.GetAccount(addr)
	if acc == nil || acc.Status != 3 {
		t.Fatalf("EnsureAccount status = %d, want OP_ACC_NODE (3)", func() byte {
			if acc == nil {
				return 255
			}
			return acc.Status
		}())
	}
}
