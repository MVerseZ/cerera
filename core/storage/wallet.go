package storage

import (
	"fmt"
	"strings"

	"github.com/cerera/core/account"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
)

// Create generates a new account and stores it in the vault.
func (v *D5Vault) Create(pass string) (string, string, string, *types.Address, error) {
	if pass == "" {
		return "", "", "", nil, ErrMnemonicEmpty
	}

	privateKey, err := types.GenerateAccount()
	if err != nil {
		return "", "", "", nil, err
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
	return "", crypto.EncodePublicKeyToString(&privateKey.PublicKey), "", &address, nil
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

	return types.EmptyAddress(), "", ErrErrorWhileRestore
}

func (v *D5Vault) VerifyAccount(addr types.Address, pass string) (types.Address, error) {
	acc := v.accounts.GetAccount(addr)
	if acc == nil {
		return types.EmptyAddress(), ErrAccountNotFound
	}
	return acc.Address, nil
}

func (v *D5Vault) execRestore(params []any) any {
	mnemonic, ok1 := params[0].(string)
	pass, ok2 := params[1].(string)
	if !ok1 || !ok2 {
		return ErrErrorParsingParameters.Error()
	}
	if strings.Count(mnemonic, " ") != 23 {
		return ErrWrongWordsCount.Error()
	}
	addr, pk, err := v.Restore(mnemonic, pass)
	if err != nil {
		return err.Error()
	}
	type res struct {
		Addr types.Address `json:"address,omitempty"`
		Priv string        `json:"priv,omitempty"`
		Pub  string        `json:"pub,omitempty"`
	}
	return &res{Priv: pk, Addr: addr}
}
