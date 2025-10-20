package storage

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

// Test helper functions
func createTestConfig() *config.Config {
	// Generate a valid private key for testing
	privateKey, _ := types.GenerateAccount()
	privKeyString := types.EncodePrivateKeyToToString(privateKey)

	return &config.Config{
		Vault: config.VaultConfig{
			MEM:  true,
			PATH: "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.PubkeyToAddress(privateKey.PublicKey),
			PRIV: privKeyString,
		},
		IN_MEM: true,
	}
}

func createTestStateAccount(name string, balance float64) *types.StateAccount {
	privateKey, _ := types.GenerateAccount()
	pubkey := &privateKey.PublicKey
	address := types.PubkeyToAddress(*pubkey)
	derBytes := types.EncodePrivateKeyToByte(privateKey)

	testStateAccount := &types.StateAccount{
		Address:  address,
		Nonce:    1,
		Root:     common.Hash{},
		CodeHash: derBytes,
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte("test_pass")),
	}
	testStateAccount.SetBalance(balance)
	return testStateAccount
}

// Test NewD5Vault
func TestNewD5Vault(t *testing.T) {
	// Initialize coinbase data first
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	if vault == nil {
		t.Fatal("NewD5Vault returned nil vault")
	}

	// Test that coinbase and faucet accounts are added
	if vault.GetCount() < 2 {
		t.Errorf("Expected at least 2 accounts (coinbase + faucet), got %d", vault.GetCount())
	}

	// Test that vault status is set correctly
	if vault.Status() != 0xa {
		t.Errorf("Expected vault status 0xa, got 0x%x", vault.Status())
	}
}

// Test Create method
func TestD5Vault_Create(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	tests := []struct {
		name     string
		pass     string
		wantErr  bool
		wantName string
	}{
		{"test_account", "password123", false, "test_account"},
		{"", "password123", false, ""}, // Empty name should use address
		{"another_account", "", false, "another_account"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masterKey, publicKey, mnemonic, address, err := vault.Create(tt.pass)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Create() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}

			if masterKey == "" {
				t.Error("Create() masterKey should not be empty")
			}

			if publicKey == "" {
				t.Error("Create() publicKey should not be empty")
			}

			if mnemonic == "" {
				t.Error("Create() mnemonic should not be empty")
			}

			if address == nil {
				t.Error("Create() address should not be nil")
			}

			// Verify the account was created
			if address != nil {
				account := vault.Get(*address)
				if account == nil {
					t.Error("Create() account should be retrievable after creation")
				} else {
					if account.GetBalance() != 0.0 {
						t.Errorf("Create() account balance = %v, want 0.0", account.GetBalance())
					}
				}
			}
		})
	}
}

// Test Restore method
func TestD5Vault_Restore(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// First create an account to restore
	_, _, mnemonic, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	tests := []struct {
		name        string
		mnemonic    string
		pass        string
		wantErr     bool
		expectEmpty bool
	}{
		{"valid_restore", mnemonic, "password123", false, false},
		{"empty_mnemonic", "", "password123", true, true},
		{"invalid_mnemonic", "invalid mnemonic phrase", "password123", true, true},
		{"wrong_password", mnemonic, "wrong_password", true, true}, // Should fail with wrong password
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, masterKey, publicKey, err := vault.Restore(tt.mnemonic, tt.pass)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Restore() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Restore() error = %v", err)
				return
			}

			if tt.expectEmpty {
				if !addr.IsEmpty() {
					t.Errorf("Restore() expected empty address, got %v", addr)
				}
				if masterKey != "" {
					t.Errorf("Restore() expected empty masterKey, got %v", masterKey)
				}
				if publicKey != "" {
					t.Errorf("Restore() expected empty publicKey, got %v", publicKey)
				}
			} else {
				if addr.IsEmpty() {
					t.Error("Restore() address should not be empty")
				}
				if masterKey == "" {
					t.Error("Restore() masterKey should not be empty")
				}
				if publicKey == "" {
					t.Error("Restore() publicKey should not be empty")
				}

				// For valid restore, verify we get the correct address
				if tt.name == "valid_restore" && addr != *address {
					t.Errorf("Restore() address = %v, want %v", addr, *address)
				}
			}
		})
	}
}

