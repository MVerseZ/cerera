package storage

import (
	"context"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func TestReplayFaucetMints_AfterRestoreBaseline(t *testing.T) {
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
	v.SaveBaseline()

	to := types.HexToAddress("0x1000000000000000000000000000000000000000000000000000000000000001")
	amount := types.FloatToBigInt(10)
	if err := v.DropFaucet(to, amount, common.BytesToHash([]byte("faucet_test"))); err != nil {
		t.Fatalf("DropFaucet: %v", err)
	}
	if got := v.Get(to); got == nil || got.GetBalanceBI().Cmp(amount) != 0 {
		t.Fatalf("live faucet balance missing")
	}

	v.RestoreBaseline()
	if acc := v.Get(to); acc != nil && acc.GetBalanceBI().Sign() != 0 {
		t.Fatalf("baseline still has faucet credit: %s", acc.GetBalanceBI())
	}

	if err := v.ReplayFaucetMints(); err != nil {
		t.Fatalf("ReplayFaucetMints: %v", err)
	}
	restored := v.Get(to)
	if restored == nil {
		t.Fatal("account missing after ReplayFaucetMints")
	}
	if restored.GetBalanceBI().Cmp(amount) != 0 {
		t.Fatalf("replayed faucet balance = %s, want %s", restored.GetBalanceBI(), amount)
	}
}
