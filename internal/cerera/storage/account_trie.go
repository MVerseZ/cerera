package storage

import (
	"sync"

	"github.com/cerera/internal/cerera/types"
	"github.com/tyler-smith/go-bip32"
)

// structure stores account and other accounting stuff
// in smth like merkle-b-tree (cool data structure)
type AccountsTrie struct {
	mu         sync.RWMutex
	index      map[int64]types.StateAccount
	accounts   map[types.Address]types.StateAccount
	lastInsert int64
}

func GetAccountsTrie() *AccountsTrie {
	// this smth like init function
	return &AccountsTrie{
		index:      make(map[int64]types.StateAccount),
		accounts:   make(map[types.Address]types.StateAccount),
		lastInsert: 0,
	}
}

// add account with address to Account Tree
func (at *AccountsTrie) Append(addr types.Address, sa types.StateAccount) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts[addr] = sa
	at.index[at.lastInsert] = sa
	at.lastInsert++
}

func (at *AccountsTrie) Clear() error {
	// at.mu.Lock()
	// defer at.mu.Unlock()
	at.accounts = make(map[types.Address]types.StateAccount)
	at.lastInsert = 0
	return nil
}

func (at *AccountsTrie) GetAccount(addr types.Address) types.StateAccount {
	// at.mu.Lock()
	// defer at.mu.Unlock()
	return at.accounts[addr]
}

func (at *AccountsTrie) GetKBytes(pubKey *bip32.Key) []byte {
	// at.mu.Lock()
	// defer at.mu.Unlock()
	for _, account := range at.accounts {
		if pubKey.B58Serialize() == account.MPub {
			return account.CodeHash
		}
	}
	return nil
}

func (at *AccountsTrie) Size() int {
	// at.mu.Lock()
	// defer at.mu.Unlock()
	return len(at.accounts)
}

func (at *AccountsTrie) GetAll() map[types.Address]float64 {
	// at.mu.Lock()
	res := make(map[types.Address]float64)
	for addr, v := range at.accounts {
		res[addr] = types.BigIntToFloat(v.Balance)
	}
	// at.mu.Unlock()

	return res
}

func (at *AccountsTrie) GetByIndex(idx int64) types.StateAccount {
	return at.index[idx]
}
