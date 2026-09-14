package storage

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/account"
	"github.com/cerera/core/types"
)

func closeTestVault(t *testing.T, v *D5Vault) {
	t.Helper()
	if v == nil {
		return
	}
	if err := v.Close(); err != nil {
		t.Logf("Warning: failed to close vault in test cleanup: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}

func newDiskTestVault(t *testing.T, vaultPath string) *D5Vault {
	t.Helper()
	os.RemoveAll(vaultPath)
	privateKey, _ := types.GenerateAccount()
	cfg := &config.Config{
		Vault: config.VaultConfig{PATH: vaultPath},
		IN_MEM: false,
		NetCfg: config.NetworkConfig{
			ADDR: types.PubkeyToAddress(&privateKey.PublicKey),
		},
	}
	vaultIface, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault() error = %v", err)
	}
	return vaultIface.(*D5Vault)
}

func createTestStateAccountForSource(balance float64) *account.StateAccount {
	privateKey, _ := types.GenerateAccount()
	pubkey := &privateKey.PublicKey
	address := types.PubkeyToAddress(pubkey)

	testStateAccount := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: address,
			Nonce:   1,
		},
		Status: 0,
	}
	testStateAccount.SetBalance(balance)
	return testStateAccount
}

func TestEncryptDecrypt(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		key  []byte
	}{
		{"simple_data", []byte("test data"), []byte("1234567890123456")},
		{"empty_data", []byte(""), []byte("1234567890123456")},
		{"long_data", []byte("this is a very long test data string that should be encrypted properly"), []byte("1234567890123456")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := encrypt(tt.data, tt.key)
			if err != nil {
				t.Fatalf("encrypt() error = %v", err)
			}
			if len(encrypted) <= len(tt.data) {
				t.Errorf("encrypt() encrypted length = %d, should be > %d", len(encrypted), len(tt.data))
			}
			decrypted, err := decrypt(encrypted, tt.key)
			if err != nil {
				t.Fatalf("decrypt() error = %v", err)
			}
			if !reflect.DeepEqual(decrypted, tt.data) {
				t.Errorf("decrypt() decrypted = %v, want %v", decrypted, tt.data)
			}
		})
	}
}

func TestInitSecureVault(t *testing.T) {
	vaultPath := filepath.Join(os.TempDir(), "test_vault_init")
	v := newDiskTestVault(t, vaultPath)
	defer func() {
		closeTestVault(t, v)
		os.RemoveAll(vaultPath)
	}()

	rootSa := createTestStateAccountForSource(100.0)
	if err := v.InitSecureVault(rootSa); err != nil {
		t.Fatalf("InitSecureVault() on new address should succeed: %v", err)
	}
	if err := v.InitSecureVault(rootSa); err == nil {
		t.Fatal("InitSecureVault() duplicate address should fail")
	}
}

func TestPersistAndSyncFromDB(t *testing.T) {
	vaultPath := filepath.Join(os.TempDir(), "test_vault_sync")
	v := newDiskTestVault(t, vaultPath)
	defer func() {
		closeTestVault(t, v)
		os.RemoveAll(vaultPath)
	}()

	account2 := createTestStateAccountForSource(200.0)
	if err := v.SaveAccountBytes(account2.Bytes()); err != nil {
		t.Fatalf("SaveAccountBytes() error = %v", err)
	}

	v.accounts.Clear()
	if err := v.SyncFromDB(); err != nil {
		t.Fatalf("SyncFromDB() error = %v", err)
	}
	if v.accounts.GetCount() == 0 {
		t.Fatal("SyncFromDB() should load persisted accounts")
	}
}

func TestVaultSourceSize(t *testing.T) {
	vaultPath := filepath.Join(os.TempDir(), "test_vault_size")
	v := newDiskTestVault(t, vaultPath)
	defer func() {
		closeTestVault(t, v)
		os.RemoveAll(vaultPath)
	}()

	size, err := v.VaultSourceSize()
	if err != nil {
		t.Fatalf("VaultSourceSize() error = %v", err)
	}
	if size <= 0 {
		t.Errorf("VaultSourceSize() size = %d, should be > 0", size)
	}

	account2 := createTestStateAccountForSource(50.0)
	if err := v.SaveAccountBytes(account2.Bytes()); err != nil {
		t.Fatalf("SaveAccountBytes() error = %v", err)
	}
	newSize, err := v.VaultSourceSize()
	if err != nil {
		t.Fatalf("VaultSourceSize() error = %v", err)
	}
	if newSize <= size {
		t.Errorf("VaultSourceSize() new size = %d, should be > %d", newSize, size)
	}
}

func TestEncodeDecodeAccountPayload(t *testing.T) {
	plain := createTestStateAccountForSource(10).Bytes()
	encoded, err := encodeAccountPayload(plain)
	if err != nil {
		t.Fatalf("encodeAccountPayload: %v", err)
	}
	decoded, err := decodeAccountPayload(encoded)
	if err != nil {
		t.Fatalf("decodeAccountPayload: %v", err)
	}
	if !bytesEqualPrefix(decoded, plain) {
		t.Fatal("decoded payload mismatch")
	}
}

func bytesEqualPrefix(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWalletRestoreRoundTrip(t *testing.T) {
	t.Skip("wallet restore round-trip is not supported in current account model")
}

func TestWalletRestoreAfterPersist(t *testing.T) {
	t.Skip("wallet restore after persist is not supported in current account model")
}

