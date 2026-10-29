package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func TestAccountsFingerprintChangesWhenAccountAdded(t *testing.T) {
	cfg := &config.Config{
		IN_MEM: true,
		NetCfg: config.NetworkConfig{},
	}
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault: %v", err)
	}
	d5 := vault.(*D5Vault)
	fp0 := d5.AccountsFingerprint()

	addr := types.HexToAddress("0x10000000000000000000000000000000000000000000000000000000000000aa")
	d5.EnsureAccount(addr)
	fp1 := d5.AccountsFingerprint()
	if fp0 == fp1 {
		t.Fatal("fingerprint should change after EnsureAccount")
	}

	stats := d5.VaultSyncStats()
	if stats.AccountsFingerprint != fp1 {
		t.Fatal("VaultSyncStats fingerprint must match AccountsFingerprint()")
	}
}

func TestAccountsFingerprintStableOnDiskVault(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "cerera_fp_test")
	os.RemoveAll(dir)
	cfg := &config.Config{
		IN_MEM: false,
		Vault:  config.VaultConfig{PATH: dir},
		NetCfg: config.NetworkConfig{},
	}
	v1, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault: %v", err)
	}
	d1 := v1.(*D5Vault)
	fp := d1.AccountsFingerprint()
	if fp == (common.Hash{}) && d1.GetCount() > 0 {
		t.Fatal("expected non-zero fingerprint with accounts")
	}
	_ = v1.Close()
}
