package storage

import (
	"fmt"
	"sync"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// structure stores account and other accounting stuff
// in smth like merkle-b-tree (cool data structure)
type AccountsTrie struct {
	mu         sync.RWMutex
	index      map[int64]*types.StateAccount
	accounts   map[types.Address]*types.StateAccount
	lastInsert int64
}

func GetAccountsTrie() *AccountsTrie {
	// this smth like init function
	return &AccountsTrie{
		index:      make(map[int64]*types.StateAccount),
		accounts:   make(map[types.Address]*types.StateAccount),
		lastInsert: 0,
	}
}

// add account with address to Account Tree
func (at *AccountsTrie) Append(addr types.Address, sa *types.StateAccount) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts[addr] = sa
	at.index[at.lastInsert] = sa
	at.lastInsert++
}

func (at *AccountsTrie) Clear() error {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.accounts = make(map[types.Address]*types.StateAccount)
	at.index = make(map[int64]*types.StateAccount)
	at.lastInsert = 0
	return nil
}

func (at *AccountsTrie) GetAccount(addr types.Address) *types.StateAccount {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.accounts[addr]
}

// func (at *AccountsTrie) GetKBytes(pubKey *bip32.Key) []byte {
// 	at.mu.RLock()
// 	defer at.mu.RUnlock()
// 	pubKeyBytes := []byte(pubKey.B58Serialize())

// 	// MPub is [78]byte, so we can only store up to 78 bytes
// 	// Compare only the first 78 bytes (or less) of both keys
// 	maxCompareLen := 78 // MPub array size

// 	for _, account := range at.accounts {
// 		// Determine how many bytes to compare
// 		compareLen := maxCompareLen
// 		if len(pubKeyBytes) < compareLen {
// 			compareLen = len(pubKeyBytes)
// 		}
// 		// if len(account.MPub) < compareLen {
// 		// 	compareLen = len(account.MPub)
// 		// }

// 		// Compare the first compareLen bytes
// 		if compareLen > 0 && bytes.Equal(pubKeyBytes[:compareLen], account.MPub[:compareLen]) {
// 			return account.CodeHash
// 		}
// 	}
// 	return nil
// }

func (at *AccountsTrie) Size() int {
	// at.mu.Lock()
	// defer at.mu.Unlock()
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
	// at.mu.Lock()
	res := make(map[types.Address]float64)
	for addr, v := range at.accounts {
		res[addr] = v.GetBalance()
	}
	// at.mu.Unlock()

	return res
}

func (at *AccountsTrie) GetByIndex(idx int64) *types.StateAccount {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.index[idx]
}

// FindAddrByPub searches for an address by its public key serialization
func (at *AccountsTrie) FindAddrByPub(pubKey []byte) (types.Address, error) {
	at.mu.RLock()
	defer at.mu.RUnlock()
	totalAccounts := len(at.accounts)
	inputPubKeyHash := common.BytesToHash(pubKey)
	for _, account := range at.accounts {

		if account.PublicKeyHash == inputPubKeyHash {
			return account.Address, nil
		}
	}
	// Return more informative error
	return types.EmptyAddress(), fmt.Errorf("public key not found (searched %d accounts, pubKey length: %d)", totalAccounts, len(pubKey))
}
