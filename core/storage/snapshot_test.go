package storage

import (
	"context"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

func TestRestoreBaseline_DoesNotMutateSavedSnapshot(t *testing.T) {
	cfg := &config.Config{
		IN_MEM: true,
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x34435Dd4078275B01D76A456FF9599fc07e61B90"),
		},
	}
	vaultIface, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault: %v", err)
	}
	v := vaultIface.(*D5Vault)
	addr := cfg.NetCfg.ADDR
	v.SaveBaseline()

	v.RestoreBaseline()
	if err := v.RewardMiner(addr, coinbase.BlockReward(), common.Hash{1}); err != nil {
		t.Fatalf("RewardMiner: %v", err)
	}

	v.RestoreBaseline()
	restored := v.Get(addr)
	if restored == nil {
		t.Fatal("account missing after second RestoreBaseline")
	}
	if restored.Status != 3 {
		t.Fatalf("baseline status = %d, want OP_ACC_NODE (3)", restored.Status)
	}
	if restored.GetBalanceBI().Sign() != 0 {
		t.Fatalf("baseline balance = %s, want 0", restored.GetBalanceBI())
	}
}

func TestRestoreAccounts_DeepClonesSnapshot(t *testing.T) {
	v := &D5Vault{accounts: NewAccountIndex(), rootHash: common.EmptyHash()}
	addr := types.HexToAddress("0x1000000000000000000000000000000000000000000000000000000000000001")
	acc := account.NewStateAccount(addr, 1, common.EmptyHash())
	acc.Status = 3
	acc.SetBalance(0)
	snap := AccountSnapshot{addr: acc}

	v.RestoreAccounts(snap)
	live := v.Get(addr)
	if live == nil {
		t.Fatal("missing account")
	}
	live.Status = 0
	live.SetBalanceBI(coinbase.BlockReward())

	v.RestoreAccounts(snap)
	restored := v.Get(addr)
	if restored.Status != 3 || restored.GetBalanceBI().Sign() != 0 {
		t.Fatalf("snapshot source was mutated: status=%d balance=%s", restored.Status, restored.GetBalanceBI())
	}
}
