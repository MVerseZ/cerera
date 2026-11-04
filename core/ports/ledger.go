package ports

import (
	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
)

// ChainReader exposes read-only chain head and block lookup.
type ChainReader interface {
	GetLatestBlock() *block.Block
	GetBlockByHeight(height int) *block.Block
	GetBlock(blockHash common.Hash) *block.Block
	GetChainId() int
}

// ChainWriter applies canonical chain updates.
type ChainWriter interface {
	UpdateChain(newBlock *block.Block) error
	Reorg(blocks []*block.Block) error
}

// ChainLedger combines chain read and write ports used by ICENet.
type ChainLedger interface {
	ChainReader
	ChainWriter
}

// VaultReader exposes vault statistics and export for P2P sync.
type VaultReader interface {
	GetCount() int
	EnsureAccount(addr types.Address)
	VaultSyncStats() storage.VaultSyncStats
	ExportAccountRangeSortedByAddress(offset, limit int) ([][]byte, int)
	ExportContractCodeRange(offset, limit int) ([][]byte, int)
	ExportContractStorageRange(offset, limit int) ([][]byte, int)
}

// VaultWriter applies replicated vault state from peers.
type VaultWriter interface {
	Sync(saBytes []byte)
	SyncContractCodeBlob(blob []byte) error
	SyncContractStorageBlob(blob []byte) error
}

// VaultStore combines vault read and write ports used by ICENet.
type VaultStore interface {
	VaultReader
	VaultWriter
}
