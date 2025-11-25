package chain

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/logger"
)

var chainSrcLogger = logger.Named("chain_source")

func InitChainVault(initBLock *block.Block) {
	InitChainVaultWithPath(initBLock, "./chain.dat")
}

func InitChainVaultWithPath(initBLock *block.Block, chainPath string) {
	// Open file for writing, create if it doesn't exist
	f, err := os.OpenFile(chainPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	buf, errjson := json.Marshal(initBLock)
	if errjson != nil {
		panic(errjson)
	}
	buf = append(buf, '\n') // Добавляем разделитель новой строки
	if _, errWrite := f.Write(buf); errWrite != nil {
		panic(errWrite)
	}
}

// load from file
func SyncVault() ([]*block.Block, error) {
	return SyncVaultWithPath("./chain.dat")
}

func SyncVaultWithPath(chainPath string) ([]*block.Block, error) {
	file, err := os.OpenFile(chainPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the vault file: %w", err)
	}
	defer file.Close()
	var readBlocks = make([]*block.Block, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip empty lines
		if len(line) == 0 {
			continue
		}
		bl := &block.Block{}
		// parse error fix - skip corrupted blocks instead of panicking
		err := json.Unmarshal(line, bl)
		if err != nil {
			// Log error but continue processing other blocks
			chainSrcLogger.Warnw("Skipping corrupted block", "err", err, "data", string(line))
			continue
		}
		readBlocks = append(readBlocks, bl)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read block data from file: %w", err)
	}
	chainSrcLogger.Infow("Loaded blocks from chain file", "count", len(readBlocks), "path", chainPath)

	return readBlocks, nil
}

func SaveToVault(newBlock block.Block) error {

	// Only write to file if batch size is reached
	// if totalSize < BATCH_SIZE {
	// 	return nil
	// }

	f, err := os.OpenFile("./chain.dat", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 0)
	blockData, err := json.Marshal(newBlock)
	if err != nil {
		return err
	}
	buf = append(buf, blockData...)
	buf = append(buf, '\n') // Добавляем разделитель новой строки

	if _, err := f.Write(buf); err != nil {
		return err
	}
	return nil
}

// UpdateVault updates a block in the vault file.
// Note: This function works with blocks, not accounts.
func UpdateVault(blockData []byte) error {
	filePath := "./chain.dat"

	// Read all blocks from the file
	readFile, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file: %w", err)
	}

	var blocks = make([]*block.Block, 0)
	scanner := bufio.NewScanner(readFile)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip empty lines
		if len(line) == 0 {
			continue
		}
		bl := &block.Block{}
		err := json.Unmarshal(line, bl)
		if err != nil {
			// Skip corrupted blocks
			continue
		}
		blocks = append(blocks, bl)
	}

	readFile.Close()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read block data from file: %w", err)
	}

	// Parse the updated block
	updatedBlock := &block.Block{}
	err = json.Unmarshal(blockData, updatedBlock)
	if err != nil {
		return fmt.Errorf("failed to decode block data for update: %w", err)
	}

	// Update the specific block by hash
	found := false
	for i, bl := range blocks {
		if bl != nil && bl.GetHash().Compare(updatedBlock.GetHash()) == 0 {
			blocks[i] = updatedBlock
			found = true
			break
		}
	}

	// If block not found, append it
	if !found {
		blocks = append(blocks, updatedBlock)
	}

	// Write all blocks back to the file
	writeFile, err := os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open the vault file for writing: %w", err)
	}
	defer writeFile.Close()

	writer := bufio.NewWriter(writeFile)
	for _, bl := range blocks {
		if bl == nil {
			continue
		}
		blockData, err := json.Marshal(bl)
		if err != nil {
			return fmt.Errorf("failed to marshal block: %w", err)
		}
		blockData = append(blockData, '\n')
		if _, err := writer.Write(blockData); err != nil {
			return fmt.Errorf("failed to write to the vault file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

func GetChainSourceSize() (int64, error) {
	filePath := "./chain.dat"
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

func ClearVault() error {
	if err := os.Truncate("./chain.dat", 0); err != nil {
		fmt.Printf("Failed to truncate: %v", err)
		return err
	}
	return nil
}
