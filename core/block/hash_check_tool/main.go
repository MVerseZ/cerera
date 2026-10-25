package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func main() {
	chainID := flag.Int("chainid", 11, "Chain ID for genesis block")
	difficulty := flag.Uint64("difficulty", 0, "Difficulty override (0 = genesis default)")
	nonce := flag.Uint64("nonce", 0, "Nonce override (0 = genesis default)")
	findValid := flag.Bool("find", false, "Search for a valid PoW nonce at genesis difficulty")
	maxAttempts := flag.Uint64("max", 10_000_000_000, "Max nonce attempts when searching")
	flag.Parse()

	fmt.Println("=== Genesis / PoW Hash Tool ===")

	header := block.GenesisHead(*chainID)
	if *difficulty > 0 {
		header.Difficulty = *difficulty
	}
	if *nonce > 0 {
		header.Nonce = *nonce
	}

	chainBlock := block.NewBlockWithHeaderAndHash(header)
	fmt.Printf("Chain genesis hash (CrvBlockHash): %s\n", chainBlock.Hash.Hex())

	powBlock := preparePowBlock(header)
	powHash, err := block.CalculateBlockHash(powBlock)
	if err != nil {
		fmt.Printf("Error calculating PoW hash: %v\n", err)
		os.Exit(1)
	}
	powBlock.Hash = common.BytesToHash(powHash)

	fmt.Printf("PoW hash (CalculateHash):            0x%x\n", powHash)
	fmt.Printf("Difficulty:                          %d\n", header.Difficulty)
	fmt.Printf("Nonce:                               %d\n", header.Nonce)
	fmt.Printf("Size (header):                       %d\n", powBlock.Head.Size)
	fmt.Println()

	isValid, err := block.VerifyBlockHash(powBlock)
	if err != nil {
		fmt.Printf("PoW verify error: %v\n", err)
		os.Exit(1)
	}
	if isValid {
		fmt.Println("PoW status: VALID")
	} else {
		fmt.Println("PoW status: INVALID")
	}

	result, err := block.VerifyBlockHashWithDetails(powBlock)
	if err != nil {
		fmt.Printf("Detailed verify error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println(result.String())

	fmt.Println("\n=== PoW search stats ===")
	fmt.Printf("Approx. success probability: ~%.2e\n", powProbability(header.Difficulty))
	fmt.Printf("Expected attempts:           ~%.0f\n", expectedAttempts(header.Difficulty))

	if !*findValid {
		fmt.Println("\nTip: run with -find to search a valid nonce at current difficulty")
		return
	}

	searchHeader := block.GenesisHead(*chainID)
	if *difficulty > 0 {
		searchHeader.Difficulty = *difficulty
	}
	if *nonce > 0 {
		searchHeader.Nonce = *nonce
	}

	fmt.Println("\n=== Searching valid PoW nonce ===")
	fmt.Printf("Search difficulty: %d\n", searchHeader.Difficulty)
	fmt.Printf("Max attempts:      %d\n", *maxAttempts)

	validNonce, attempts, err := block.FindValidNonceForGenesis(searchHeader, *maxAttempts)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		fmt.Println("Try a lower -difficulty or higher -max")
		os.Exit(1)
	}

	searchHeader.Nonce = validNonce
	foundBlock := preparePowBlock(searchHeader)
	foundHash, _ := block.CalculateBlockHash(foundBlock)
	foundBlock.Hash = common.BytesToHash(foundHash)

	fmt.Printf("\nValid nonce found in %d attempts\n", attempts)
	fmt.Printf("  Nonce:       %d\n", validNonce)
	fmt.Printf("  Difficulty:  %d\n", searchHeader.Difficulty)
	fmt.Printf("  PoW hash:    0x%x\n", foundHash)
	fmt.Printf("  Size:        %d\n", foundBlock.Head.Size)

	ok, _ := block.VerifyBlockHash(foundBlock)
	if ok {
		fmt.Println("  Verify:      OK")
	} else {
		fmt.Println("  Verify:      FAILED (unexpected)")
	}

	fmt.Println("\nUpdate genesis.go with:")
	fmt.Printf("  Difficulty: uint64(%d), Nonce: %d, Size: %d\n",
		searchHeader.Difficulty, validNonce, foundBlock.Head.Size)
}

func preparePowBlock(header *block.Header) *block.Block {
	b := block.NewBlock(header)
	b.Transactions = []types.GTransaction{}
	if bytes := b.ToBytes(); bytes != nil {
		b.Head.Size = len(bytes)
	}
	return b
}

func powProbability(difficulty uint64) float64 {
	if difficulty == 0 {
		return 0
	}
	p := 1.0 / float64(difficulty)
	if p > 1 {
		return 1
	}
	return p
}

func expectedAttempts(difficulty uint64) float64 {
	if difficulty == 0 {
		return 0
	}
	return float64(difficulty)
}
