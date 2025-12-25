package block

import (
	"fmt"
	"math/big"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

// CalculateBlockHash вычисляет хэш блока и возвращает его в виде байтов
func CalculateBlockHash(b *Block) ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("block is nil")
	}
	return b.CalculateHash()
}

// VerifyBlockHash проверяет, соответствует ли хэш блока требованиям difficulty
// Возвращает true, если хэш валидный (меньше target), false если невалидный
func VerifyBlockHash(b *Block) (bool, error) {
	if b == nil || b.Head == nil {
		return false, fmt.Errorf("block or header is nil")
	}

	// Проверка difficulty
	if b.Head.Difficulty == 0 {
		return false, fmt.Errorf("difficulty cannot be zero")
	}

	// Вычисляем target: target = 2^256 / difficulty
	target := new(big.Int).Div(
		new(big.Int).Lsh(big.NewInt(1), 256),
		big.NewInt(int64(b.Head.Difficulty)),
	)

	// Вычисляем хэш блока
	blockHash, err := b.CalculateHash()
	if err != nil {
		return false, fmt.Errorf("failed to calculate block hash: %w", err)
	}

	// Преобразуем хэш в big.Int для сравнения
	blockHashInt := new(big.Int).SetBytes(blockHash)

	// Проверяем: хэш валидный, если blockHashInt < target
	isValid := blockHashInt.Cmp(target) < 0

	return isValid, nil
}

// VerifyBlockHashWithDetails проверяет хэш блока и возвращает детальную информацию
func VerifyBlockHashWithDetails(b *Block) (*HashVerificationResult, error) {
	if b == nil || b.Head == nil {
		return nil, fmt.Errorf("block or header is nil")
	}

	result := &HashVerificationResult{
		BlockHeight: b.Head.Height,
		Difficulty:  b.Head.Difficulty,
		Nonce:       b.Head.Nonce,
	}

	// Проверка difficulty
	if b.Head.Difficulty == 0 {
		result.IsValid = false
		result.Error = "difficulty cannot be zero"
		return result, nil
	}

	// Вычисляем target: target = 2^256 / difficulty
	target := new(big.Int).Div(
		new(big.Int).Lsh(big.NewInt(1), 256),
		big.NewInt(int64(b.Head.Difficulty)),
	)
	result.Target = target

	// Вычисляем хэш блока
	blockHash, err := b.CalculateHash()
	if err != nil {
		result.IsValid = false
		result.Error = fmt.Sprintf("failed to calculate block hash: %v", err)
		return result, nil
	}

	result.BlockHash = common.BytesToHash(blockHash)
	result.BlockHashBytes = blockHash

	// Преобразуем хэш в big.Int для сравнения
	blockHashInt := new(big.Int).SetBytes(blockHash)
	result.BlockHashInt = blockHashInt

	// Сравнение
	comparison := blockHashInt.Cmp(target)
	result.Comparison = comparison

	// Проверяем: хэш валидный, если blockHashInt < target
	result.IsValid = comparison < 0

	// Вычисляем разницу для информации
	if comparison < 0 {
		// Хэш меньше target (валидный)
		result.Difference = new(big.Int).Sub(target, blockHashInt)
	} else {
		// Хэш больше или равен target (невалидный)
		result.Difference = new(big.Int).Sub(blockHashInt, target)
	}

	return result, nil
}

// HashVerificationResult содержит результаты проверки хэша блока
type HashVerificationResult struct {
	BlockHeight    int
	Difficulty     uint64
	Nonce          uint64
	Target         *big.Int
	BlockHash      common.Hash
	BlockHashBytes []byte
	BlockHashInt   *big.Int
	Comparison     int // -1 если hash < target, 0 если ==, 1 если >
	IsValid        bool
	Difference     *big.Int // Разница между hash и target
	Error          string
}

