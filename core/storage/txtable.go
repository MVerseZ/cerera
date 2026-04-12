package storage

import (
	"sync"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

var txTable TxTable

// TxTable maps transaction hash to block index (-1 while pending).
type TxTable struct {
	mu     sync.RWMutex
	byHash map[common.Hash]int
}

func (t *TxTable) Add(tx *types.GTransaction) {
	if tx == nil {
		return
	}
	h := tx.Hash()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byHash == nil {
		t.byHash = make(map[common.Hash]int)
	}
	if _, ok := t.byHash[h]; !ok {
		t.byHash[h] = -1
	}
}

func (t *TxTable) UpdateIndex(tx *types.GTransaction, bIndex int) {
	if tx == nil {
		return
	}
	h := tx.Hash()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byHash == nil {
		return
	}
	if _, ok := t.byHash[h]; ok {
		t.byHash[h] = bIndex
	}
}

func (t *TxTable) Get(hash common.Hash) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.byHash == nil {
		return -1
	}
	idx, ok := t.byHash[hash]
	if !ok {
		return -1
	}
	return idx
}

func InitTxTable() {
	txTable = TxTable{
		byHash: make(map[common.Hash]int),
	}
}

func GetTxTable() *TxTable {
	return &txTable
}
