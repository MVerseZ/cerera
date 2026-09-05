package storage

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/types"
)

func TestVaultSyncContractExportApply(t *testing.T) {
	cfg := &config.Config{
		Vault: config.VaultConfig{
			MEM:  false,
			PATH: filepath.Join(os.TempDir(), "test_vault_sync"),
		},
		IN_MEM: false,
	}
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	d5 := vault.(*D5Vault)
	priv, _ := types.GenerateAccount()
	addr := types.PubkeyToAddress(&priv.PublicKey)
	code := []byte{0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3}
	if err := d5.StoreContractCode(addr, code); err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	key := big.NewInt(42)
	value := big.NewInt(100)
	if err := d5.SetStorage(addr, key, value); err != nil {
		t.Fatalf("SetStorage failed: %v", err)
	}

	stats := d5.VaultSyncStats()
	if stats.ContractCodes != 1 {
		t.Fatalf("ContractCodes = %d, want 1", stats.ContractCodes)
	}
	if stats.ContractSlots != 1 {
		t.Fatalf("ContractSlots = %d, want 1", stats.ContractSlots)
	}

	codeBlobs, total := d5.ExportContractCodeRange(0, 10)
	if total != 1 || len(codeBlobs) != 1 {
		t.Fatalf("ExportContractCodeRange = %d blobs total %d, want 1/1", len(codeBlobs), total)
	}

	storageBlobs, total := d5.ExportContractStorageRange(0, 10)
	if total != 1 || len(storageBlobs) != 1 {
		t.Fatalf("ExportContractStorageRange = %d blobs total %d, want 1/1", len(storageBlobs), total)
	}

	// Second vault receives peer blobs.
	cfg2 := &config.Config{
		Vault: config.VaultConfig{
			MEM:  false,
			PATH: filepath.Join(os.TempDir(), "test_vault_sync_peer"),
		},
		IN_MEM: false,
	}
	os.RemoveAll(cfg2.Vault.PATH)
	peerVault, err := NewD5Vault(context.Background(), cfg2)
	if err != nil {
		t.Fatalf("peer NewD5Vault failed: %v", err)
	}
	defer func() {
		peerVault.Close()
		os.RemoveAll(cfg2.Vault.PATH)
	}()
	peer := peerVault.(*D5Vault)

	if err := peer.SyncContractCodeBlob(codeBlobs[0]); err != nil {
		t.Fatalf("SyncContractCodeBlob failed: %v", err)
	}
	if err := peer.SyncContractStorageBlob(storageBlobs[0]); err != nil {
		t.Fatalf("SyncContractStorageBlob failed: %v", err)
	}

	gotCode, err := peer.GetContractCode(addr)
	if err != nil {
		t.Fatalf("GetContractCode failed: %v", err)
	}
	if string(gotCode) != string(code) {
		t.Fatalf("synced code mismatch: got %v want %v", gotCode, code)
	}

	gotValue, err := peer.GetStorage(addr, key)
	if err != nil {
		t.Fatalf("GetStorage failed: %v", err)
	}
	if gotValue.Cmp(value) != 0 {
		t.Fatalf("synced storage value = %s, want %s", gotValue, value)
	}

	peerStats := peer.VaultSyncStats()
	if peerStats.ContractCodes != 1 || peerStats.ContractSlots != 1 {
		t.Fatalf("peer stats = %+v, want 1 code and 1 slot", peerStats)
	}
}

func TestEncodeDecodeContractBlobs(t *testing.T) {
	priv, _ := types.GenerateAccount()
	addr := types.PubkeyToAddress(&priv.PublicKey)
	code := []byte{1, 2, 3, 4}

	blob, err := EncodeContractCodeBlob(addr, code)
	if err != nil {
		t.Fatalf("EncodeContractCodeBlob: %v", err)
	}
	gotAddr, gotCode, err := DecodeContractCodeBlob(blob)
	if err != nil {
		t.Fatalf("DecodeContractCodeBlob: %v", err)
	}
	if gotAddr != addr || string(gotCode) != string(code) {
		t.Fatalf("roundtrip mismatch addr=%s code=%v", gotAddr, gotCode)
	}

	key := big.NewInt(7)
	value := big.NewInt(99)
	storageBlob := EncodeContractStorageBlob(addr, key, value)
	gotAddr2, gotKey, gotValue, err := DecodeContractStorageBlob(storageBlob)
	if err != nil {
		t.Fatalf("DecodeContractStorageBlob: %v", err)
	}
	if gotAddr2 != addr || gotKey.Cmp(key) != 0 || gotValue.Cmp(value) != 0 {
		t.Fatalf("storage roundtrip mismatch")
	}
}

func TestLoadContractsFromDB(t *testing.T) {
	cfg := &config.Config{
		Vault: config.VaultConfig{
			MEM:  false,
			PATH: filepath.Join(os.TempDir(), "test_vault_reload_contracts"),
		},
		IN_MEM: false,
	}
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	d5 := vault.(*D5Vault)
	priv, _ := types.GenerateAccount()
	addr := types.PubkeyToAddress(&priv.PublicKey)
	code := []byte{0xde, 0xad, 0xbe, 0xef}
	if err := d5.StoreContractCode(addr, code); err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}
	if err := d5.SetStorage(addr, big.NewInt(1), big.NewInt(2)); err != nil {
		t.Fatalf("SetStorage failed: %v", err)
	}
	if err := d5.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reloaded, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("reload NewD5Vault failed: %v", err)
	}
	defer func() {
		reloaded.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	rd5 := reloaded.(*D5Vault)
	if err := rd5.SyncFromDB(); err != nil {
		t.Fatalf("SyncFromDB failed: %v", err)
	}
	if !rd5.HasContractCode(addr) {
		t.Fatal("expected contract code loaded into memory")
	}
	got, err := rd5.GetStorage(addr, big.NewInt(1))
	if err != nil || got.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("expected storage slot loaded, got %v err=%v", got, err)
	}
}
