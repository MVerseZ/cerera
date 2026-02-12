package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

func main() {
	// Парсинг аргументов командной строки
	chainId := flag.Int("chainid", 11, "Chain ID for genesis block")
	difficulty := flag.Uint64("difficulty", 0, "Difficulty value (0 = use genesis default)")
	nonce := flag.Uint64("nonce", 0, "Nonce value (0 = use genesis default)")
	findValid := flag.Bool("find", false, "Find valid nonce for genesis block")
	maxAttempts := flag.Uint64("max", 10000000000, "Maximum attempts when searching for valid nonce")
	flag.Parse()

	fmt.Printf("=== Block Hash Calculator and Verifier v.%x===\r\n", time.Now().Unix())

	// Создаем genesis блок для тестирования
	genesisHeader := block.GenesisHead(*chainId)

	// Если указаны difficulty или nonce, используем их
	if *difficulty > 0 {
		genesisHeader.Difficulty = *difficulty
		fmt.Printf("Using custom difficulty: %d\n", *difficulty)
	} else {
		fmt.Printf("Using genesis difficulty: %d\n", genesisHeader.Difficulty)
	}

	if *nonce > 0 {
		genesisHeader.Nonce = *nonce
		fmt.Printf("Using custom nonce: %d\n", *nonce)
	} else {
		fmt.Printf("Using genesis nonce: %d\n", genesisHeader.Nonce)
	}

	// Создаем блок
	testBlock := block.NewBlock(genesisHeader)
	testBlock.Transactions = []types.GTransaction{}

	// ВАЖНО: Рассчитываем размер блока после установки nonce, так как он влияет на размер JSON
	blockBytes := testBlock.ToBytes()
	if blockBytes != nil {
		testBlock.Head.Size = len(blockBytes)
	}

	fmt.Printf("Block Height: %d\n", testBlock.Head.Height)
	fmt.Printf("Block Chain ID: %d\n", testBlock.Head.ChainId)
	fmt.Println()

	// Вычисляем хэш
	hashBytes, err := block.CalculateBlockHash(testBlock)
	if err != nil {
		fmt.Printf("Error calculating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Block Hash (hex): %x\n", hashBytes)
	fmt.Printf("Block Hash (bytes length): %d\n", len(hashBytes))
	fmt.Println()

	// Проверяем хэш
	isValid, err := block.VerifyBlockHash(testBlock)
	if err != nil {
		fmt.Printf("Error verifying hash: %v\n", err)
		os.Exit(1)
	}

	if isValid {
		fmt.Println("✓ Block hash is VALID (meets difficulty requirement)")
	} else {
		fmt.Println("✗ Block hash is INVALID (does not meet difficulty requirement)")
	}
	fmt.Println()

	// Детальная проверка
	fmt.Println("=== Detailed Verification ===")
	result, err := block.VerifyBlockHashWithDetails(testBlock)
	if err != nil {
		fmt.Printf("Error getting detailed verification: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.String())

	// Дополнительная информация
	fmt.Println("\n=== Additional Information ===")
	fmt.Printf("Probability of valid hash: ~%.2e\n", calculateProbability(result.Difficulty))
	fmt.Printf("Probability of valid hash, percentage: ~%.2f%%\n", calculateProbability(result.Difficulty)*100)
	fmt.Printf("Expected attempts to find valid hash: ~%.0f\n", calculateExpectedAttempts(result.Difficulty))

	// Если нужно найти валидный nonce
	if *findValid {
		fmt.Println("\n=== Searching for Valid Nonce ===")
		fmt.Printf("Starting search from nonce: %d\n", genesisHeader.Nonce)
		fmt.Printf("Maximum attempts: %d\n", *maxAttempts)
		fmt.Println("Searching...")

		validNonce, attempts, err := block.FindValidNonceForGenesis(genesisHeader, *maxAttempts)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✓ Valid nonce found!\n")
		fmt.Printf("  Valid nonce: %d\n", validNonce)
		fmt.Printf("  Attempts: %d\n", attempts)
		fmt.Printf("\nUpdate genesis.go with:\n")
		fmt.Printf("  Nonce: %d\n", validNonce)

		// Проверяем найденный nonce
		genesisHeader.Nonce = validNonce
		testBlock.Head.Nonce = validNonce
		blockBytes = testBlock.ToBytes()
		if blockBytes != nil {
			testBlock.Head.Size = len(blockBytes)
		}

		isValid, err := block.VerifyBlockHash(testBlock)
		if err == nil && isValid {
			fmt.Println("\n✓ Verification: Block hash is now VALID")
		} else {
			fmt.Println("\n✗ Verification: Block hash is still INVALID (this should not happen!)")
		}
	}
}

// calculateProbability вычисляет приблизительную вероятность валидного хеша
func calculateProbability(difficulty uint64) float64 {
	if difficulty == 0 {
		return 0
	}
	// Приблизительно: вероятность = difficulty / 2^256
	// Но для больших чисел это очень маленькое значение
	return float64(difficulty) / (1 << 32) // Упрощенная формула для демонстрации
}

// calculateExpectedAttempts вычисляет ожидаемое количество попыток для нахождения валидного хеша
func calculateExpectedAttempts(difficulty uint64) float64 {
	if difficulty == 0 {
		return 0
	}
	// Ожидаемое количество попыток ≈ 2^256 / difficulty
	// Упрощенная формула для демонстрации
	return float64(1<<32) / float64(difficulty)
}
