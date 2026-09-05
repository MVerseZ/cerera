package storage

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func createTestConfig() *config.Config {
	privateKey, _ := types.GenerateAccount()
	return &config.Config{
		Vault: config.VaultConfig{
			MEM:  true,
			PATH: "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.PubkeyToAddress(&privateKey.PublicKey),
			PRIV: types.EncodePrivateKeyToToString(privateKey),
		},
		IN_MEM: true,
	}
}

func createTestStateAccount(balance float64) *account.StateAccount {
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(&privateKey.PublicKey)
	acc := account.NewStateAccount(address, balance, common.Hash{})
	acc.Passphrase = common.BytesToHash([]byte("test_pass"))
	return acc
}

func TestNewD5Vault(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	if vault == nil {
		t.Fatal("NewD5Vault returned nil vault")
	}
	if vault.GetCount() < 1 {
		t.Errorf("Expected at least the initiator account, got %d", vault.GetCount())
	}
	if vault.Status() != 0xa {
		t.Errorf("Expected vault status 0xa, got 0x%x", vault.Status())
	}
}

func TestD5Vault_Create(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{"test_account", "password123", false},
		{"empty_pass", "", false},
		{"another_account", "secret", false},
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
			if masterKey == "" || publicKey == "" || mnemonic == "" || address == nil {
				t.Error("Create() returned empty account material")
			}
			account := vault.Get(*address)
			if account == nil {
				t.Error("Create() account should be retrievable after creation")
			} else if account.GetBalance() != 0.0 {
				t.Errorf("Create() account balance = %v, want 0.0", account.GetBalance())
			}
		})
	}
}

func TestD5Vault_Restore(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, mnemonic, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	tests := []struct {
		name     string
		mnemonic string
		pass     string
		wantErr  bool
	}{
		{"valid_restore", mnemonic, "password123", false},
		{"empty_mnemonic", "", "password123", true},
		{"invalid_mnemonic", "invalid mnemonic phrase", "password123", true},
		{"wrong_password", mnemonic, "wrong_password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, priv, err := vault.Restore(tt.mnemonic, tt.pass)
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
			if addr.IsEmpty() || priv == "" {
				t.Error("Restore() returned empty material")
			}
			if tt.name == "valid_restore" && addr != *address {
				t.Errorf("Restore() address = %v, want %v", addr, *address)
			}
		})
	}
}

func TestD5Vault_Get(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, _, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	got := vault.Get(*address)
	if got == nil {
		t.Error("Get() should return account for existing address")
	} else if got.Address != *address {
		t.Errorf("Get() account address = %v, want %v", got.Address, *address)
	}

	if vault.Get(types.EmptyAddress()) != nil {
		t.Error("Get() should return nil for empty address")
	}
}

func TestD5Vault_Put(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	acc := createTestStateAccount(50.0)
	vault.Put(acc.Address, acc)

	retrieved := vault.Get(acc.Address)
	if retrieved == nil {
		t.Error("Put() account should be retrievable after Put")
	} else if retrieved.GetBalance() != 50.0 {
		t.Errorf("Put() account balance = %v, want 50.0", retrieved.GetBalance())
	}
}

func TestD5Vault_GetAll(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	vault.Create("pass1")
	vault.Create("pass2")

	if vault.GetAll() == nil {
		t.Error("GetAll() should not return nil")
	}
	if vault.GetCount() < 3 {
		t.Errorf("GetAll() expected at least initiator + 2 created, got %d", vault.GetCount())
	}
}

func TestD5Vault_GetCount(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	initialCount := vault.GetCount()
	vault.Create("password123")
	if vault.GetCount() != initialCount+1 {
		t.Errorf("GetCount() after create = %d, want %d", vault.GetCount(), initialCount+1)
	}
}

func TestD5Vault_UpdateBalance(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, _, fromAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create from account: %v", err)
	}
	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create to account: %v", err)
	}

	d5Vault := vault.(*D5Vault)
	if err := d5Vault.DropFaucet(*fromAddr, types.FloatToBigInt(100.0), common.Hash{0x9}); err != nil {
		t.Fatalf("DropFaucet: %v", err)
	}

	fromAccount := vault.Get(*fromAddr)
	toAccount := vault.Get(*toAddr)
	initialFromBalance := types.FloatToBigInt(fromAccount.GetBalance())
	initialToBalance := types.FloatToBigInt(toAccount.GetBalance())

	transferAmount := types.FloatToBigInt(25.0)
	txHash := common.Hash{0x1, 0x2, 0x3}
	if err := d5Vault.UpdateBalance(*fromAddr, *toAddr, transferAmount, txHash); err != nil {
		t.Fatalf("UpdateBalance: %v", err)
	}

	fromAccount = vault.Get(*fromAddr)
	toAccount = vault.Get(*toAddr)
	expectedFrom := new(big.Int).Sub(initialFromBalance, transferAmount)
	expectedTo := new(big.Int).Add(initialToBalance, transferAmount)
	if fromAccount.GetBalance() != types.BigIntToFloat(expectedFrom) {
		t.Errorf("UpdateBalance() from balance = %v, want %v", fromAccount.GetBalance(), types.BigIntToFloat(expectedFrom))
	}
	if toAccount.GetBalance() != types.BigIntToFloat(expectedTo) {
		t.Errorf("UpdateBalance() to balance = %v, want %v", toAccount.GetBalance(), types.BigIntToFloat(expectedTo))
	}

	toAccount.Inputs.RLock()
	defer toAccount.Inputs.RUnlock()
	if val, exists := toAccount.Inputs.M[txHash]; !exists || val.Cmp(transferAmount) != 0 {
		t.Errorf("UpdateBalance() transaction not properly recorded in inputs")
	}
}

