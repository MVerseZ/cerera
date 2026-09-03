package validation

import (
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
)

// BlockValidator performs structural, PoW, and state-root checks on blocks.
type BlockValidator struct {
	ChainID          int
	Signer           types.Signer
	StateRootAfter   func(*block.Block) (common.Hash, error)
	SkipPoWAtGenesis bool
}

// ValidatePoW verifies proof-of-work unless genesis skip is enabled.
func (bv *BlockValidator) ValidatePoW(b *block.Block) bool {
	if b == nil || b.Head == nil {
		return false
	}
	if bv.SkipPoWAtGenesis && b.Head.Height == 0 {
		return true
	}
	ok, err := block.VerifyBlockHash(b)
	return err == nil && ok
}

// ValidateContent checks header fields, transaction layout, gas accounting, and state root.
func (bv *BlockValidator) ValidateContent(b *block.Block) error {
	if b == nil || b.Head == nil {
		return fmt.Errorf("block is nil")
	}
	head := b.Head
	if head.GasUsed > head.GasLimit {
		return fmt.Errorf("gasUsed %d exceeds gasLimit %d", head.GasUsed, head.GasLimit)
	}
	if head.ChainId != bv.ChainID {
		return fmt.Errorf("chain id mismatch: got %d want %d", head.ChainId, bv.ChainID)
	}

	if head.Height > 0 && b.Hash == (common.Hash{}) {
		computed, err := b.CalculateHash()
		if err != nil {
			return fmt.Errorf("calculate block hash: %w", err)
		}
		b.Hash = common.BytesToHash(computed)
	}

	txs := b.Transactions
	if len(txs) == 0 {
		if head.Height == 0 {
			return bv.verifyStateRoot(b)
		}
		return fmt.Errorf("block has no transactions")
	}

	last := txs[len(txs)-1]
	if err := ValidateCoinbaseTx(last, head.Node); err != nil {
		return err
	}
	coinbaseCount := 0
	for i := range txs {
		if txs[i].Type() == types.CoinbaseTxType {
			coinbaseCount++
		}
	}
	if coinbaseCount != 1 {
		return fmt.Errorf("expected exactly one coinbase transaction, got %d", coinbaseCount)
	}

	var gasSum uint64
	vault := storage.GetVault()
	var snap storage.AccountSnapshot
	if vault != nil {
		snap = vault.SnapshotAccounts()
	}

	for i := 0; i < len(txs)-1; i++ {
		tx := txs[i]
		switch tx.Type() {
		case types.LegacyTxType:
			if snap == nil {
				if err := ValidateBlockLegacyTx(bv.Signer, tx); err != nil {
					return fmt.Errorf("tx %s: %w", tx.Hash().Hex(), err)
				}
			} else if err := ApplyLegacyTxSimulation(vault, bv.Signer, tx); err != nil {
				return fmt.Errorf("tx %s: %w", tx.Hash().Hex(), err)
			}
			gasSum += uint64(tx.Gas())
		case types.FaucetTxType:
			// allowed in blocks
		case types.CoinbaseTxType:
			return fmt.Errorf("coinbase must be the last transaction")
		default:
			return fmt.Errorf("unsupported transaction type %d in block", tx.Type())
		}
	}

	if gasSum != head.GasUsed {
		return fmt.Errorf("gasUsed mismatch: header %d sum %d", head.GasUsed, gasSum)
	}

	if vault != nil && len(snap) > 0 {
		vault.RestoreAccounts(snap)
	}

	return bv.verifyStateRoot(b)
}

func (bv *BlockValidator) verifyStateRoot(b *block.Block) error {
	if bv.StateRootAfter == nil {
		return nil
	}
	expected := b.Head.Root
	computed, err := bv.StateRootAfter(b)
	if err != nil {
		return fmt.Errorf("state root simulation: %w", err)
	}
	if computed != expected {
		return fmt.Errorf("state root mismatch: got %s want %s", computed.Hex(), expected.Hex())
	}
	return nil
}

// ValidateOrphan validates a block against an explicit parent (fork/orphan path).
func (bv *BlockValidator) ValidateOrphan(b *block.Block, parent *block.Block) error {
	if b == nil || b.Head == nil {
		return fmt.Errorf("block is nil")
	}
	if !bv.ValidatePoW(b) {
		return fmt.Errorf("invalid proof of work")
	}
	if parent == nil {
		if b.Head.Height != 0 {
			return fmt.Errorf("orphan block missing parent")
		}
	} else {
		if parent.Head == nil {
			return fmt.Errorf("parent header nil")
		}
		if b.Head.PrevHash != parent.Hash {
			return fmt.Errorf("prevHash mismatch")
		}
		if b.Head.Height != parent.Head.Height+1 {
			return fmt.Errorf("unexpected orphan height %d, want %d", b.Head.Height, parent.Head.Height+1)
		}
	}
	return bv.ValidateContent(b)
}
