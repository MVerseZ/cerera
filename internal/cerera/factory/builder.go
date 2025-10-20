package factory

import (
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/observer"
	"github.com/cerera/internal/cerera/types"
)

func GetBuilder() (Builder, error) {
	return &BlockBuilder{}, nil
}

type Builder interface {
	Build(tx *types.GTransaction) (common.Hash, error)
}

// GetID implements pool.Observer.
func (b *BlockBuilder) GetID() string {
	panic("unimplemented")
}

// Update implements pool.Observer.
func (b *BlockBuilder) Update(tx *types.GTransaction) {
	panic("unimplemented")
}

type BlockBuilder struct {
	txObserver observer.Observer
	txs        []*types.GTransaction
}

func (b *BlockBuilder) Build(tx *types.GTransaction) (common.Hash, error) {
	b.txs = append(b.txs, tx)
	return tx.Hash(), nil
}