func TestD5Vault_DropFaucet(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	toAccount := vault.Get(*toAddr)
	initialToBalance := types.FloatToBigInt(toAccount.GetBalance())
	faucetAmount := types.FloatToBigInt(100.0)
	txHash := common.Hash{0x4, 0x5, 0x6}

	d5Vault := vault.(*D5Vault)
	if err := d5Vault.DropFaucet(*toAddr, faucetAmount, txHash); err != nil {
		t.Errorf("DropFaucet() error = %v", err)
	}

	toAccount = vault.Get(*toAddr)
	expectedTo := new(big.Int).Add(initialToBalance, faucetAmount)
	if toAccount.GetBalance() != types.BigIntToFloat(expectedTo) {
		t.Errorf("DropFaucet() to balance = %v, want %v", toAccount.GetBalance(), types.BigIntToFloat(expectedTo))
	}

	toAccount.Inputs.RLock()
	defer toAccount.Inputs.RUnlock()
	if val, exists := toAccount.Inputs.M[txHash]; !exists || val.Cmp(faucetAmount) != 0 {
		t.Errorf("DropFaucet() transaction not properly recorded in inputs")
	}
}

func TestD5Vault_DropFaucetWithLimits(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, _, toAddr, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	d5Vault := vault.(*D5Vault)
	txHash := common.Hash{0x4, 0x5, 0x6}

	t.Run("valid_faucet_request", func(t *testing.T) {
		if err := d5Vault.DropFaucet(*toAddr, types.FloatToBigInt(100.0), txHash); err != nil {
			t.Errorf("DropFaucet() error = %v", err)
		}
	})

	t.Run("amount_below_minimum", func(t *testing.T) {
		err := d5Vault.DropFaucet(*toAddr, types.FloatToBigInt(0.5), txHash)
		if err == nil {
			t.Error("DropFaucet() should fail for amount below minimum")
		} else if !strings.Contains(err.Error(), "below minimum") {
			t.Errorf("DropFaucet() error message should contain 'below minimum', got: %v", err)
		}
	})

	t.Run("amount_above_maximum", func(t *testing.T) {
		err := d5Vault.DropFaucet(*toAddr, types.FloatToBigInt(2000.0), txHash)
		if err == nil {
			t.Error("DropFaucet() should fail for amount above maximum")
		} else if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("DropFaucet() error message should contain 'exceeds maximum', got: %v", err)
		}
	})

	t.Run("cooldown_period", func(t *testing.T) {
		err := d5Vault.DropFaucet(*toAddr, types.FloatToBigInt(100.0), txHash)
		if err == nil {
			t.Error("DropFaucet() should fail due to cooldown period")
		} else if !strings.Contains(err.Error(), "cooldown") {
			t.Errorf("DropFaucet() error message should contain 'cooldown', got: %v", err)
		}
	})
}

func TestD5Vault_VerifyAccount(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

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
			} else if err != nil {
				t.Errorf("VerifyAccount() error = %v", err)
			} else if addr != tt.addr {
				t.Errorf("VerifyAccount() address = %v, want %v", addr, tt.addr)
			}
		})
	}
}

func TestD5Vault_GetOwner(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	owner := vault.GetOwner()
	if owner == nil {
		t.Error("GetOwner() should not return nil")
	} else if owner.Status != 3 {
		t.Errorf("GetOwner() status = %d, want OP_ACC_NODE (3)", owner.Status)
	}
}

func TestD5Vault_Clear(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	vault.Create("pass1")
	vault.Create("pass2")
	if vault.GetCount() < 2 {
		t.Error("Expected at least 2 accounts before clear")
	}

	if err := vault.Clear(); err != nil {
		t.Errorf("Clear() error = %v", err)
	}
	if vault.GetCount() != 0 {
		t.Errorf("Clear() expected 0 accounts, got %d", vault.GetCount())
	}
}

func TestD5Vault_Status(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	if status := vault.Status(); status != 0xa {
		t.Errorf("Status() = 0x%x, want 0xa", status)
	}
}

func TestD5Vault_Sync(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	acc := createTestStateAccount(75.0)
	vault.Sync(acc.Bytes())

	synced := vault.Get(acc.Address)
	if synced == nil {
		t.Error("Sync() account should be retrievable after sync")
	} else if synced.Address != acc.Address {
		t.Errorf("Sync() address = %v, want %v", synced.Address, acc.Address)
	}
}

func TestD5Vault_Size(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	_ = vault.Size()
}

func TestD5Vault_CheckRunnable(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	d5Vault := vault.(*D5Vault)
	if d5Vault.CheckRunnable(big.NewInt(1), big.NewInt(2), &types.GTransaction{}) {
		t.Errorf("CheckRunnable() = true, want false")
	}
}

func TestD5Vault_Prepare(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	vault.Prepare()
}

func TestD5Vault_GetKeyRemoved(t *testing.T) {
	t.Skip("GetKey is no longer part of the vault API")
}
