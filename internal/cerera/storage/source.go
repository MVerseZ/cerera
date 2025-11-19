package storage

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cerera/internal/cerera/types"
)

var vaultSrcLogger = log.New(os.Stdout, "[vault_source] ", log.LstdFlags|log.Lmicroseconds)

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
	// Check if file already exists to prevent overwriting
	if _, err := os.Stat(vaultPath); err == nil {
		return fmt.Errorf("vault file already exists: %s", vaultPath)
	}

	// Open file for writing, create if it doesn't exist
	f, err := os.OpenFile(vaultPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open the file for writing: %w", err)
	}
	defer f.Close()

	accountData := rootSa.Bytes()
	vaultSrcLogger.Printf("Writing account data to file: %s", rootSa.Address.Hex())
	accountData = append(accountData, []byte("CERERA_ACC_")...)
	if _, err := f.Write(accountData); err != nil {
		return fmt.Errorf("failed to write account data to file: %w", err)
	}
	return nil
}

// load from file
func SyncVault(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()

	// Support both legacy "CERERA_ACC_" delimiter and newline-separated entries.
	splitEntries := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		trimmed := func(b []byte) []byte {
			return bytes.TrimSpace(b)
		}
		delimiter := []byte("CERERA_ACC_")
		if i := bytes.Index(data, delimiter); i >= 0 {
			token = trimmed(data[:i])
			advance = i + len(delimiter)
			if len(token) > 0 {
				vaultSrcLogger.Printf("SyncVault: found account with CERERA_ACC_ delimiter (token length: %d)", len(token))
			}
			return advance, token, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			token = trimmed(data[:i])
			advance = i + 1
			if len(token) > 0 {
				vaultSrcLogger.Printf("SyncVault: found account with newline delimiter (token length: %d, advance: %d)", len(token), advance)
			}
			return advance, token, nil
		}
		if atEOF && len(data) > 0 {
			token = trimmed(data)
			advance = len(data)
			if len(token) > 0 {
				vaultSrcLogger.Printf("SyncVault: found account at EOF (token length: %d)", len(token))
			}
			return advance, token, nil
		}
		return 0, nil, nil
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large accounts (default is 64KB)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max
	scanner.Split(splitEntries)
	GetVault().Clear()
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip empty segments
		if len(line) == 0 {
			continue
		}

		// Try to deserialize account, skip on error
		func() {
			defer func() {
				if r := recover(); r != nil {
					vltlogger.Printf("Skipping corrupted account data in vault file: %v (data length: %d)", r, len(line))
				}
			}()
			account := types.BytesToStateAccount(line)
			if account != nil {
				vltlogger.Printf("Read account from file vault: %s", account.Address.Hex())
				GetVault().accounts.Append(account.Address, account)
			} else {
				previewLen := 20
				if len(line) < previewLen {
					previewLen = len(line)
				}
				vltlogger.Printf("Failed to deserialize account from vault file (data length: %d, first bytes: %x)", len(line), line[:previewLen])
			}
		}()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read account data from file: %w", err)
	}

	return nil
}

func SaveToVault(account []byte, vaultPath string) error {
	f, err := os.OpenFile(vaultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open vault file: %w", err)
	}
	defer f.Close()

	// Decode account from bytes using BytesToStateAccount
	accountData := types.BytesToStateAccount(account)
	if accountData == nil {
		return fmt.Errorf("failed to decode account data")
	}
	// Note: Status values are 0-4 (0: OP_ACC_NEW, 1: OP_ACC_STAKE, 2: OP_ACC_F, 3: OP_ACC_NODE, 4: VOID)
	// Status is preserved from the original account data
	// accountDataToWrite := accountData.Bytes()
	// accountDataToWrite = append(accountDataToWrite, '\n')

	vaultSrcLogger.Printf("Writing account data to file: %s", accountData.Address.Hex())
	account = append(account, []byte("CERERA_ACC_")...)
	if _, err := f.Write(account); err != nil {
		return fmt.Errorf("failed to write account data: %w", err)
	}
	return nil
}

// UpdateVault updates an account in the vault file.
func UpdateVault(account []byte, vaultPath string) error {
	filePath := vaultPath

	// Read all accounts from the file
	readFile, err := os.OpenFile(filePath, os.O_RDONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}

	var accounts = make([]*types.StateAccount, 0)
	splitCER := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		trimmed := func(b []byte) []byte {
			return bytes.TrimSpace(b)
		}
		delimiter := []byte("CERERA_ACC_")
		if i := bytes.Index(data, delimiter); i >= 0 {
			return i + len(delimiter), trimmed(data[:i]), nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, trimmed(data[:i]), nil
		}
		if atEOF && len(data) > 0 {
			return len(data), trimmed(data), nil
		}
		return 0, nil, nil
	}
	scanner := bufio.NewScanner(readFile)
	// Increase buffer size to handle large accounts (default is 64KB)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max
	scanner.Split(splitCER)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip empty segments
		if len(line) == 0 {
			continue
		}
		acc := types.BytesToStateAccount(line)
		if acc != nil {
			vaultSrcLogger.Printf("UpdateVault: read account %s from file", acc.Address.Hex())
			accounts = append(accounts, acc)
		} else {
			previewLen := 20
			if len(line) < previewLen {
				previewLen = len(line)
			}
			vaultSrcLogger.Printf("UpdateVault: failed to deserialize account (data length: %d, first bytes: %x)", len(line), line[:previewLen])
		}
	}
	vaultSrcLogger.Printf("UpdateVault: read %d accounts from file", len(accounts))

	readFile.Close()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read account data from file: %w", err)
	}

	// Update the specific account
	updatedAccount := types.BytesToStateAccount(account)
	if updatedAccount == nil {
		return fmt.Errorf("failed to decode account data for update")
	}

	found := false
	for i, acc := range accounts {
		if acc != nil && acc.Address == updatedAccount.Address {
			accounts[i] = updatedAccount
			found = true
			break
		}
	}

	// If account not found, append it
	if !found {
		accounts = append(accounts, updatedAccount)
	}

	// Write all accounts back to the file
	writeFile, err := os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open the vault file for writing: %w", err)
	}
	defer writeFile.Close()

	writer := bufio.NewWriter(writeFile)
	writtenCount := 0
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		accountData := acc.Bytes()
		accountData = append(accountData, []byte("CERERA_ACC_")...)
		if _, err := writer.Write(accountData); err != nil {
			return fmt.Errorf("failed to write to the vault file: %w", err)
		}
		writtenCount++
		vaultSrcLogger.Printf("UpdateVault: wrote account %s to file", acc.Address.Hex())
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	vaultSrcLogger.Printf("UpdateVault: wrote %d accounts to file", writtenCount)

	return nil
}

func VaultSourceSize(vaultPath string) (int64, error) {
	filePath := vaultPath
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fi, err2 := f.Stat()
	if err2 != nil {
		return 0, err2
	}

	return fi.Size(), nil
}
