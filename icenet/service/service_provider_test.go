package service

import (
	"context"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validation"
)

func TestServiceProviderGuards(t *testing.T) {
	sp, err := NewServiceProvider(context.Background())
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
	sp.SetVault(nil)
	sp.SetChainRef(nil)
	if sp.GetVaultSyncStats().Accounts != 0 {
		t.Fatal(sp.GetVaultSyncStats())
	}
}

func TestServiceProviderRPCAndChain(t *testing.T) {
	want := &block.Block{Head: &block.Header{Height: 3}, Hash: common.BytesToHash([]byte("rpc-block"))}
	registry, err := service.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(service.NewService(service.CHAIN_SERVICE_NAME, map[string]service.RPCHandler{
		"getBlockByHash": func(context.Context, []any) (any, error) { return want, nil },
		"getBlockByIndex": func(context.Context, []any) (any, error) {
			return "not-a-block", nil
		},
		"getChainId":       func(context.Context, []any) (any, error) { return 42, nil },
		"getCurrentHeight": func(context.Context, []any) (any, error) { return 0, nil },
		"getLatestBlock":   func(context.Context, []any) (any, error) { return want, nil },
		"updateChain":      func(context.Context, []any) (any, error) { return true, nil },
	})); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(service.NewService(service.VAULT_SERVICE_NAME, map[string]service.RPCHandler{
		"getCount": func(context.Context, []any) (any, error) { return 9, nil },
	})); err != nil {
		t.Fatal(err)
	}
	sp, err := NewServiceProvider(service.WithRegistry(context.Background(), registry))
	if err != nil {
		t.Fatal(err)
	}
	if sp.GetBlockByHash(want.Hash) != want {
		t.Fatal("hash lookup")
	}
	if sp.GetBlockByHeight(3) != nil {
		t.Fatal("wrong rpc type should be ignored")
	}
	if sp.GetChainID() != 42 || sp.GetCurrentHeight() != 0 || sp.GetLatestHash() != want.Hash {
		t.Fatalf("id=%d height=%d hash=%s", sp.GetChainID(), sp.GetCurrentHeight(), sp.GetLatestHash())
	}
	if err := sp.AddBlock(&block.Block{Head: &block.Header{Height: 1}}); err != nil {
		t.Fatal(err)
	}
	if sp.GetStorageSize() != 9 {
		t.Fatal(sp.GetStorageSize())
	}
	if sp.GetVaultSyncStats().Accounts != 9 {
		t.Fatal(sp.GetVaultSyncStats())
	}

	mismatch := &block.Block{Head: &block.Header{Height: 1, PrevHash: common.BytesToHash([]byte("other"))}}
	if err := sp.ValidateBlock(mismatch); err == nil {
		t.Fatal("expected prev hash mismatch")
	}

	cfg := &config.Config{Chain: config.ChainConfig{ChainID: 11, Path: "EMPTY"}, IN_MEM: true}
	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sp.SetChainRef(bc)
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
		t.Fatal("missing height should not use the string rpc result")
	}
	if sp.GetGenesisHash() == (common.Hash{}) {
		t.Fatal("genesis hash")
	}
	if err := sp.Reorg(nil); err == nil {
		t.Fatal("empty reorg")
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
	vault, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	concrete, ok := vault.(*storage.D5Vault)
	if !ok {
		t.Fatal("expected concrete vault")
	}
	sp, err := NewServiceProvider(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sp.SetVault(concrete)
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
