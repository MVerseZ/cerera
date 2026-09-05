package validation

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/cerera/core/account"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrTxUnsigned         = errors.New("transaction is not signed")
	ErrTxInvalidSender      = errors.New("transaction sender mismatch")
	ErrTxAlreadyInChain     = errors.New("transaction already in chain")
	ErrTxTypeNotAllowed     = errors.New("transaction type not allowed in mempool")
	ErrTxMissingRecipient   = errors.New("transaction missing recipient")
	ErrTxInsufficientBal    = errors.New("insufficient balance")
	ErrTxGasTooLow          = errors.New("gas below minimum")
	ErrTxBadNonce           = errors.New("invalid transaction nonce")
)

// ValidateLegacyTx checks balance, gas, nonce, and recipient for a legacy transfer.
func ValidateLegacyTx(vault *storage.D5Vault, tx types.GTransaction, sender types.Address, checkNonce bool) error {
	if tx.To() == nil {
		return ErrTxMissingRecipient
	}
	gasLimit := uint64(tx.Gas())
	if gasLimit > 0 && gasLimit < pallada.MinTransferGas {
		return fmt.Errorf("%w: got %d, need %d", ErrTxGasTooLow, gasLimit, pallada.MinTransferGas)
	}
	senderAcc := vault.Get(sender)
	if senderAcc == nil {
		return ErrTxInsufficientBal
	}
	if checkNonce && tx.Nonce() != senderAcc.Nonce {
		return fmt.Errorf("%w: got %d, want %d", ErrTxBadNonce, tx.Nonce(), senderAcc.Nonce)
	}
	val := tx.Value()
	gasCost := tx.Cost()
	totalDebit := new(big.Int).Add(new(big.Int).Set(val), gasCost)
	if senderAcc.GetBalanceBI().Cmp(totalDebit) < 0 {
		return ErrTxInsufficientBal
	}
	return nil
}

// RecoverSender resolves and caches the transaction sender using the given signer.
func RecoverSender(signer types.Signer, tx *types.GTransaction) (types.Address, error) {
	if !tx.IsSigned() {
		return types.Address{}, ErrTxUnsigned
	}
	return types.Sender(signer, tx)
}

// ValidateMempoolTx applies admission rules for transactions entering the mempool or P2P.
func ValidateMempoolTx(signer types.Signer, tx types.GTransaction) error {
	switch tx.Type() {
	case types.LegacyTxType:
		// allowed
	case types.FaucetTxType, types.CoinbaseTxType, types.AppTxType:
		return ErrTxTypeNotAllowed
	default:
		return fmt.Errorf("unknown transaction type: %d", tx.Type())
	}

	if storage.GetTxTable().Get(tx.Hash()) != -1 {
		return ErrTxAlreadyInChain
	}

	sender, err := RecoverSender(signer, &tx)
	if err != nil {
		return err
	}
	tx.SetFrom(sender)

	vault := storage.GetVault()
	if vault == nil {
		return errors.New("vault not initialized")
	}
	return ValidateLegacyTx(vault, tx, sender, true)
}

// ValidateCoinbaseTx checks the trailing coinbase transaction for a block.
func ValidateCoinbaseTx(tx types.GTransaction, minerAddr types.Address) error {
	if tx.Type() != types.CoinbaseTxType {
		return fmt.Errorf("expected coinbase transaction, got type %d", tx.Type())
	}
	if tx.To() == nil || *tx.To() != minerAddr {
		return fmt.Errorf("coinbase recipient must be block miner")
	}
	return nil
}

// ApplyLegacyTxSimulation updates vault balances and nonce for sequential in-block checks.
func ApplyLegacyTxSimulation(vault *storage.D5Vault, signer types.Signer, tx types.GTransaction) error {
	sender, err := RecoverSender(signer, &tx)
	if err != nil {
		return err
	}
	if err := ValidateLegacyTx(vault, tx, sender, true); err != nil {
		return err
	}
	senderAcc := vault.Get(sender)
	gasCost := tx.Cost()
	val := tx.Value()
	senderBal := senderAcc.GetBalanceBI()
	senderAcc.SetBalanceBI(new(big.Int).Sub(senderBal, gasCost))
	vault.UpdateBalance(sender, *tx.To(), val, tx.Hash())
	senderAcc.Nonce++
	return nil
}

// ValidateBlockLegacyTx validates a legacy tx inside a block (signature + economics).
func ValidateBlockLegacyTx(signer types.Signer, tx types.GTransaction) error {
	if tx.Type() != types.LegacyTxType {
		return fmt.Errorf("expected legacy transaction in block")
	}
	sender, err := RecoverSender(signer, &tx)
	if err != nil {
		return err
	}
	tx.SetFrom(sender)
	vault := storage.GetVault()
	if vault == nil {
		return errors.New("vault not initialized")
	}
	return ValidateLegacyTx(vault, tx, sender, true)
}

// ConsensusAccountLeaf is blake2b-256(address || balance32 || nonce8).
func ConsensusAccountLeaf(acc *account.StateAccount) []byte {
	if acc == nil {
		return nil
	}
	var buf bytes.Buffer
	buf.Write(acc.Address.Bytes())
	buf.Write(padBigIntBytes(acc.GetBalanceBI().Bytes(), 32))
	_ = binary.Write(&buf, binary.LittleEndian, acc.Nonce)
	sum := blake2b.Sum256(buf.Bytes())
	return sum[:]
}

func padBigIntBytes(b []byte, width int) []byte {
	if len(b) >= width {
		return b[len(b)-width:]
	}
	out := make([]byte, width)
	copy(out[width-len(b):], b)
	return out
}
