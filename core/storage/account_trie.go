package storage

import (
	"fmt"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// AccountsTrie holds accounts by address (O(1)) and by stable insertion index (O(1)).
// Re-appending the same address updates the existing slot instead of growing index.
type AccountsTrie struct {
	mu         sync.RWMutex
	index      map[int64]*account.StateAccount
	accounts   map[types.Address]*account.StateAccount
	addrIndex  map[types.Address]int64 // address -> ordinal in index
	lastInsert int64                   // next free ordinal == distinct address count
}

func GetAccountsTrie() *AccountsTrie {
	// this smth like init function
	return &AccountsTrie{
		index:      make(map[int64]*account.StateAccount),
		accounts:   make(map[types.Address]*account.StateAccount),
		addrIndex:  make(map[types.Address]int64),
		lastInsert: 0,
	}
}

// Append inserts a new address or replaces the account at an existing address in O(1).
func (at *AccountsTrie) Append(addr types.Address, sa *account.StateAccount) {
	at.mu.Lock()
	defer at.mu.Unlock()
	if idx, ok := at.addrIndex[addr]; ok {
		at.accounts[addr] = sa
		at.index[idx] = sa
		return
	}
	idx := at.lastInsert
	at.accounts[addr] = sa
	at.index[idx] = sa
	at.addrIndex[addr] = idx
	at.lastInsert++
}

func (at *AccountsTrie) Clear() error {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts = make(map[types.Address]*account.StateAccount)
	at.index = make(map[int64]*account.StateAccount)
	at.addrIndex = make(map[types.Address]int64)
	at.lastInsert = 0
	return nil
}

func (at *AccountsTrie) GetAccount(addr types.Address) *account.StateAccount {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.accounts[addr]
}

// GetCount returns the total number of accounts in the trie
func (at *AccountsTrie) GetCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return len(at.accounts)
}

// Size returns the total size of all accounts in the trie
func (at *AccountsTrie) Size() int {
	at.mu.Lock()
	defer at.mu.Unlock()
	size := 0
	for _, account := range at.accounts {
		if account != nil {
			data := account.Bytes()
			size += len(data)
		}
	}
	return size
	// return len(at.accounts)
}

func (at *AccountsTrie) ReadableSize() string {
	size := at.Size()
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%d KB", size/1024)
	}
	return fmt.Sprintf("%d MB", size/(1024*1024))
}

func (at *AccountsTrie) GetAll() map[types.Address]float64 {
	at.mu.Lock()
	defer at.mu.Unlock()
	res := make(map[types.Address]float64)
	for addr, v := range at.accounts {
		res[addr] = v.GetBalance()
	}
	return res
}

func (at *AccountsTrie) GetByIndex(idx int64) *account.StateAccount {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.index[idx]
}

func (at *AccountsTrie) FindByKeyHash(keyHash common.Hash) (*account.StateAccount, error) {
	at.mu.RLock()
	defer at.mu.RUnlock()
	for _, account := range at.accounts {
		if account.KeyHash == keyHash {
			return account, nil
		}
	}
	return nil, fmt.Errorf("key hash not found")
}
