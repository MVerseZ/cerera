package validation

import (
	"math/big"
	"testing"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

func TestBlockValidator_ValidatePoW_GenesisSkip(t *testing.T) {
	bv := &BlockValidator{SkipPoWAtGenesis: true, ChainID: 11}
	genesis := block.NewBlock(block.GenesisHead(11))
	if !bv.ValidatePoW(genesis) {
		t.Fatal("genesis PoW should be skipped")
	}
}

func TestBlockValidator_ValidateContent_CoinbaseLayout(t *testing.T) {
	miner := types.HexToAddress("0x94F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")
	head := &block.Header{
		Ctx:        1,
		Difficulty: 100,
		GasLimit:   750000,
		GasUsed:    0,
		Height:     1,
		Index:      1,
		ChainId:    11,
		Node:       miner,
		PrevHash:   common.EmptyHash(),
		Root:       common.EmptyRootHash,
		Nonce:      1,
	}
	cb := coinbase.CreateCoinBaseTransation(1, miner)
	b := block.NewBlock(head)
	b.Transactions = []types.GTransaction{cb}

	bv := &BlockValidator{
		ChainID:          11,
		Signer:           types.NewSimpleSigner(big.NewInt(11)),
		SkipPoWAtGenesis: true,
		StateRootAfter: func(*block.Block) (common.Hash, error) {
			return common.EmptyRootHash, nil
		},
	}
	if err := bv.ValidateContent(b); err != nil {
		t.Fatalf("valid coinbase block rejected: %v", err)
	}
}

func TestBlockValidator_ValidateContent_GasUsedMismatch(t *testing.T) {
	miner := types.HexToAddress("0x94F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")
	head := &block.Header{
		GasLimit: 750000,
		GasUsed:  999,
		Height:   1,
		ChainId:  11,
		Node:     miner,
		Root:     common.EmptyRootHash,
	}
	cb := coinbase.CreateCoinBaseTransation(1, miner)
	b := block.NewBlock(head)
	b.Transactions = []types.GTransaction{cb}

	bv := &BlockValidator{ChainID: 11, Signer: types.NewSimpleSigner(big.NewInt(11))}
	if err := bv.ValidateContent(b); err == nil {
		t.Fatal("expected gasUsed mismatch error")
	}
}
