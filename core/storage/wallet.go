package storage

import (
	"fmt"

	"github.com/cerera/core/account"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
)

// Create generates a new account and stores it in the vault.
func (v *D5Vault) Create(pass string) (string, string, string, *types.Address, error) {
	if pass == "" {
		return "", "", "", nil, ErrMnemonicEmpty
	}

	mnemonic, err := types.NewWalletMnemonic()
	if err != nil {
		return "", "", "", nil, fmt.Errorf("%w: %s", ErrGenerateMnemonicFailed, err.Error())
	}
	privateKey, err := types.PrivateKeyFromMnemonic(mnemonic, pass)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("%w: %s", ErrGenerateMnemonicFailed, err.Error())
	}
	pubkey := &privateKey.PublicKey
	address := types.PubkeyToAddress(pubkey)

	if existing := v.accounts.GetAccount(address); existing != nil {
		return "", "", "", nil, fmt.Errorf("%w: %s", ErrAddressAlreadyExists, address.Hex())
	}

	newAccount := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: address,
			Nonce:   1,
		},
		Status: 0,
	}
	newAccount.SetBalance(0.0)
	v.accounts.Append(address, newAccount)
	vaultAccountsTotal.Set(float64(v.accounts.GetCount()))

	v.markDirty(address)
	if err := v.persistAccount(address, newAccount); err != nil {
		vltlogger().Errorw("Failed to save account to vault", "address", address.Hex(), "err", err)
		return "", "", "", nil, fmt.Errorf("failed to save account to vault: %w", err)
	}

	notifyAccountCreated(newAccount)
	return crypto.EncodePrivateKeyToToString(privateKey), crypto.EncodePublicKeyToString(pubkey), mnemonic, &address, nil
}

// Restore recovers an account from a mnemonic phrase.
func (v *D5Vault) Restore(mnemonic string, pass string) (types.Address, string, error) {
	if mnemonic == "" {
		return types.EmptyAddress(), "", ErrMnemonicEmpty
	}

	if !v.inMem && v.db != nil {
		if err := v.SyncFromDB(); err != nil {
			return types.EmptyAddress(), "", fmt.Errorf("failed to sync vault: %v", err)
		}
	}

	privateKey, err := types.PrivateKeyFromMnemonic(mnemonic, pass)
	if err != nil {
		if err == crypto.ErrInvalidMnemonic {
			return types.EmptyAddress(), "", fmt.Errorf("%w: %s", ErrErrorWhileRestore, err.Error())
		}
		return types.EmptyAddress(), "", fmt.Errorf("%w: %v", ErrErrorWhileRestore, err)
	}
	address := types.PubkeyToAddress(&privateKey.PublicKey)
	return address, crypto.EncodePrivateKeyToToString(privateKey), nil
}

func (v *D5Vault) VerifyAccount(addr types.Address, pass string) (types.Address, error) {
	acc := v.accounts.GetAccount(addr)
	if acc == nil {
		return types.EmptyAddress(), ErrAccountNotFound
	}
	return acc.Address, nil
}