// Test Get method
func TestD5Vault_Get(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	_, _, _, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Test getting existing account
	account := vault.Get(*address)
	if account == nil {
		t.Error("Get() should return account for existing address")
	} else {
		if account.Address != *address {
			t.Errorf("Get() account name = %v, want test_get", account.Address)
		}
	}

	// Test getting non-existing account
	emptyAddr := types.EmptyAddress()
	account = vault.Get(emptyAddr)
	if account != nil {
		t.Error("Get() should return nil for non-existing address")
	}
}

// Test Put method
func TestD5Vault_Put(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	account := createTestStateAccount("test_put", 50.0)
	address := account.Address

	// Put the account
	vault.Put(address, account)

	// Verify it was stored
	retrieved := vault.Get(address)
	if retrieved == nil {
		t.Error("Put() account should be retrievable after Put")
	} else {
		if retrieved.Address != address {
			t.Errorf("Put() account name = %v, want test_put", retrieved.Address)
		}
		if retrieved.GetBalance() != 50.0 {
			t.Errorf("Put() account balance = %v, want 50.0", retrieved.GetBalance())
		}
	}
}

// Test GetAll method
func TestD5Vault_GetAll(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create some test accounts
	vault.Create("pass1")
	vault.Create("pass2")

	// Get all accounts
	all := vault.GetAll()
	if all == nil {
		t.Error("GetAll() should not return nil")
	}

	// Verify we have at least the accounts we created plus coinbase/faucet
	count := vault.GetCount()
	if count < 4 { // 2 created + coinbase + faucet
		t.Errorf("GetAll() expected at least 4 accounts, got %d", count)
	}
}

// Test GetCount method
func TestD5Vault_GetCount(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	initialCount := vault.GetCount()

	// Create a test account
	vault.Create("password123")

	// Verify count increased
	newCount := vault.GetCount()
	if newCount != initialCount+1 {
		t.Errorf("GetCount() after create = %d, want %d", newCount, initialCount+1)
	}
}

// Test UpdateBalance method
func TestD5Vault_UpdateBalance(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create two test accounts
	_, _, _, fromAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create from account: %v", err)
	}

	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create to account: %v", err)
	}

	// Get initial balances
	fromAccount := vault.Get(*fromAddr)
	toAccount := vault.Get(*toAddr)
	initialFromBalance := types.FloatToBigInt(fromAccount.GetBalance())
	initialToBalance := types.FloatToBigInt(toAccount.GetBalance())

	// Update balance
	transferAmount := types.FloatToBigInt(25.0)
	txHash := common.Hash{0x1, 0x2, 0x3}

	// Cast to D5Vault to access UpdateBalance method
	d5Vault := vault.(*D5Vault)
	d5Vault.UpdateBalance(*fromAddr, *toAddr, transferAmount, txHash)

	// Verify balances were updated
	fromAccount = vault.Get(*fromAddr)
	toAccount = vault.Get(*toAddr)

	expectedFromBalance := new(big.Int).Sub(initialFromBalance, transferAmount)
	expectedToBalance := new(big.Int).Add(initialToBalance, transferAmount)

	if fromAccount.GetBalance() != types.BigIntToFloat(expectedFromBalance) {
		t.Errorf("UpdateBalance() from balance = %v, want %v", fromAccount.GetBalance(), types.BigIntToFloat(expectedFromBalance))
	}

	if toAccount.GetBalance() != types.BigIntToFloat(expectedToBalance) {
		t.Errorf("UpdateBalance() to balance = %v, want %v", toAccount.GetBalance(), types.BigIntToFloat(expectedToBalance))
	}

	// Verify transaction was added to inputs
	toAccount.Inputs.RLock()
	defer toAccount.Inputs.RUnlock()
	if val, exists := toAccount.Inputs.M[txHash]; !exists || val.Cmp(transferAmount) != 0 {
		t.Errorf("UpdateBalance() transaction not properly recorded in inputs")
	}
}

