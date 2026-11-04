package service

import (
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/ports"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/validation"
)

var splogger = logger.Named("chain")

// ServiceProvider is a facade from ICENet sync/consensus to chain and vault ports.
type ServiceProvider struct {
	chain    ports.ChainLedger
	vault    ports.VaultStore
	blockVal *validation.BlockValidator
}

// NewServiceProvider wires chain and vault ports for in-process use (no RPC registry).
func NewServiceProvider(chain ports.ChainLedger, vault ports.VaultStore, bv *validation.BlockValidator) (*ServiceProvider, error) {
	return &ServiceProvider{
		chain:    chain,
		vault:    vault,
		blockVal: bv,
	}, nil
}

func (sp *ServiceProvider) SetBlockValidator(bv *validation.BlockValidator) {
	sp.blockVal = bv
}

func (sp *ServiceProvider) EnsureAccount(addr types.Address) {
	if sp.vault != nil {
		sp.vault.EnsureAccount(addr)
	}
}

func (sp *ServiceProvider) AddBlock(b *block.Block) error {
	if b == nil {
		return fmt.Errorf("block is nil")
	}
	if sp.chain == nil {
		return fmt.Errorf("chain not configured")
	}
	return sp.chain.UpdateChain(b)
}

func (sp *ServiceProvider) Reorg(blocks []*block.Block) error {
	if sp.chain == nil {
		return fmt.Errorf("chain reference not configured")
	}
	return sp.chain.Reorg(blocks)
}

func (sp *ServiceProvider) ValidateOrphanBlock(b *block.Block, parent *block.Block) error {
	if sp.blockVal == nil {
		return fmt.Errorf("block validator not configured")
	}
	return sp.blockVal.ValidateOrphan(b, parent)
}

func (sp *ServiceProvider) GetBlockByHash(h common.Hash) *block.Block {
	if sp.chain == nil {
		return nil
	}
	return sp.chain.GetBlock(h)
}

func (sp *ServiceProvider) GetBlockByHeight(height int) *block.Block {
	if sp.chain == nil {
		return nil
	}
	return sp.chain.GetBlockByHeight(height)
}

func (sp *ServiceProvider) GetChainID() int {
	if sp.chain == nil {
		return 0
	}
	return sp.chain.GetChainId()
}

func (sp *ServiceProvider) GetCurrentHeight() int {
	if sp.chain == nil {
		return 0
	}
	if head := sp.chain.GetLatestBlock(); head != nil && head.Head != nil {
		return head.Head.Height
	}
	return 0
}

func (sp *ServiceProvider) GetLatestHash() common.Hash {
	if sp.chain == nil {
		return common.Hash{}
	}
	if head := sp.chain.GetLatestBlock(); head != nil {
		return head.Hash
	}
	return common.Hash{}
}

func (sp *ServiceProvider) GetStorageSize() int {
	if sp.vault == nil {
		return 0
	}
	return sp.vault.GetCount()
}

func (sp *ServiceProvider) GetStorageServiceName() string {
	return storage.VAULT_SERVICE_NAME
}

func (sp *ServiceProvider) ValidateBlock(b *block.Block) error {
	if b == nil || b.Head == nil {
		return fmt.Errorf("block is nil")
	}

	currentHeight := sp.GetCurrentHeight()
	if b.Head.Height == 0 {
		if currentHeight > 0 {
			return fmt.Errorf("unexpected genesis at height 0 with local height %d", currentHeight)
		}
	} else {
		expectedHeight := currentHeight + 1
		if b.Head.Height != expectedHeight {
			return fmt.Errorf("unexpected block height %d, expected %d", b.Head.Height, expectedHeight)
		}
		latestHash := sp.GetLatestHash()
		if latestHash != (common.Hash{}) && b.Head.PrevHash != latestHash {
			return fmt.Errorf("prev hash mismatch: got %s want %s", b.Head.PrevHash, latestHash)
		}
	}

	if sp.blockVal != nil {
		return sp.blockVal.ValidateContent(b)
	}
	_, err := chain.ValidateBlocks([]*block.Block{b})
	return err
}

func (sp *ServiceProvider) ValidateBlockPoW(b *block.Block) bool {
	if sp.blockVal != nil {
		return sp.blockVal.ValidatePoW(b)
	}
	return b != nil && b.Head != nil
}

func (sp *ServiceProvider) GetVaultSyncStats() storage.VaultSyncStats {
	if sp.vault != nil {
		return sp.vault.VaultSyncStats()
	}
	return storage.VaultSyncStats{
		Accounts: sp.GetStorageSize(),
	}
}

func (sp *ServiceProvider) ExportStorageAccountRange(offset, limit int) ([][]byte, int) {
	if sp.vault != nil {
		return sp.vault.ExportAccountRangeSortedByAddress(offset, limit)
	}
	return nil, 0
}

func (sp *ServiceProvider) ExportStorageContractCodeRange(offset, limit int) ([][]byte, int) {
	if sp.vault != nil {
		return sp.vault.ExportContractCodeRange(offset, limit)
	}
	return nil, 0
}

func (sp *ServiceProvider) ExportStorageContractStorageRange(offset, limit int) ([][]byte, int) {
	if sp.vault != nil {
		return sp.vault.ExportContractStorageRange(offset, limit)
	}
	return nil, 0
}

func (sp *ServiceProvider) ImportStorageAccounts(accounts [][]byte) error {
	return sp.ApplyStorageAccounts(accounts)
}

func (sp *ServiceProvider) GetGenesisBlock() *block.Block {
	return sp.GetBlockByHeight(0)
}

func (sp *ServiceProvider) GetGenesisHash() common.Hash {
	if genesis := sp.GetGenesisBlock(); genesis != nil {
		return genesis.Hash
	}
	return common.Hash{}
}

func (sp *ServiceProvider) ApplyStorageAccounts(accounts [][]byte) error {
	if sp.vault == nil {
		return fmt.Errorf("vault not initialized")
	}
	for _, blob := range accounts {
		if len(blob) == 0 {
			continue
		}
		sp.vault.Sync(blob)
	}
	return nil
}

func (sp *ServiceProvider) ApplyStorageContractCodes(blobs [][]byte) error {
	if sp.vault == nil {
		return fmt.Errorf("vault not initialized")
	}
	for _, blob := range blobs {
		if len(blob) == 0 {
			continue
		}
		if err := sp.vault.SyncContractCodeBlob(blob); err != nil {
			return err
		}
	}
	return nil
}

func (sp *ServiceProvider) ApplyStorageContractStorage(blobs [][]byte) error {
	if sp.vault == nil {
		return fmt.Errorf("vault not initialized")
	}
	for _, blob := range blobs {
		if len(blob) == 0 {
			continue
		}
		if err := sp.vault.SyncContractStorageBlob(blob); err != nil {
			return err
		}
	}
	return nil
}
