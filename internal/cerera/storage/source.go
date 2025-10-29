package storage

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cerera/internal/cerera/types"
)

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
	// Open file for writing, create if it doesn't exist
	f, err := os.OpenFile(vaultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the file for writing: %w", err)
	}
	defer f.Close()

	accountData := rootSa.Bytes()
	accountData = append(accountData, '\n') // Добавляем разделитель новой строки
	if _, err := f.Write(accountData); err != nil {
		return fmt.Errorf("failed to write account data to file: %w", err)
	}
	return nil
}

// load from file
func SyncVault(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	GetVault().Clear()
	for scanner.Scan() {
		line := scanner.Bytes()
		account := types.BytesToStateAccount(line)
		GetVault().accounts.Append(account.Address, account)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read account data from file: %w", err)
	}

	return nil
}

func SaveToVault(account []byte, vaultPath string) error {
	f, err := os.OpenFile(vaultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Decode account from bytes using BytesToStateAccount
	accountData := types.BytesToStateAccount(account)
	accountData.Status = "LOCAL"
	accountDataToWrite := accountData.Bytes()
	accountDataToWrite = append(accountDataToWrite, '\n')

	if _, err := f.Write(accountDataToWrite); err != nil {
		return err
	}

	return nil
}

// UpdateVault updates an account in the vault file.
func UpdateVault(account []byte, vaultPath string) error {
	filePath := vaultPath

	// Read all accounts from the file
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()

	var accounts = make([]*types.StateAccount, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		account := types.BytesToStateAccount(line)
		accounts = append(accounts, account)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read account data from file: %w", err)
	}

	// Update the specific account
	updatedAccount := types.BytesToStateAccount(account)
	for i, acc := range accounts {
		if acc.Address == updatedAccount.Address {
			accounts[i] = updatedAccount
			break
		}
	}

	// Write all accounts back to the file
	file, err = os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file for writing: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, acc := range accounts {
		accountData := acc.Bytes()
		accountData = append(accountData, '\n')
		if _, err := writer.Write(accountData); err != nil {
			return fmt.Errorf("failed to write to the vault file: %w", err)
		}
	}
	writer.Flush()

	return nil
}

func VaultSourceSize(vaultPath string) (int64, error) {
	filePath := vaultPath
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	fi, err2 := f.Stat()
	if err2 != nil {
		return 0, err2
	}

	return fi.Size(), nil
}
