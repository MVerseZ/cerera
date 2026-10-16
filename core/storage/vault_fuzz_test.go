package storage

import (
	"strings"
	"testing"

	"github.com/cerera/core/account"
	"github.com/cerera/core/types"
)

// FuzzStateAccountBytes tests the serialization/deserialization of StateAccount
func FuzzStateAccountBytes(f *testing.F) {
	testAccount := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: types.BytesToAddress([]byte("test_address_123456789012345678901234567890")),
			Nonce:   1,
		},
		Status: 0,
	}
	testAccount.SetBalance(100.0)
	f.Add(testAccount.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 100 {
			return
		}

		acc := types.BytesToStateAccount(data)

		if acc != nil {
			serialized := acc.Bytes()

			if len(serialized) == 0 {
				t.Errorf("Serialized account should not be empty")
			}

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
	f.Add([]byte("test_addr"), []byte("test_data"))
	f.Add([]byte(""), []byte("data"))
	f.Add([]byte("very_long_address_string_that_might_cause_issues_123456789012345678901234567890"), []byte("test"))

	f.Fuzz(func(t *testing.T, addrBytes, data []byte) {
		at := GetAccountsTrie()

		var addr types.Address
		if len(addrBytes) >= 48 {
			copy(addr[:], addrBytes[:48])
		} else {
			copy(addr[:], addrBytes)
		}

		account := &account.StateAccount{
			StateAccountData: account.StateAccountData{
				Address: addr,
				Nonce:   1,
			},
			Status: 0,
		}
		account.SetBalance(0)

		at.Append(addr, account)

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

		var addr types.Address
		copy(addr[:], addrBytes)

		account := &account.StateAccount{
			StateAccountData: account.StateAccountData{
				Address: addr,
				Nonce:   1,
			},
			Status: 0,
		}
		account.SetBalance(0)

		at.Append(addr, account)

		retrieved := at.GetAccount(addr)
		if retrieved == nil {
			t.Errorf("GetAccount should return the account we just added")
		}
	})
}

// FuzzAccountsTrieSize tests the Size function
func FuzzAccountsTrieSize(f *testing.F) {
	f.Fuzz(func(t *testing.T, numAccounts int) {
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat("a", 40)))
			account := &account.StateAccount{
				StateAccountData: account.StateAccountData{
					Address: addr,
					Nonce:   1,
				},
				Status: 0,
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
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat(string(rune(i%26+97)), 40)))
			account := &account.StateAccount{
				StateAccountData: account.StateAccountData{
					Address: addr,
					Nonce:   1,
				},
				Status: 0,
			}
			account.SetBalance(float64(i * 10))
			at.Append(addr, account)
		}

		all := at.GetAll()
		if len(all) != numAccounts {
			t.Errorf("GetAll() returned %d accounts, want %d", len(all), numAccounts)
		}

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
		if numAccounts < 0 || numAccounts > 1000 {
			return
		}

		at := GetAccountsTrie()

		for i := 0; i < numAccounts; i++ {
			addr := types.BytesToAddress([]byte(strings.Repeat("b", 40)))
			account := &account.StateAccount{
				StateAccountData: account.StateAccountData{
					Address: addr,
					Nonce:   1,
				},
				Status: 0,
			}
			account.SetBalance(float64(i))
			at.Append(addr, account)
		}

		if at.Size() != numAccounts {
			t.Errorf("Expected %d accounts before clear, got %d", numAccounts, at.Size())
		}

		err := at.Clear()
		if err != nil {
			t.Errorf("Clear() error = %v", err)
		}

		if at.Size() != 0 {
			t.Errorf("Size() after clear = %d, want 0", at.Size())
		}
	})
}
