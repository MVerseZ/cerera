package storage

import (
	"github.com/cerera/core/types"
	"github.com/cerera/internal/cerera/common"
)

var txTable TxTable

type TxRecord struct {
	Hash       common.Hash
	BlockIndex int
}
type TxTable struct {
	recs []TxRecord
}

func (t *TxTable) Add(tx *types.GTransaction) {
	txRecord := TxRecord{
		Hash:       tx.Hash(),
		BlockIndex: -1,
	}
	t.recs = append(t.recs, txRecord)
}

func (t *TxTable) UpdateIndex(tx *types.GTransaction, bIndex int) {
	for i, rec := range t.recs {
		if rec.Hash == tx.Hash() {
			t.recs[i].BlockIndex = bIndex
		}
	}
}

func (t *TxTable) Get(hash common.Hash) int {
	for _, rec := range t.recs {
		if rec.Hash == hash {
			return rec.BlockIndex
		}
	}
	return -1
}

func InitTxTable() {
	txTable = TxTable{
		recs: make([]TxRecord, 0),
	}
}

func GetTxTable() *TxTable {
	return &txTable
}
