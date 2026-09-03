package service

import (
	"context"
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validation"
)

var splogger = logger.Named("chain")

type ServiceProvider struct {
	ctx context.Context

	blockVal *validation.BlockValidator
	chainRef *chain.Chain
}

func (sp *ServiceProvider) SetChainRef(bc *chain.Chain) {
	sp.chainRef = bc
}

func NewServiceProvider(ctx context.Context) (*ServiceProvider, error) {
	return &ServiceProvider{ctx: ctx}, nil
}

func (sp *ServiceProvider) rpcCtx() context.Context {
	if sp.ctx != nil {
		return sp.ctx
	}
	return context.Background()
}

func (sp *ServiceProvider) callChain(method string, params ...any) (any, error) {
	reg, ok := service.GetRegistryFromContext(sp.ctx)
	if !ok {
		return nil, fmt.Errorf("service registry not available")
	}
	svc, ok := reg.GetService(service.CHAIN_SERVICE_NAME)
	if !ok || svc == nil {
		return nil, fmt.Errorf("chain service %q not registered", service.CHAIN_SERVICE_NAME)
	}
	handler, ok := svc.Methods[method]
	if !ok {
		return nil, fmt.Errorf("chain method %q not found", method)
	}
	return handler(sp.rpcCtx(), params)
}

func (sp *ServiceProvider) callVault(method string, params ...any) (any, error) {
	reg, ok := service.GetRegistryFromContext(sp.ctx)
	if !ok {
		return nil, fmt.Errorf("service registry not available")
	}
	svc, ok := reg.GetService(service.VAULT_SERVICE_NAME)
	if !ok || svc == nil {
		return nil, fmt.Errorf("vault service %q not registered", service.VAULT_SERVICE_NAME)
	}
	handler, ok := svc.Methods[method]
	if !ok {
		return nil, fmt.Errorf("vault method %q not found", method)
	}
	return handler(sp.rpcCtx(), params)
}

func (sp *ServiceProvider) vault() *storage.D5Vault {
	return storage.GetVault()
}

func (sp *ServiceProvider) AddBlock(b *block.Block) error {
	if b == nil {
		return fmt.Errorf("block is nil")
	}
	_, err := sp.callChain("updateChain", b)
	return err
}

// Reorg switches canonical chain and replays state via chain.ReorgHandler.
func (sp *ServiceProvider) Reorg(blocks []*block.Block) error {
	if sp.chainRef == nil {
		return fmt.Errorf("chain reference not configured")
	}
	return sp.chainRef.Reorg(blocks)
}

func (sp *ServiceProvider) ValidateOrphanBlock(b *block.Block, parent *block.Block) error {
	if sp.blockVal == nil {
		return fmt.Errorf("block validator not configured")
	}
	return sp.blockVal.ValidateOrphan(b, parent)
}

func (sp *ServiceProvider) GetBlockByHash(h common.Hash) *block.Block {
	res, err := sp.callChain("getBlockByHash", h.Hex())
	if err != nil {
		splogger.Debugw("GetBlockByHash failed", "hash", h, "error", err)
		return nil
	}
	b, _ := res.(*block.Block)
	return b
}

func (sp *ServiceProvider) GetBlockByHeight(height int) *block.Block {
	if sp.chainRef != nil {
		if b := sp.chainRef.GetBlockByHeight(height); b != nil {
			return b
		}
	}
	res, err := sp.callChain("getBlockByIndex", float64(height))
	if err != nil {
		splogger.Debugw("GetBlockByHeight failed", "height", height, "error", err)
		return nil
	}
	b, _ := res.(*block.Block)
	return b
}

func (sp *ServiceProvider) GetChainID() int {
	res, err := sp.callChain("getChainId")
	if err != nil {
		splogger.Errorw("GetChainID failed", "error", err)
		return 0
	}
	id, _ := res.(int)
	return id
}

func (sp *ServiceProvider) GetCurrentHeight() int {
	if sp.chainRef != nil {
		if head := sp.chainRef.GetLatestBlock(); head != nil && head.Head != nil {
			return head.Head.Height
		}
	}
	res, err := sp.callChain("getCurrentHeight")
	if err != nil {
		splogger.Errorw("GetCurrentHeight failed", "error", err)
		return 0
	}
	height, _ := res.(int)
	return height
}

func (sp *ServiceProvider) GetLatestHash() common.Hash {
	if sp.chainRef != nil {
		if head := sp.chainRef.GetLatestBlock(); head != nil {
			return head.Hash
		}
	}
	res, err := sp.callChain("getLatestBlock")
	if err != nil {
		splogger.Errorw("GetLatestHash failed", "error", err)
		return common.Hash{}
	}
	b, ok := res.(*block.Block)
	if !ok || b == nil {
		return common.Hash{}
	}
	return b.Hash
}

func (sp *ServiceProvider) GetStorageSize() int {
	if v := sp.vault(); v != nil {
		return v.GetCount()
	}
	res, err := sp.callVault("getCount")
	if err != nil {
		splogger.Debugw("GetStorageSize failed", "error", err)
		return 0
	}
	count, _ := res.(int)
	return count
}

func (sp *ServiceProvider) GetStorageServiceName() string {
	return storage.VAULT_SERVICE_NAME
}

func (sp *ServiceProvider) SetBlockValidator(bv *validation.BlockValidator) {
	sp.blockVal = bv
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

func (sp *ServiceProvider) ExportStorageAccountRange(offset, limit int) ([][]byte, int) {
	if v := sp.vault(); v != nil {
		return v.ExportAccountRangeSortedByAddress(offset, limit)
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
	v := sp.vault()
	if v == nil {
		return fmt.Errorf("vault not initialized")
	}
	for _, blob := range accounts {
		if len(blob) == 0 {
			continue
		}
		v.Sync(blob)
	}
	return nil
}
