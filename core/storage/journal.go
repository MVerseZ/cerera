package storage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/akrylysov/pogreb"
	"github.com/cerera/core/account"
	"github.com/cerera/core/types"
)

// CRV1 prefix marks AES-CFB encrypted StateAccount payloads.
var vaultAccountEncMagic = []byte{'C', 'R', 'V', 1}

func isContractOrStorageKey(key []byte) bool {
	return bytes.HasPrefix(key, []byte("code:")) || bytes.HasPrefix(key, []byte("storage:"))
}

func deriveVaultAccountEncKey() []byte {
	priv, _, err := GetKeys()
	if err != nil || priv == nil {
		return nil
	}
	ser, err := priv.Serialize()
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(ser)
	return sum[:]
}

func encodeAccountPayload(plain []byte) ([]byte, error) {
	key := deriveVaultAccountEncKey()
	if key == nil {
		return nil, fmt.Errorf("vault encryption keys are not loaded")
	}
	ct, err := encrypt(plain, key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(vaultAccountEncMagic)+len(ct))
	copy(out, vaultAccountEncMagic)
	copy(out[len(vaultAccountEncMagic):], ct)
	return out, nil
}

func decodeAccountPayload(stored []byte) ([]byte, error) {
	if len(stored) < len(vaultAccountEncMagic) || !bytes.HasPrefix(stored, vaultAccountEncMagic) {
		return nil, fmt.Errorf("account record is not CRV1 encrypted")
	}
	key := deriveVaultAccountEncKey()
	if key == nil {
		return nil, fmt.Errorf("encrypted account record but vault keys are not loaded")
	}
	return decrypt(stored[len(vaultAccountEncMagic):], key)
}

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)
	return ciphertext, nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return ciphertext, nil
}

func putAccountPayload(db *pogreb.DB, addressKey []byte, serializedAccount []byte) error {
	payload, err := encodeAccountPayload(serializedAccount)
	if err != nil {
		return err
	}
	return db.Put(addressKey, payload)
}

func (v *D5Vault) persistAccount(addr types.Address, acc *account.StateAccount) error {
	if v.inMem || v.db == nil || acc == nil {
		return nil
	}
	v.dbMu.Lock()
	defer v.dbMu.Unlock()
	return putAccountPayload(v.db, addr.Bytes(), acc.Bytes())
}

func (v *D5Vault) markDirty(addr types.Address) {
	if v.inMem {
		return
	}
	v.dirtyMu.Lock()
	if v.dirty == nil {
		v.dirty = make(map[types.Address]struct{})
	}
	v.dirty[addr] = struct{}{}
	v.dirtyMu.Unlock()
}

// Flush writes all dirty accounts to disk.
func (v *D5Vault) Flush() error {
	if v.inMem || v.db == nil {
		return nil
	}
	v.dirtyMu.Lock()
	addrs := make([]types.Address, 0, len(v.dirty))
	for addr := range v.dirty {
		addrs = append(addrs, addr)
	}
	v.dirty = make(map[types.Address]struct{})
	v.dirtyMu.Unlock()

	for _, addr := range addrs {
		acc := v.accounts.GetAccount(addr)
		if acc == nil {
			continue
		}
		if err := v.persistAccount(addr, acc); err != nil {
			return fmt.Errorf("flush account %s: %w", addr.Hex(), err)
		}
	}
	return nil
}

// InitSecureVault seeds the vault database with a root account (tests / tooling).
func (v *D5Vault) InitSecureVault(rootSa *account.StateAccount) error {
	if v.db == nil {
		return fmt.Errorf("database not initialized")
	}
	key := rootSa.Address.Bytes()
	has, err := v.db.Has(key)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %w", err)
	}
	if has {
		return fmt.Errorf("vault already exists: %s", v.path)
	}
	if err := putAccountPayload(v.db, key, rootSa.Bytes()); err != nil {
		return fmt.Errorf("failed to write root account: %w", err)
	}
	return nil
}

// SaveAccountBytes persists a serialized account row.
func (v *D5Vault) SaveAccountBytes(accountBytes []byte) error {
	accountData := types.BytesToStateAccount(accountBytes)
	if accountData == nil {
		return fmt.Errorf("failed to decode account data")
	}
	if v.db == nil {
		return fmt.Errorf("database not initialized")
	}
	v.dbMu.Lock()
	defer v.dbMu.Unlock()
	return putAccountPayload(v.db, accountData.Address.Bytes(), accountBytes)
}

// VaultSourceSize counts account rows in the pogreb database.
func (v *D5Vault) VaultSourceSize() (int64, error) {
	if v.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	count := int64(0)
	it := v.db.Items()
	for {
		k, _, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to iterate database: %w", err)
		}
		if isContractOrStorageKey(k) {
			continue
		}
		count++
	}
	return count, nil
}
