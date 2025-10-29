package storage

import (
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

// FuzzStateAccountBytes tests the serialization/deserialization of StateAccount
func FuzzStateAccountBytes(f *testing.F) {
	// Add seed values
	testAccount := &types.StateAccount{
		Address:  types.BytesToAddress([]byte("test_address_123456789012345678901234567890")),
		Nonce:    1,
		Root:     common.Hash{},
		CodeHash: []byte("test_code_hash"),
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte("test_pass")),
		MPub:       "test_mpub",
	}
	testAccount.SetBalance(100.0)
	f.Add(testAccount.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip if data is too small
		if len(data) < 100 {
			return
		}

		// Try to deserialize
		account := types.BytesToStateAccount(data)

		// If we got a valid account, try to serialize it back
		if account != nil {
			serialized := account.Bytes()

			// Basic sanity check: serialized data should not be empty
			if len(serialized) == 0 {
				t.Errorf("Serialized account should not be empty")
			}

			// If original data was valid, deserialized should match some properties
			if len(data) > 0 {
				deserialized := types.BytesToStateAccount(serialized)
				if deserialized == nil {
					t.Errorf("Should be able to deserialize what we just serialized")
				}
			}
		}
	})
}

// FuzzAccountsTrieAppend tests the Append function with various inputs
func FuzzAccountsTrieAppend(f *testing.F) {
	// Add seed values
	f.Add([]byte("test_addr"), []byte("test_data"))
	f.Add([]byte(""), []byte("data"))
	f.Add([]byte("very_long_address_string_that_might_cause_issues_123456789012345678901234567890"), []byte("test"))

	f.Fuzz(func(t *testing.T, addrBytes, data []byte) {
		at := GetAccountsTrie()

		// Convert bytes to Address type (limit to 48 bytes as per Address length)
		var addr types.Address
		if len(addrBytes) >= 48 {
			copy(addr[:], addrBytes[:48])
		} else {
			copy(addr[:], addrBytes)
		}

		// Create a minimal StateAccount
		account := &types.StateAccount{
			Address:  addr,
			Nonce:    1,
			Root:     common.Hash{},
			CodeHash: data,
			Status:   "OP_ACC_NEW",
			Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			Inputs: &types.Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
			Passphrase: common.BytesToHash(data),
			MPub:       "",
		}
		account.SetBalance(0)

		at.Append(addr, account)

		// Verify the account was added
		retrieved := at.GetAccount(addr)
		if retrieved == nil {
			t.Errorf("Account should be retrievable after Append")
		}
	})
}

// FuzzAccountsTrieGet tests the GetAccount function
func FuzzAccountsTrieGet(f *testing.F) {
	f.Fuzz(func(t *testing.T, addrBytes []byte) {
		at := GetAccountsTrie()

		// Create and add an account first
		var addr types.Address
		copy(addr[:], addrBytes)

		account := &types.StateAccount{
			Address: addr,
			Nonce:   1,
			Status:  "OP_ACC_NEW",
			Bloom:   []byte{0xf, 0xf, 0xf, 0x1},
			Inputs: &types.Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		account.SetBalance(0)

		at.Append(addr, account)

		// Try to get it
		retrieved := at.GetAccount(addr)
		if retrieved == nil {
			t.Errorf("GetAccount should return the account we just added")
		}
	})
}

// FuzzAccountsTrieSize tests the Size function
func FuzzAccountsTrieSize(f *testing.F) {
	f.Fuzz(func(t *testing.T, numAccounts int) {
		// Limit the number to avoid excessive memory usage
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		// Add some accounts
		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat("a", 40)))
			account := &types.StateAccount{
				Address: addr,
				Nonce:   1,
				Status:  "OP_ACC_NEW",
				Bloom:   []byte{0xf, 0xf, 0xf, 0x1},
				Inputs: &types.Input{
					RWMutex: &sync.RWMutex{},
					M:       make(map[common.Hash]*big.Int),
				},
			}
			account.SetBalance(float64(i))
			at.Append(addr, account)
		}

		size := at.Size()
		if size != numAccounts {
			t.Errorf("Size() = %d, want %d", size, numAccounts)
		}
	})
}

// FuzzAccountsTrieGetAll tests the GetAll function
func FuzzAccountsTrieGetAll(f *testing.F) {
	f.Fuzz(func(t *testing.T, numAccounts int) {
		// Limit the number to avoid excessive memory usage
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		// Add some accounts with balances
		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat(string(rune(i%26+97)), 40)))
			account := &types.StateAccount{
				Address: addr,
				Nonce:   1,
				Status:  "OP_ACC_NEW",
				Bloom:   []byte{0xf, 0xf, 0xf, 0x1},
				Inputs: &types.Input{
					RWMutex: &sync.RWMutex{},
					M:       make(map[common.Hash]*big.Int),
				},
			}
			account.SetBalance(float64(i * 10))
			at.Append(addr, account)
		}

		all := at.GetAll()
		if len(all) != numAccounts {
			t.Errorf("GetAll() returned %d accounts, want %d", len(all), numAccounts)
		}

		// Verify balances
		for addr, balance := range all {
			if addr == (types.Address{}) {
				t.Errorf("GetAll() should not return empty addresses")
			}
			if balance < 0 {
				t.Errorf("GetAll() balance should not be negative: %f", balance)
			}
		}
	})
}

// FuzzAccountsTrieClear tests the Clear function
func FuzzAccountsTrieClear(f *testing.F) {
	f.Fuzz(func(t *testing.T, numAccounts int) {
		// Limit the number to avoid excessive memory usage
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		// Add some accounts
		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat("b", 40)))
			account := &types.StateAccount{
				Address: addr,
				Nonce:   1,
				Status:  "OP_ACC_NEW",
				Bloom:   []byte{0xf, 0xf, 0xf, 0x1},
				Inputs: &types.Input{
					RWMutex: &sync.RWMutex{},
					M:       make(map[common.Hash]*big.Int),
				},
			}
			account.SetBalance(float64(i))
			at.Append(addr, account)
		}

		// Verify accounts were added
		if at.Size() != numAccounts {
			t.Errorf("Expected %d accounts before clear, got %d", numAccounts, at.Size())
		}

		// Clear
		err := at.Clear()
		if err != nil {
			t.Errorf("Clear() error = %v", err)
		}

		// Verify cleared
		if at.Size() != 0 {
			t.Errorf("Size() after clear = %d, want 0", at.Size())
		}
	})
}
