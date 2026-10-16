package storage

import (
	"context"
	"testing"

	"github.com/cerera/core/types"
)

func TestWallet_Create(t *testing.T) {
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
		{"valid_pass", "password123", false},
		{"empty_pass", "", true},
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
			_ = masterKey
			_ = mnemonic
		})
	}
}

func TestWallet_Restore(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	createdPriv, _, mnemonic, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	tests := []struct {
		name     string
		mnemonic string
		pass     string
		wantErr  bool
	}{
		{"valid_roundtrip", mnemonic, "password123", false},
		{"empty_mnemonic", "", "password123", true},
		{"invalid_mnemonic", "invalid mnemonic phrase", "password123", true},
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
			if tt.name == "valid_roundtrip" {
				if addr != *address {
					t.Errorf("Restore() address = %v, want %v", addr, *address)
				}
				if priv != createdPriv {
					t.Error("Restore() private key should match Create()")
				}
			}
		})
	}
}

func TestWallet_VerifyAccount(t *testing.T) {
	cfg := createTestConfig()
	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}

	_, _, _, address, err := vault.Create("password123")
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	tests := []struct {
		name    string
		addr    types.Address
		pass    string
		wantErr bool
	}{
		{"existing_account", *address, "password123", false},
		{"empty_address", types.EmptyAddress(), "password123", true},
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
