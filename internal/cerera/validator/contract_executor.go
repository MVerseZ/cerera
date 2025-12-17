package validator

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/pallada/pallada"
)

var (
	ErrContractNotFound  = errors.New("contract code not found")
	ErrContractExecution = errors.New("contract execution failed")
	ErrOutOfGas          = errors.New("out of gas")
)

// ExecuteContract выполняет контракт с использованием VM
// Возвращает результат выполнения и использованный газ
func ExecuteContract(tx types.GTransaction, contractAddress types.Address, localVault storage.Vault, chainInstance *chain.Chain) ([]byte, uint64, error) {
	// Загружаем код контракта
	contractCode, err := localVault.GetContractCode(contractAddress)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrContractNotFound, err)
	}

	if len(contractCode) == 0 {
		return nil, 0, ErrContractNotFound
	}

	// Получаем информацию о текущем блоке
	blockInfo := getBlockInfo(chainInstance)

	// Создаем Storage адаптер для Vault
	storageAdapter := &VaultStorageAdapter{vault: localVault}

	// Создаем Context для VM с storage
	ctx := pallada.NewContextWithStorage(
		tx.From(),        // Caller
		contractAddress,  // Address (адрес контракта)
		tx.Value(),       // Value
		tx.Data(),        // Input (данные транзакции)
		uint64(tx.Gas()), // GasLimit
		tx.GasPrice(),    // GasPrice
		blockInfo,        // BlockInfo
		storageAdapter,   // Storage
	)

	// Создаем VM с контекстом
	vm := pallada.NewVMWithContext(contractCode, ctx)

	// Выполняем контракт
	result, err := vm.Run()

	// Получаем использованный газ
	gasUsed := uint64(0)
	if vm.GetGasMeter() != nil {
		gasUsed = vm.GasUsed()
	}

	if err != nil {
		// Проверяем, это REVERT или другая ошибка
		if vm.GetGasMeter() != nil && vm.GetGasMeter().GasRemaining() == 0 {
			return nil, gasUsed, fmt.Errorf("%w: %v", ErrOutOfGas, err)
		}
		// REVERT возвращает данные, но устанавливает ошибку
		if result != nil {
			// Это REVERT - возвращаем данные и ошибку
			return result, gasUsed, fmt.Errorf("revert: %x", result)
		}
		return nil, gasUsed, fmt.Errorf("%w: %v", ErrContractExecution, err)
	}

	vlogger.Debugw("Contract executed successfully",
		"contract", contractAddress.Hex(),
		"gasUsed", gasUsed,
		"resultSize", len(result),
	)

	return result, gasUsed, nil
}

// getBlockInfo получает информацию о текущем блоке для Context
func getBlockInfo(chainInstance *chain.Chain) *pallada.BlockInfo {
	if chainInstance == nil {
		// Если chain недоступен, возвращаем пустую информацию
		return &pallada.BlockInfo{
			Number:    0,
			Timestamp: 0,
			Hash:      make([]byte, 32),
		}
	}

	// Получаем последний блок
	latestBlock := chainInstance.GetLatestBlock()
	if latestBlock == nil || latestBlock.Header() == nil {
		return &pallada.BlockInfo{
			Number:    0,
			Timestamp: 0,
			Hash:      make([]byte, 32),
		}
	}

	header := latestBlock.Header()
	blockHash := latestBlock.GetHash()

	return &pallada.BlockInfo{
		Number:    uint64(header.Index),
		Timestamp: header.Timestamp,
		Hash:      blockHash.Bytes(),
	}
}

// GenerateContractAddress генерирует адрес контракта на основе адреса отправителя и nonce
// Адрес = blake2b(sender + nonce)[:32] (последние 32 байта хеша)
func GenerateContractAddress(sender types.Address, nonce uint64) types.Address {
	// Используем INRISeqHash (blake2b) для генерации адреса
	senderBytes := sender.Bytes()
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)

	// Хешируем sender + nonce
	hash := types.INRISeqHash(senderBytes, nonceBytes)

	// Берем последние 32 байта хеша как адрес контракта
	return types.BytesToAddress(hash.Bytes())
}

