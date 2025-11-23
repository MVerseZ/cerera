package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cerera/internal/cerera/types"
	"go.mills.io/bitcask/v2"
)

var vaultSrcLogger = log.New(os.Stdout, "[vault_source] ", log.LstdFlags|log.Lmicroseconds)
var IS_DEGUG = true

// getBitcaskDB returns the vault's database instance, opening it if needed
func getBitcaskDB(vaultPath string) (*bitcask.Bitcask, error) {
	vault := GetVault()

	// If database is already open, return it
	vault.dbMu.RLock()
	if vault.db != nil {
		db := vault.db
		vault.dbMu.RUnlock()
		return db, nil
	}
	vault.dbMu.RUnlock()

	// Database not open, try to open it
	vault.dbMu.Lock()
	defer vault.dbMu.Unlock()

	// Double check after acquiring write lock
	if vault.db != nil {
		return vault.db, nil
	}

	// Extract directory from path (bitcask needs a directory, not a file)
	dbDir := vaultPath
	// Ensure directory exists
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}

	// Open bitcask database
	db, err := bitcask.Open(dbDir, bitcask.WithSyncWrites(false))
	if err != nil {
		return nil, fmt.Errorf("failed to open bitcask database: %w", err)
	}

	// Store in vault
	vault.db = db

	return db, nil
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

func InitSecureVault(rootSa *types.StateAccount, vaultPath string) error {
	// Get bitcask database from vault
	db, err := getBitcaskDB(vaultPath)
	if err != nil {
		return err
	}

	// Use address bytes as key (shorter than hex string)
	key := rootSa.Address.Bytes()
	if db.Has(key) {
		return fmt.Errorf("vault already exists: %s", vaultPath)
	}

	// Save account data
	accountData := rootSa.Bytes()
	if IS_DEGUG {
		vaultSrcLogger.Printf("Writing account data to bitcask: %s", rootSa.Address.Hex())
	}

	if err := db.Put(key, accountData); err != nil {
		return fmt.Errorf("failed to write account data to bitcask: %w", err)
	}

	return nil
}

// load from bitcask database
func SyncVault(path string) error {
	// Get bitcask database from vault
	db, err := getBitcaskDB(path)
	if err != nil {
		return err
	}

	// Clear existing accounts in memory
	GetVault().Clear()

	// Iterate over all keys in the database
	err = db.Scan([]byte{}, func(key bitcask.Key) error {
		// Get account data
		accountData, err := db.Get(key)
		if err != nil {
			if IS_DEGUG {
				vaultSrcLogger.Printf("SyncVault: failed to get account data for key %x: %v", key, err)
			}
			return nil // Continue with next key
		}

		// Try to deserialize account, skip on error
		func() {
			defer func() {
				if r := recover(); r != nil {
					vltlogger.Printf("Skipping corrupted account data in vault: %v (key: %x, data length: %d)", r, key, len(accountData))
				}
			}()
			account := types.BytesToStateAccount(accountData)
			if account != nil {
				if IS_DEGUG {
					vltlogger.Printf("Read account from bitcask vault: %s", account.Address.Hex())
				}
				GetVault().accounts.Append(account.Address, account)
			} else {
				previewLen := 20
				if len(accountData) < previewLen {
					previewLen = len(accountData)
				}
				if IS_DEGUG {
					vltlogger.Printf("Failed to deserialize account from vault (key: %x, data length: %d, first bytes: %x)", key, len(accountData), accountData[:previewLen])
				}
			}
		}()

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan bitcask database: %w", err)
	}

	return nil
}

func SaveToVault(account []byte, vaultPath string) error {
	// Decode account from bytes using BytesToStateAccount
	accountData := types.BytesToStateAccount(account)
	if accountData == nil {
		return fmt.Errorf("failed to decode account data")
	}

	// Get bitcask database from vault
	db, err := getBitcaskDB(vaultPath)
	if err != nil {
		return err
	}

	// Use address bytes as key (shorter than hex string)
	key := accountData.Address.Bytes()

	if IS_DEGUG {
		vaultSrcLogger.Printf("Writing account data to bitcask: %s", accountData.Address.Hex())
	}

	if err := db.Put(key, account); err != nil {
		return fmt.Errorf("failed to write account data to bitcask: %w", err)
	}

	return nil
}

// UpdateVault updates an account in the bitcask database.
func UpdateVault(account []byte, vaultPath string) error {
	// Decode account from bytes
	updatedAccount := types.BytesToStateAccount(account)
	if updatedAccount == nil {
		return fmt.Errorf("failed to decode account data for update")
	}

	// Get bitcask database from vault
	db, err := getBitcaskDB(vaultPath)
	if err != nil {
		return err
	}

	// Use address bytes as key (shorter than hex string)
	key := updatedAccount.Address.Bytes()

	if IS_DEGUG {
		vaultSrcLogger.Printf("UpdateVault: updating account %s in bitcask", updatedAccount.Address.Hex())
	}

	// Put will overwrite if key exists, or create if it doesn't
	if err := db.Put(key, account); err != nil {
		return fmt.Errorf("failed to update account in bitcask: %w", err)
	}

	if IS_DEGUG {
		vaultSrcLogger.Printf("UpdateVault: successfully updated account %s", updatedAccount.Address.Hex())
	}

	return nil
}

func VaultSourceSize(vaultPath string) (int64, error) {
	// Get bitcask database from vault
	db, err := getBitcaskDB(vaultPath)
	if err != nil {
		return 0, err
	}

	// Get database stats
	stats, err := db.Stats()
	if err != nil {
		return 0, fmt.Errorf("failed to get bitcask stats: %w", err)
	}
	return int64(stats.Size), nil
}