// Test DropFaucet method
func TestD5Vault_DropFaucet(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Get initial balances
	faucetAccount := vault.Get(coinbase.GetFaucetAddress())
	toAccount := vault.Get(*toAddr)
	initialFaucetBalance := types.FloatToBigInt(faucetAccount.GetBalance())
	initialToBalance := types.FloatToBigInt(toAccount.GetBalance())

	// Drop faucet
	faucetAmount := types.FloatToBigInt(100.0)
	txHash := common.Hash{0x4, 0x5, 0x6}

	// Cast to D5Vault to access DropFaucet method
	d5Vault := vault.(*D5Vault)
	err = d5Vault.DropFaucet(*toAddr, faucetAmount, txHash)
	if err != nil {
		t.Errorf("DropFaucet() error = %v", err)
	}

	// Verify balances were updated
	faucetAccount = vault.Get(coinbase.GetFaucetAddress())
	toAccount = vault.Get(*toAddr)

	expectedFaucetBalance := new(big.Int).Sub(initialFaucetBalance, faucetAmount)
	expectedToBalance := new(big.Int).Add(initialToBalance, faucetAmount)

	if faucetAccount.GetBalance() != types.BigIntToFloat(expectedFaucetBalance) {
		t.Errorf("DropFaucet() faucet balance = %v, want %v", faucetAccount.GetBalance(), types.BigIntToFloat(expectedFaucetBalance))
	}

	if toAccount.GetBalance() != types.BigIntToFloat(expectedToBalance) {
		t.Errorf("DropFaucet() to balance = %v, want %v", toAccount.GetBalance(), types.BigIntToFloat(expectedToBalance))
	}

	// Verify transaction was added to inputs
	toAccount.Inputs.RLock()
	defer toAccount.Inputs.RUnlock()
	if val, exists := toAccount.Inputs.M[txHash]; !exists || val.Cmp(faucetAmount) != 0 {
		t.Errorf("DropFaucet() transaction not properly recorded in inputs")
	}
}

// Test DropFaucet method with limits
func TestD5Vault_DropFaucetWithLimits(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Cast to D5Vault to access DropFaucet method
	d5Vault := vault.(*D5Vault)
	txHash := common.Hash{0x4, 0x5, 0x6}

	t.Run("valid_faucet_request", func(t *testing.T) {
		// Test valid faucet request
		faucetAmount := types.FloatToBigInt(100.0)
		err = d5Vault.DropFaucet(*toAddr, faucetAmount, txHash)
		if err != nil {
			t.Errorf("DropFaucet() error = %v", err)
		}
	})

	t.Run("amount_below_minimum", func(t *testing.T) {
		// Test amount below minimum
		smallAmount := types.FloatToBigInt(0.5)
		err = d5Vault.DropFaucet(*toAddr, smallAmount, txHash)
		if err == nil {
			t.Error("DropFaucet() should fail for amount below minimum")
		}
		if !strings.Contains(err.Error(), "below minimum") {
			t.Errorf("DropFaucet() error message should contain 'below minimum', got: %v", err)
		}
	})

	t.Run("amount_above_maximum", func(t *testing.T) {
		// Test amount above maximum
		largeAmount := types.FloatToBigInt(2000.0)
		err = d5Vault.DropFaucet(*toAddr, largeAmount, txHash)
		if err == nil {
			t.Error("DropFaucet() should fail for amount above maximum")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("DropFaucet() error message should contain 'exceeds maximum', got: %v", err)
		}
	})

	t.Run("cooldown_period", func(t *testing.T) {
		// Test cooldown period - second request should fail
		faucetAmount := types.FloatToBigInt(100.0)
		err = d5Vault.DropFaucet(*toAddr, faucetAmount, txHash)
		if err == nil {
			t.Error("DropFaucet() should fail due to cooldown period")
		}
		if !strings.Contains(err.Error(), "cooldown") {
			t.Errorf("DropFaucet() error message should contain 'cooldown', got: %v", err)
		}
	})
}