// String возвращает строковое представление результата проверки
func (r *HashVerificationResult) String() string {
	if r.Error != "" {
		return fmt.Sprintf("Error: %s", r.Error)
	}

	status := "INVALID"
	if r.IsValid {
		status = "VALID"
	}

	comparisonStr := ""
	switch r.Comparison {
	case -1:
		comparisonStr = "hash < target"
	case 0:
		comparisonStr = "hash == target"
	case 1:
		comparisonStr = "hash > target"
	}

	return fmt.Sprintf(
		"Block Hash Verification:\n"+
			"  Status: %s\n"+
			"  Height: %d\n"+
			"  Difficulty: %d\n"+
			"  Nonce: %d\n"+
			"  Target: %s\n"+
			"  Block Hash: %x\n"+
			"  Block Hash (int): %s\n"+
			"  Comparison: %s\n"+
			"  Difference: %s\n",
		status,
		r.BlockHeight,
		r.Difficulty,
		r.Nonce,
		r.Target.String(),
		r.BlockHashBytes,
		r.BlockHashInt.String(),
		comparisonStr,
		r.Difference.String(),
	)
}

// PrintVerificationResult выводит результат проверки в консоль
func PrintVerificationResult(b *Block) {
	result, err := VerifyBlockHashWithDetails(b)
	if err != nil {
		fmt.Printf("Error verifying block hash: %v\n", err)
		return
	}
	fmt.Println(result.String())
}

// FindValidNonce ищет валидный nonce для блока, перебирая значения до тех пор, пока не найдет валидный хеш
// Возвращает найденный nonce и количество попыток
func FindValidNonce(b *Block, startNonce uint64, maxAttempts uint64) (uint64, uint64, error) {
	if b == nil || b.Head == nil {
		return 0, 0, fmt.Errorf("block or header is nil")
	}

	// Проверка difficulty
	if b.Head.Difficulty == 0 {
		return 0, 0, fmt.Errorf("difficulty cannot be zero")
	}

	// Вычисляем target: target = 2^256 / difficulty
	target := new(big.Int).Div(
		new(big.Int).Lsh(big.NewInt(1), 256),
		big.NewInt(int64(b.Head.Difficulty)),
	)

	// Сохраняем оригинальный nonce
	originalNonce := b.Head.Nonce
	currentNonce := startNonce
	attempts := uint64(0)

	// Перебираем nonce
	for attempts < maxAttempts {
		// Устанавливаем текущий nonce
		b.Head.Nonce = currentNonce

		// Вычисляем хэш
		blockHash, err := b.CalculateHash()
		if err != nil {
			return 0, attempts, fmt.Errorf("failed to calculate block hash: %w", err)
		}

		// Преобразуем хэш в big.Int для сравнения
		blockHashInt := new(big.Int).SetBytes(blockHash)

		// Проверяем: хэш валидный, если blockHashInt < target
		if blockHashInt.Cmp(target) < 0 {
			// Найден валидный nonce!
			return currentNonce, attempts + 1, nil
		}

		// Продолжаем поиск
		currentNonce++
		attempts++
	}

	// Восстанавливаем оригинальный nonce
	b.Head.Nonce = originalNonce

	return 0, attempts, fmt.Errorf("valid nonce not found after %d attempts", maxAttempts)
}

// FindValidNonceForGenesis ищет валидный nonce для genesis блока
// Это удобная функция специально для genesis блоков
func FindValidNonceForGenesis(genesisHeader *Header, maxAttempts uint64) (uint64, uint64, error) {
	if genesisHeader == nil {
		return 0, 0, fmt.Errorf("genesis header is nil")
	}

	// Создаем временный блок для проверки
	testBlock := NewBlock(genesisHeader)
	testBlock.Transactions = []types.GTransaction{}

	// Рассчитываем размер блока
	blockBytes := testBlock.ToBytes()
	if blockBytes != nil {
		testBlock.Head.Size = len(blockBytes)
	}

	// Ищем валидный nonce, начиная с текущего значения
	return FindValidNonce(testBlock, genesisHeader.Nonce, maxAttempts)
}
