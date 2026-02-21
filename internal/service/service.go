package service

import (
	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
)

// service provide api for stateful components (pool, chain, etc)
type ServiceProvider interface {
	GenesisHash() common.Hash
	AddBlock(b *block.Block) error
	ValidateBlock(b *block.Block) error
	ValidateBlockPoW(b *block.Block) bool
	GetCurrentHeight() int
	GetLatestHash() common.Hash
	GetChainID() int
	GetBlockByHeight(height int) *block.Block
	GetBlockByHash(hash common.Hash) *block.Block
}
