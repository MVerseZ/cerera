package storage

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// Create generates a new account and stores it in the vault.
func (v *D5Vault) Create(pass string) (string, string, string, *types.Address, error) {
	masterKey, mnemonic, err := crypto.GenerateMasterKey(pass)
	if err != nil {
		return "", "", "", nil, err
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
	masterKeyBytes, _ := masterKey.Serialize()
	masterKeyHash := common.BytesToHash(masterKeyBytes)

	etaKeyBytes := crypto.EncodePrivateKeyToByte(privateKey)
	xorResult := crypto.Xor(privateKey, masterKey)

	newAccount := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: address,
			Nonce:   1,
			Root:    v.rootHash,
			KeyHash: masterKeyHash,
			Data:    xorResult,
		},
		Status: 0,
		Bloom:  []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &account.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte(pass)),
	}
	newAccount.SetBalance(0.0)
	v.accounts.Append(address, newAccount)
	vaultAccountsTotal.Set(float64(v.accounts.GetCount()))

	v.markDirty(address)
	if err := v.persistAccount(address, newAccount); err != nil {
		vltlogger().Errorw("Failed to save account to vault", "address", address.Hex(), "err", err)
		return "", "", "", nil, fmt.Errorf("failed to save account to vault: %w", err)
	}

	restoredPrivateKey := crypto.RXor(masterKey, xorResult)
	vltlogger().Infow("Account created", "restored private key without offset", restoredPrivateKey)
	vltlogger().Infow("Account created", "keys equality", bytes.Equal(restoredPrivateKey, etaKeyBytes))

	notifyAccountCreated(newAccount)
	return crypto.EncodePrivateKeyToToString(privateKey), crypto.EncodePublicKeyToString(&privateKey.PublicKey), mnemonic, &address, nil
}

// Restore recovers an account from a mnemonic phrase.
func (v *D5Vault) Restore(mnemonic string, pass string) (types.Address, string, error) {
	if mnemonic == "" {
		return types.EmptyAddress(), "", ErrMnemonicEmpty
	}

	if !v.inMem && v.db != nil {
		if err := v.SyncFromDB(); err != nil {
			return types.EmptyAddress(), "", fmt.Errorf("failed to sync vault: %w", err)
		}
	}

	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return types.EmptyAddress(), "", fmt.Errorf("%w: %v", ErrFailedCreateMasterKey, err)
	}

	masterKeyBytes, _ := masterKey.Serialize()
	acc, err := v.accounts.FindByKeyHash(common.BytesToHash(masterKeyBytes))
	if err != nil {
		return types.EmptyAddress(), "", fmt.Errorf("%w: %v", ErrAccountNotFound, err)
	}

	privateKeyBytes := crypto.RXor(masterKey, acc.Data)
	privateKey, err := crypto.DecodeBytesToPrivateKey(privateKeyBytes)
	if err != nil {
		return types.EmptyAddress(), "", fmt.Errorf("failed to decode private key: %w", err)
	}
	return acc.Address, crypto.EncodePrivateKeyToToString(privateKey), nil
}

func (v *D5Vault) VerifyAccount(addr types.Address, pass string) (types.Address, error) {
	acc := v.accounts.GetAccount(addr)
	if acc == nil {
		return types.EmptyAddress(), ErrAccountNotFound
	}
	if acc.Passphrase == common.BytesToHash([]byte(pass)) {
		return acc.Address, nil
	}
	return types.EmptyAddress(), ErrWrongCredentials
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
