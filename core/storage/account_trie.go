package storage

import (
	"fmt"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// structure stores account and other accounting stuff
// in smth like merkle-b-tree (cool data structure)
type AccountsTrie struct {
	mu         sync.RWMutex
	index      map[int64]*account.StateAccount
	accounts   map[types.Address]*account.StateAccount
	lastInsert int64
}

func GetAccountsTrie() *AccountsTrie {
	// this smth like init function
	return &AccountsTrie{
		index:      make(map[int64]*account.StateAccount),
		accounts:   make(map[types.Address]*account.StateAccount),
		lastInsert: 0,
	}
}

// add account with address to Account Tree
func (at *AccountsTrie) Append(addr types.Address, sa *account.StateAccount) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts[addr] = sa
	at.index[at.lastInsert] = sa
	at.lastInsert++
}

func (at *AccountsTrie) Clear() error {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts = make(map[types.Address]*account.StateAccount)
	at.index = make(map[int64]*account.StateAccount)
	at.lastInsert = 0
	return nil
}

func (at *AccountsTrie) GetAccount(addr types.Address) *account.StateAccount {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.accounts[addr]
}

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