// Test VerifyAccount method
func TestD5Vault_VerifyAccount(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account with specific password
	_, _, _, address, err := vault.Create("correct_password")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	tests := []struct {
		name    string
		addr    types.Address
		pass    string
		wantErr bool
	}{
		{"correct_password", *address, "correct_password", false},
		{"wrong_password", *address, "wrong_password", true},
		{"empty_address", types.EmptyAddress(), "correct_password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := vault.VerifyAccount(tt.addr, tt.pass)

			if tt.wantErr {
				if err == nil {
					t.Errorf("VerifyAccount() expected error, got nil")
				}
				if !addr.IsEmpty() {
					t.Errorf("VerifyAccount() expected empty address on error, got %v", addr)
				}
			} else {
				if err != nil {
					t.Errorf("VerifyAccount() error = %v", err)
				}
				if addr != tt.addr {
					t.Errorf("VerifyAccount() address = %v, want %v", addr, tt.addr)
				}
			}
		})
	}
}

// Test GetKey method
func TestD5Vault_GetKey(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	_, publicKey, _, _, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Test getting key for existing public key
	keyBytes := vault.GetKey(publicKey)
	if keyBytes == nil {
		t.Error("GetKey() should return key bytes for existing public key")
	}

	// Test getting key for non-existing public key
	nonExistentKey := "non_existent_key"
	keyBytes = vault.GetKey(nonExistentKey)
	expectedDefault := []byte{0x0, 0x0, 0xf, 0xf}
	if string(keyBytes) != string(expectedDefault) {
		t.Errorf("GetKey() for non-existing key = %v, want %v", keyBytes, expectedDefault)
	}
}

// Test GetOwner method
func TestD5Vault_GetOwner(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	owner := vault.GetOwner()
	if owner == nil {
		t.Error("GetOwner() should not return nil")
	} else {
		if owner.Status != "OP_ACC_NODE" {
			t.Errorf("GetOwner() status = %v, want OP_ACC_NODE", owner.Status)
		}
	}
}

// Test Clear method
func TestD5Vault_Clear(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create some accounts
	vault.Create("pass1")
	vault.Create("pass2")

	// Verify accounts exist
	if vault.GetCount() < 2 {
		t.Error("Expected at least 2 accounts before clear")
	}

	// Clear vault
	err = vault.Clear()
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify vault is empty
	if vault.GetCount() != 0 {
		t.Errorf("Clear() expected 0 accounts, got %d", vault.GetCount())
	}
}

// Test Status method
func TestD5Vault_Status(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	status := vault.Status()
	if status != 0xa {
		t.Errorf("Status() = 0x%x, want 0xa", status)
	}
}

// Test Sync method
func TestD5Vault_Sync(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test account
	account := createTestStateAccount("test_sync", 75.0)
	accountBytes := account.Bytes()

	// Sync the account
	vault.Sync(accountBytes)

	// Verify the account was synced
	syncedAccount := vault.Get(account.Address)
	if syncedAccount == nil {
		t.Error("Sync() account should be retrievable after sync")
	} else {
		if syncedAccount.Address != account.Address {
			t.Errorf("Sync() account name = %v, want test_sync", syncedAccount.Address)
		}
	}
}

// Test Size method
func TestD5Vault_Size(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// For in-memory vault, size should be -1 or 0
	size := vault.Size()
	if size < 0 {
		t.Logf("Size() returned %d (expected for in-memory vault)", size)
	}
}

// Test CheckRunnable method
func TestD5Vault_CheckRunnable(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Create a test transaction
	_, _, _, _, err = vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	// Test CheckRunnable (currently always returns false)
	r := big.NewInt(1)
	s := big.NewInt(2)
	tx := &types.GTransaction{} // Empty transaction for testing

	// Cast to D5Vault to access CheckRunnable method
	d5Vault := vault.(*D5Vault)
	result := d5Vault.CheckRunnable(r, s, tx)
	if result != false {
		t.Errorf("CheckRunnable() = %v, want false", result)
	}
}

// Test Prepare method
func TestD5Vault_Prepare(t *testing.T) {
	// Initialize coinbase data
	err := coinbase.InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	// Prepare should not panic
	vault.Prepare()
	// No assertions needed as Prepare() is currently empty
}
