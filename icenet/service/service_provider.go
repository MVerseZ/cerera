package service

import (
	"context"
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/internal/service"
)

type ServiceProvider struct {
	ctx context.Context
}

func NewServiceProvider(ctx context.Context) (*ServiceProvider, error) {
	return &ServiceProvider{ctx: ctx}, nil
}

func (sp *ServiceProvider) AddBlock(b *block.Block) error { return nil }

func (sp *ServiceProvider) GetBlockByHash(h common.Hash) *block.Block { return nil }
func (sp *ServiceProvider) GetBlockByHeight(height int) *block.Block  { return nil }
func (sp *ServiceProvider) GetChainID() int {
	reg, ok := service.GetRegistryFromContext(sp.ctx)
	if !ok {
		fmt.Println("Do not get register")
	}
	chain, _ := reg.GetService("CHAIN_CERERA_001_1_7")
	return chain.GetID()
}
func (sp *ServiceProvider) GetCurrentHeight() int {
	ctx := context.Background()
	reg, ok := service.GetRegistryFromContext(sp.ctx)
	if !ok {
		fmt.Println("Do not get register")
	}
	chain, _ := reg.GetService("CHAIN_CERERA_001_1_7")
	handler := chain.Methods["getCurrentHeight"]
	res, _ := handler(ctx, nil)
	h := res.(int)
	return h
}
func (sp *ServiceProvider) GetLatestHash() common.Hash {
	ctx := context.Background()
	reg, ok := service.GetRegistryFromContext(sp.ctx)
	if !ok {
		fmt.Println("Do not get register")
	}
	chain, _ := reg.GetService("CHAIN_CERERA_001_1_7")
	handler := chain.Methods["getLatestBlock"]
	res, _ := handler(ctx, nil)
	h := res.(common.Hash)
	return h
}
func (sp *ServiceProvider) GetStorageSize() int                { return 0 }
func (sp *ServiceProvider) GetStorageServiceName() string      { return "D5Vault" }
func (sp *ServiceProvider) ValidateBlock(b *block.Block) error { return nil }

func (sp *ServiceProvider) ValidateBlockPoW(b *block.Block) bool { return true }

func (sp *ServiceProvider) ExportStorageAccountRange(offset, limit int) ([][]byte, int) {
	// This is a stub implementation. In a real implementation, this method would interact with the storage system to retrieve account data.
	// For demonstration purposes, we return an empty slice and zero total.
	return [][]byte{}, 0
}

func (sp *ServiceProvider) ImportStorageAccounts(accounts [][]byte) error {
	// This is a stub implementation. In a real implementation, this method would interact with the storage system to import account data.
	// For demonstration purposes, we simply return nil to indicate success.
	return nil
}
func (sp *ServiceProvider) GetGenesisBlock() *block.Block {
	// This is a stub implementation. In a real implementation, this method would retrieve the genesis block from the blockchain.
	// For demonstration purposes, we return nil to indicate that the genesis block is not available.
	return nil
}
func (sp *ServiceProvider) GetGenesisHash() common.Hash {
	// This is a stub implementation. In a real implementation, this method would retrieve the genesis block's hash from the blockchain.
	// For demonstration purposes, we return an empty hash to indicate that the genesis hash is not available.
	return common.Hash{}
}
func (sp *ServiceProvider) ApplyStorageAccounts(accounts [][]byte) error {
	// This is a stub implementation. In a real implementation, this function would apply the provided storage accounts to the ServiceProvider.
	// For demonstration purposes, we simply return nil to indicate success.
	return nil
}
