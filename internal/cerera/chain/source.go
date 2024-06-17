package chain

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

func InitChainVault(initBLock block.Block) {
	// Open file for writing, create if it doesn't exist
	f, err := os.OpenFile("./chain.dat", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Errorf("failed to open the file for writing: %w", err)
	}
	defer f.Close()

	buf, err := json.Marshal(initBLock)
	if err != nil {
		panic(err)
	}
	buf = append(buf, '\n') // Добавляем разделитель новой строки
	if _, err := f.Write(buf); err != nil {
		fmt.Errorf("failed to write account data to file: %w", err)
	}
}

// load from file
func SyncVault() ([]block.Block, error) {
	file, err := os.OpenFile("./chain.dat", os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()
	var readBlocks = make([]block.Block, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		bl := &block.Block{}
		// parse error fix
		err := json.Unmarshal(line, bl)
		if err != nil {
			panic(err)
		}
		readBlocks = append(readBlocks, *bl)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read account data from file: %w", err)
	}

	return readBlocks, nil
}

func SaveToVault(newBlock block.Block) {
	f, err := os.OpenFile("./chain.dat", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Errorf("failed to open the vault file for writing: %w", err)
	}
	defer f.Close()

	buf, err := json.Marshal(newBlock)
	if err != nil {
		panic(err)
	}
	buf = append(buf, '\n') // Добавляем разделитель новой строки
	if _, err := f.Write(buf); err != nil {
		fmt.Errorf("failed to write account data to file: %w", err)
	}
}

// UpdateVault updates an account in the vault file.
func UpdateVault(account []byte) error {
	filePath := "./chain.dat"

	// Read all accounts from the file
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()

	var accounts = make([]types.StateAccount, 0)
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