// ExecuteContractCreation обрабатывает создание контракта из транзакции
// Возвращает адрес созданного контракта, использованный газ и ошибку
func ExecuteContractCreation(tx types.GTransaction, localVault storage.Vault, chainInstance *chain.Chain) (types.Address, uint64, error) {
	if tx.To() != nil {
		return types.Address{}, 0, errors.New("contract creation requires To == nil")
	}

	if len(tx.Data()) == 0 {
		return types.Address{}, 0, errors.New("contract creation requires non-empty data")
	}

	// Получаем аккаунт отправителя для nonce
	senderAcc := localVault.Get(tx.From())
	if senderAcc == nil {
		return types.Address{}, 0, errors.New("sender account not found")
	}

	// Генерируем адрес контракта на основе sender + nonce
	contractAddress := GenerateContractAddress(tx.From(), senderAcc.Nonce)

	// Проверяем, что контракт с таким адресом еще не существует
	if localVault.HasContractCode(contractAddress) {
		return types.Address{}, 0, fmt.Errorf("contract already exists at address %s", contractAddress.Hex())
	}

	// Сохраняем код контракта
	contractCode := tx.Data()
	if err := localVault.StoreContractCode(contractAddress, contractCode); err != nil {
		return types.Address{}, 0, fmt.Errorf("failed to store contract code: %w", err)
	}

	// Получаем информацию о текущем блоке
	blockInfo := getBlockInfo(chainInstance)

	// Создаем Storage адаптер для Vault
	storageAdapter := &VaultStorageAdapter{vault: localVault}

	// Создаем Context для VM (для выполнения конструктора) с storage
	ctx := pallada.NewContextWithStorage(
		tx.From(),        // Caller
		contractAddress,  // Address (адрес нового контракта)
		tx.Value(),       // Value
		nil,              // Input (для создания контракта input пустой, код в Data)
		uint64(tx.Gas()), // GasLimit
		tx.GasPrice(),    // GasPrice
		blockInfo,        // BlockInfo
		storageAdapter,   // Storage
	)

	// Создаем VM с контекстом и кодом контракта
	vm := pallada.NewVMWithContext(contractCode, ctx)

	// Выполняем конструктор контракта (если есть)
	result, err := vm.Run()

	// Получаем использованный газ
	gasUsed := uint64(0)
	if vm.GetGasMeter() != nil {
		gasUsed = vm.GasUsed()
	}

	// Если конструктор вернул данные (RETURN), это может быть init code
	// В EVM конструктор возвращает runtime code, который сохраняется
	// Пока просто сохраняем исходный код, в будущем можно обработать RETURN из конструктора
	if err != nil {
		// Проверяем, это REVERT или другая ошибка
		if vm.GetGasMeter() != nil && vm.GetGasMeter().GasRemaining() == 0 {
			// Откатываем сохранение кода при нехватке газа
			// TODO: Удалить код контракта при ошибке
			return types.Address{}, gasUsed, fmt.Errorf("%w: %v", ErrOutOfGas, err)
		}
		// REVERT при создании контракта означает, что создание не удалось
		if result != nil {
			// TODO: Удалить код контракта при REVERT
			return types.Address{}, gasUsed, fmt.Errorf("contract creation reverted: %x", result)
		}
		// Другие ошибки - контракт создан, но конструктор упал
		// В EVM контракт все равно создается, даже если конструктор упал
		vlogger.Warnw("Contract created but constructor failed",
			"contract", contractAddress.Hex(),
			"error", err,
		)
	}

	// Если конструктор вернул данные через RETURN, это может быть runtime code
	// В EVM: constructor возвращает bytecode, который сохраняется как код контракта
	// Пока сохраняем исходный код, но можно обработать RETURN
	if result != nil && len(result) > 0 {
		// Обновляем код контракта результатом выполнения конструктора
		// Это соответствует EVM поведению: constructor возвращает runtime code
		if err := localVault.StoreContractCode(contractAddress, result); err != nil {
			return types.Address{}, gasUsed, fmt.Errorf("failed to update contract code with constructor result: %w", err)
		}
		vlogger.Debugw("Contract code updated with constructor result",
			"contract", contractAddress.Hex(),
			"originalSize", len(contractCode),
			"runtimeSize", len(result),
		)
	}

	vlogger.Debugw("Contract created successfully",
		"contract", contractAddress.Hex(),
		"gasUsed", gasUsed,
		"codeSize", len(contractCode),
	)

	return contractAddress, gasUsed, nil
}

// VaultStorageAdapter адаптирует Vault к StorageInterface для VM
type VaultStorageAdapter struct {
	vault storage.Vault
}

// GetStorage получает значение из storage контракта
func (v *VaultStorageAdapter) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	return v.vault.GetStorage(address, key)
}

// SetStorage сохраняет значение в storage контракта
func (v *VaultStorageAdapter) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	return v.vault.SetStorage(address, key, value)
}

// ExecuteContractCall обрабатывает вызов контракта из транзакции
// Возвращает результат выполнения и использованный газ
func ExecuteContractCall(tx types.GTransaction, localVault storage.Vault, chainInstance *chain.Chain) ([]byte, uint64, error) {
	if tx.To() == nil {
		return nil, 0, errors.New("contract call requires recipient address")
	}

	contractAddress := *tx.To()

	// Проверяем, что контракт существует
	if !localVault.HasContractCode(contractAddress) {
		return nil, 0, ErrContractNotFound
	}

	// Выполняем контракт
	result, gasUsed, err := ExecuteContract(tx, contractAddress, localVault, chainInstance)
	if err != nil {
		return nil, 0, err
	}

	return result, gasUsed, nil
}
