package storage

import (
	"context"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func TestFaucetMintJournalReplication(t *testing.T) {
	cfg := &config.Config{
		IN_MEM: true,
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x34435Dd4078275B01D76A456FF9599fc07e61B90"),
		},
	}
	makeVault := func() *D5Vault {
		vaultIface, err := NewD5Vault(context.Background(), cfg)
		if err != nil {
			t.Fatalf("NewD5Vault: %v", err)
		}
		v := vaultIface.(*D5Vault)
		v.SaveBaseline()
		return v
	}

	vA := makeVault()
	vB := makeVault()

	to := types.HexToAddress("0x1000000000000000000000000000000000000000000000000000000000000001")
	amount := types.FloatToBigInt(10)
	hash := common.BytesToHash([]byte("faucet_repl_test"))

	if err := vA.DropFaucet(to, amount, hash); err != nil {
		t.Fatalf("DropFaucet: %v", err)
	}

	wire := EncodeFaucetMintWire(to, amount, hash)
	added, err := vB.ImportFaucetMintWire(wire)
	if err != nil || !added {
		t.Fatalf("ImportFaucetMintWire: added=%v err=%v", added, err)
	}

	vA.RestoreBaseline()
	vB.RestoreBaseline()
	if err := vA.ReplayFaucetMints(); err != nil {
		t.Fatalf("ReplayFaucetMints A: %v", err)
	}
	if err := vB.ReplayFaucetMints(); err != nil {
		t.Fatalf("ReplayFaucetMints B: %v", err)
	}

	fpA := vA.AccountsFingerprint()
	fpB := vB.AccountsFingerprint()
	if fpA != fpB {
		t.Fatalf("fingerprints differ: A=%s B=%s", fpA.Hex(), fpB.Hex())
	}
	if vA.ComputeStateRoot() != vB.ComputeStateRoot() {
		t.Fatal("state roots differ after shared faucet journal")
	}
}
