package storage

import (
	"fmt"
	"sort"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// AccountIndex holds accounts by address (O(1)) and by stable insertion index (O(1).
// Re-appending the same address updates the existing slot instead of growing index.
type AccountIndex struct {
	mu         sync.RWMutex
	index      map[int64]*account.StateAccount
	accounts   map[types.Address]*account.StateAccount
	addrIndex  map[types.Address]int64
	lastInsert int64
	sortedHex  []string
	sortDirty  bool
}

// AccountsTrie is kept as a type alias for backward compatibility in tests.
type AccountsTrie = AccountIndex

func NewAccountIndex() *AccountIndex {
	return &AccountIndex{
		index:     make(map[int64]*account.StateAccount),
		accounts:  make(map[types.Address]*account.StateAccount),
		addrIndex: make(map[types.Address]int64),
	}
}

func GetAccountsTrie() *AccountIndex {
	return NewAccountIndex()
}

func (ai *AccountIndex) Append(addr types.Address, sa *account.StateAccount) {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	if idx, ok := ai.addrIndex[addr]; ok {
		ai.accounts[addr] = sa
		ai.index[idx] = sa
		return
	}
	idx := ai.lastInsert
	ai.accounts[addr] = sa
	ai.index[idx] = sa
	ai.addrIndex[addr] = idx
	ai.lastInsert++
	ai.sortDirty = true
}

func (ai *AccountIndex) Clear() error {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	ai.accounts = make(map[types.Address]*account.StateAccount)
	ai.index = make(map[int64]*account.StateAccount)
	ai.addrIndex = make(map[types.Address]int64)
	ai.lastInsert = 0
	ai.sortedHex = nil
	ai.sortDirty = false
	return nil
}

// ReplaceAll resets the index and loads the given accounts (full restore).
func (ai *AccountIndex) ReplaceAll(accounts map[types.Address]*account.StateAccount) {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	ai.accounts = make(map[types.Address]*account.StateAccount, len(accounts))
	ai.index = make(map[int64]*account.StateAccount, len(accounts))
	ai.addrIndex = make(map[types.Address]int64, len(accounts))
	ai.lastInsert = 0
	ai.sortedHex = nil
	ai.sortDirty = true
	for addr, sa := range accounts {
		idx := ai.lastInsert
		ai.accounts[addr] = sa
		ai.index[idx] = sa
		ai.addrIndex[addr] = idx
		ai.lastInsert++
	}
	ai.rebuildSortedLocked()
}

func (ai *AccountIndex) GetAccount(addr types.Address) *account.StateAccount {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return ai.accounts[addr]
}

func (ai *AccountIndex) GetCount() int {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return len(ai.accounts)
}

func (ai *AccountIndex) Size() int {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	size := 0
	for _, acc := range ai.accounts {
		if acc != nil {
			size += len(acc.Bytes())
		}
	}
	return size
}

func (ai *AccountIndex) ReadableSize() string {
	size := ai.Size()
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%d KB", size/1024)
	}
	return fmt.Sprintf("%d MB", size/(1024*1024))
}

func (ai *AccountIndex) GetAll() map[types.Address]float64 {
	ai.mu.Lock()
	defer ai.mu.Unlock()
	res := make(map[types.Address]float64, len(ai.accounts))
	for addr, v := range ai.accounts {
		res[addr] = v.GetBalance()
	}
	return res
}

func (ai *AccountIndex) GetByIndex(idx int64) *account.StateAccount {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return ai.index[idx]
}

func (ai *AccountIndex) FindByKeyHash(keyHash common.Hash) (*account.StateAccount, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	for _, acc := range ai.accounts {
		if acc.KeyHash == keyHash {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("key hash not found")
}

func (ai *AccountIndex) sortedAddresses() []string {
	ai.mu.RLock()
	if !ai.sortDirty && len(ai.sortedHex) == len(ai.accounts) {
		out := make([]string, len(ai.sortedHex))
		copy(out, ai.sortedHex)
		ai.mu.RUnlock()
		return out
	}
	ai.mu.RUnlock()

	ai.mu.Lock()
	ai.rebuildSortedLocked()
	out := make([]string, len(ai.sortedHex))
	copy(out, ai.sortedHex)
	ai.mu.Unlock()
	return out
}

func (ai *AccountIndex) rebuildSortedLocked() {
	hexes := make([]string, 0, len(ai.accounts))
	for addr := range ai.accounts {
		hexes = append(hexes, addr.Hex())
	}
	sort.Strings(hexes)
	ai.sortedHex = hexes
	ai.sortDirty = false
}

func (ai *AccountIndex) ExportRange(offset, limit int) ([]*account.StateAccount, int) {
	hexes := ai.sortedAddresses()
	total := len(hexes)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		return nil, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	out := make([]*account.StateAccount, 0, end-offset)
	for _, h := range hexes[offset:end] {
		addr := types.HexToAddress(h)
		if sa := ai.accounts[addr]; sa != nil {
			out = append(out, sa)
		}
	}
	return out, total
}
