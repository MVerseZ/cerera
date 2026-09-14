package validator

import (
	"context"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

func testValidatorForStateRoot(t *testing.T) (*CoreValidator, *storage.D5Vault, *chain.Chain) {
	t.Helper()

	cfg := &config.Config{
		IN_MEM: true,
		Chain:  config.ChainConfig{ChainID: 11, Path: "EMPTY"},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x34435Dd4078275B01D76A456FF9599fc07e61B90"),
			PRIV: testValidatorPrivPEM(t),
		},
	}

	vaultIface, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault: %v", err)
	}
	vault := vaultIface.(*storage.D5Vault)
	vault.SaveBaseline()

	bc, err := chain.Mold(cfg)
	if err != nil {
		t.Fatalf("Mold: %v", err)
	}

	valIface, err := NewValidator(context.Background(), *cfg, vault, storage.NewTxTable())
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	val := valIface.(*CoreValidator)
	val.SetChain(bc)
	return val, vault, bc
}

func testValidatorPrivPEM(t *testing.T) string {
	t.Helper()
	key, err := types.GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount: %v", err)
	}
	return types.EncodePrivateKeyToToString(key)
}

func TestSimulateBlockStateRoot_CoinbaseHeight1(t *testing.T) {
	val, _, bc := testValidatorForStateRoot(t)
	miner := val.currentAddress

	genesis := bc.GetLatestBlock()
	head := &block.Header{
		Ctx:        genesis.Head.Ctx,
		Difficulty: genesis.Head.Difficulty,
		GasLimit:   genesis.Head.GasLimit,
		GasUsed:    0,
		Height:     1,
		Index:      1,
		ChainId:    genesis.Head.ChainId,
		Node:       miner,
		PrevHash:   genesis.Hash,
		Root:       common.EmptyRootHash,
		Nonce:      genesis.Head.Nonce,
	}
	cb := coinbase.CreateCoinBaseTransation(genesis.Head.Nonce, miner)
	b := block.NewBlock(head)
	b.Transactions = []types.GTransaction{cb}

	root1, err := val.ComputeBlockStateRoot(b)
	if err != nil {
		t.Fatalf("ComputeBlockStateRoot: %v", err)
	}
	root2, err := val.ComputeBlockStateRoot(b)
	if err != nil {
		t.Fatalf("second ComputeBlockStateRoot: %v", err)
	}
	if root1 != root2 {
		t.Fatalf("state root not stable: %s vs %s", root1, root2)
	}

	b.Head.Root = root1
	if err := val.ValidateBlockContent(b); err != nil {
		t.Fatalf("ValidateBlockContent: %v", err)
	}
}

func TestSimulateBlockStateRoot_MinerPathMatchesValidation(t *testing.T) {
	val, vault, bc := testValidatorForStateRoot(t)
	miner := val.currentAddress
	genesis := bc.GetLatestBlock()

	head := &block.Header{
		Ctx:        genesis.Head.Ctx,
		Difficulty: genesis.Head.Difficulty,
		GasLimit:   genesis.Head.GasLimit,
		GasUsed:    0,
		Height:     1,
		Index:      1,
		ChainId:    genesis.Head.ChainId,
		Node:       miner,
		PrevHash:   genesis.Hash,
		Root:       common.EmptyRootHash,
		Nonce:      genesis.Head.Nonce,
	}
	cb := coinbase.CreateCoinBaseTransation(genesis.Head.Nonce, miner)
	b := block.NewBlock(head)
	b.Transactions = []types.GTransaction{cb}

	root, err := val.ComputeBlockStateRoot(b)
	if err != nil {
		t.Fatalf("ComputeBlockStateRoot: %v", err)
	}
	b.Head.Root = root

	// Simulate concurrent vault reader like EnsureAccount during validation.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				vault.EnsureAccount(types.HexToAddress("0x1000000000000000000000000000000000000000000000000000000000000099"))
			}
		}
	}()

	if err := val.ValidateBlockContent(b); err != nil {
		t.Fatalf("ValidateBlockContent with concurrent vault access: %v", err)
	}
	close(done)
}

func TestSimulateBlockStateRoot_FaucetFundedTransfers(t *testing.T) {
	val, vault, bc := testValidatorForStateRoot(t)
	miner := val.currentAddress
	genesis := bc.GetLatestBlock()

	senderKey, err := types.GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount: %v", err)
	}
	sender := types.PubkeyToAddress(&senderKey.PublicKey)
	receiver := types.HexToAddress("0x2000000000000000000000000000000000000000000000000000000000000002")

	if err := vault.DropFaucet(sender, types.FloatToBigInt(10), common.BytesToHash([]byte("faucet_sender"))); err != nil {
		t.Fatalf("DropFaucet: %v", err)
	}

	var txs []types.GTransaction
	nonce := uint64(1)
	if acc := vault.Get(sender); acc != nil {
		nonce = acc.Nonce
	}
	for i := 0; i < 3; i++ {
		tx, err := types.CreateUnbroadcastTransaction(nonce+uint64(i), receiver, 1, 21000, val.GasPrice(), "test_send")
		if err != nil {
			t.Fatalf("create tx %d: %v", i, err)
		}
		signed, err := types.SignTx(tx, val.Signer(), senderKey)
		if err != nil {
			t.Fatalf("sign tx %d: %v", i, err)
		}
		txs = append(txs, *signed)
	}

	head := &block.Header{
		Ctx:        genesis.Head.Ctx,
		Difficulty: genesis.Head.Difficulty,
		GasLimit:   genesis.Head.GasLimit,
		GasUsed:    63000,
		Height:     1,
		Index:      1,
		ChainId:    genesis.Head.ChainId,
		Node:       miner,
		PrevHash:   genesis.Hash,
		Root:       common.EmptyRootHash,
		Nonce:      genesis.Head.Nonce,
	}
	cb := coinbase.CreateCoinBaseTransation(genesis.Head.Nonce, miner)
	b := block.NewBlock(head)
	b.Transactions = append(txs, cb)

	root, err := val.ComputeBlockStateRoot(b)
	if err != nil {
		t.Fatalf("ComputeBlockStateRoot after faucet+3 transfers: %v", err)
	}
	b.Head.Root = root
	if err := val.ValidateBlockContent(b); err != nil {
		t.Fatalf("ValidateBlockContent: %v", err)
	}
}
