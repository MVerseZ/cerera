package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const (
	vaultKeysFileName     = ".vault_keys"
	defaultVaultKeyPass   = "NODE_PASS"
	envVaultMnemonic      = "VAULT_MNEMONIC"
	envVaultPassphrase    = "VAULT_PASSPHRASE"
)

type vaultKeysFile struct {
	Mnemonic   string `json:"mnemonic"`
	Passphrase string `json:"passphrase"`
}

// InitVaultKeys generates ephemeral keys (in-memory vault). Disk vaults must use ensureVaultKeys.
func InitVaultKeys(v *D5Vault) error {
	if err := setKeysFromMnemonic(mustNewMnemonic(), defaultVaultKeyPass); err != nil {
		if v != nil {
			v.status = 0xf
		}
		return err
	}
	return nil
}

func mustNewMnemonic() string {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		panic(err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		panic(err)
	}
	return mnemonic
}

func setKeysFromMnemonic(mnemonic, pass string) error {
	if pass == "" {
		pass = defaultVaultKeyPass
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("invalid vault mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return fmt.Errorf("vault master key: %w", err)
	}
	publicKey := masterKey.PublicKey()
	if err := SetKeys(masterKey, publicKey); err != nil {
		return err
	}
	vltlogger().Infow("Vault keys loaded",
		"masterKey", masterKey.B58Serialize(),
		"publicKey", publicKey.B58Serialize(),
	)
	return nil
}

// ensureVaultKeys loads stable encryption keys from the vault directory, or creates them once.
func ensureVaultKeys(vaultDir string) error {
	if envMnemonic := os.Getenv(envVaultMnemonic); envMnemonic != "" {
		pass := os.Getenv(envVaultPassphrase)
		if pass == "" {
			pass = defaultVaultKeyPass
		}
		vltlogger().Infow("Vault keys from environment", "env", envVaultMnemonic)
		return setKeysFromMnemonic(envMnemonic, pass)
	}

	keyPath := filepath.Join(vaultDir, vaultKeysFileName)
	if data, err := os.ReadFile(keyPath); err == nil {
		var stored vaultKeysFile
		if err := json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("parse %s: %w", vaultKeysFileName, err)
		}
		if stored.Passphrase == "" {
			stored.Passphrase = defaultVaultKeyPass
		}
		vltlogger().Infow("Vault keys loaded from file", "path", keyPath)
		return setKeysFromMnemonic(stored.Mnemonic, stored.Passphrase)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", keyPath, err)
	}

	mnemonic := mustNewMnemonic()
	if err := setKeysFromMnemonic(mnemonic, defaultVaultKeyPass); err != nil {
		return err
	}
	payload, err := json.Marshal(vaultKeysFile{
		Mnemonic:   mnemonic,
		Passphrase: defaultVaultKeyPass,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, payload, 0600); err != nil {
		return fmt.Errorf("write %s: %w", keyPath, err)
	}
	vltlogger().Infow("Vault encryption keys created",
		"path", keyPath,
		"mnemonic", mnemonic,
		"passphrase", defaultVaultKeyPass,
	)
	return nil
}
