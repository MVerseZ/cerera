package service

import (
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/validation"
)

func TestServiceProviderGuards(t *testing.T) {
	sp, err := NewServiceProvider(nil, nil, nil)
	if err != nil || sp == nil {
		t.Fatal(err)
	}
	if err := sp.AddBlock(nil); err == nil {
		t.Fatal("nil block")
	}
	if err := sp.Reorg(nil); err == nil {
		t.Fatal("missing chain")
	}
	if err := sp.ValidateOrphanBlock(&block.Block{}, nil); err == nil {
		t.Fatal("missing validator")
	}
	if sp.GetBlockByHash(common.Hash{}) != nil || sp.GetBlockByHeight(1) != nil {
		t.Fatal("expected nil blocks")
	}
	if sp.GetChainID() != 0 || sp.GetCurrentHeight() != 0 || sp.GetLatestHash() != (common.Hash{}) {
		t.Fatal("expected zero chain info")
	}
	if sp.GetStorageSize() != 0 || sp.GetStorageServiceName() != storage.VAULT_SERVICE_NAME {
		t.Fatal("storage defaults")
	}
	if err := sp.ValidateBlock(nil); err == nil {
		t.Fatal("nil block")
	}
	if err := sp.ValidateBlock(&block.Block{Head: &block.Header{Height: 4}}); err == nil {
		t.Fatal("expected height mismatch")
	}
	if sp.ValidateBlockPoW(nil) || !sp.ValidateBlockPoW(&block.Block{Head: &block.Header{}}) {
		t.Fatal("pow without validator")
	}
	if blobs, n := sp.ExportStorageAccountRange(0, 1); blobs != nil || n != 0 {
		t.Fatal("accounts")
	}
	if blobs, n := sp.ExportStorageContractCodeRange(0, 1); blobs != nil || n != 0 {
		t.Fatal("code")
	}
	if blobs, n := sp.ExportStorageContractStorageRange(0, 1); blobs != nil || n != 0 {
		t.Fatal("storage slots")
	}
	if err := sp.ImportStorageAccounts(nil); err == nil {
		t.Fatal("nil vault")
	}
	if err := sp.ApplyStorageContractCodes([][]byte{{1}}); err == nil {
		t.Fatal("nil vault")
	}
	if err := sp.ApplyStorageContractStorage(nil); err == nil {
		t.Fatal("nil vault")
	}
	if sp.GetGenesisBlock() != nil || sp.GetGenesisHash() != (common.Hash{}) {
		t.Fatal("genesis")
	}
	sp.EnsureAccount(types.Address{})
	sp.SetBlockValidator(nil)
	if sp.GetVaultSyncStats().Accounts != 0 {
		t.Fatal(sp.GetVaultSyncStats())
	}
}

func TestServiceProviderChainDirect(t *testing.T) {
	cfg := &config.Config{Chain: config.ChainConfig{ChainID: 11, Path: "EMPTY"}, IN_MEM: true}
	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := NewServiceProvider(bc, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sp.GetCurrentHeight() != bc.GetLatestBlock().Head.Height {
		t.Fatal("chain height")
	}
	if sp.GetLatestHash() != bc.GetLatestBlock().Hash {
		t.Fatal("chain hash")
	}
	if sp.GetBlockByHeight(0) == nil {
		t.Fatal("genesis via chain")
	}
	if sp.GetBlockByHeight(99) != nil {
		t.Fatal("missing height")
	}
	if sp.GetGenesisHash() == (common.Hash{}) {
		t.Fatal("genesis hash")
	}
	if err := sp.Reorg(nil); err == nil {
		t.Fatal("empty reorg")
	}
	if sp.GetChainID() != bc.GetChainId() {
		t.Fatal("chain id")
	}

	sp.SetBlockValidator(&validation.BlockValidator{ChainID: 11, SkipPoWAtGenesis: true})
	if !sp.ValidateBlockPoW(&block.Block{Head: &block.Header{Height: 0}}) {
		t.Fatal("genesis pow skip")
	}
	if sp.ValidateBlockPoW(nil) {
		t.Fatal("nil pow")
	}
	if err := sp.ValidateOrphanBlock(nil, nil); err == nil {
		t.Fatal("nil orphan")
	}
	head := bc.GetLatestBlock()
	gasBlock := &block.Block{Head: &block.Header{
		Height:   head.Head.Height + 1,
		PrevHash: head.Hash,
		GasUsed:  5,
		GasLimit: 1,
		ChainId:  11,
	}}
	if err := sp.ValidateBlock(gasBlock); err == nil {
		t.Fatal("expected gas error")
	}
}

func TestServiceProviderVaultApply(t *testing.T) {
	cfg := &config.Config{
		Vault:  config.VaultConfig{MEM: true, PATH: "EMPTY"},
		IN_MEM: true,
	}
	vault, err := storage.NewD5Vault(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	concrete, ok := vault.(*storage.D5Vault)
	if !ok {
		t.Fatal("expected concrete vault")
	}
	sp, err := NewServiceProvider(nil, concrete, nil)
	if err != nil {
		t.Fatal(err)
	}
	sp.EnsureAccount(types.Address{})
	if sp.GetStorageSize() < 1 {
		t.Fatal(sp.GetStorageSize())
	}
	if _, n := sp.ExportStorageAccountRange(0, 10); n < 1 {
		t.Fatal(n)
	}
	if _, n := sp.ExportStorageContractCodeRange(0, 10); n != 0 {
		t.Fatal(n)
	}
	if _, n := sp.ExportStorageContractStorageRange(0, 10); n != 0 {
		t.Fatal(n)
	}
	if err := sp.ApplyStorageAccounts([][]byte{nil, {1, 2, 3}}); err != nil {
		t.Fatal(err)
	}
	if err := sp.ApplyStorageContractCodes([][]byte{{}}); err != nil {
		t.Fatal(err)
	}
	if err := sp.ApplyStorageContractCodes([][]byte{{9}}); err == nil {
		t.Fatal("expected bad code blob")
	}
	if err := sp.ApplyStorageContractStorage([][]byte{{}}); err != nil {
		t.Fatal(err)
	}
	if err := sp.ApplyStorageContractStorage([][]byte{{9}}); err == nil {
		t.Fatal("expected bad storage blob")
	}
	if sp.GetVaultSyncStats().Accounts < 1 {
		t.Fatal(sp.GetVaultSyncStats())
	}
}
