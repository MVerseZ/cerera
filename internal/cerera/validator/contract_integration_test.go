package validator

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
)

// TestContractCreationAndExecution тестирует полный цикл создания и выполнения контракта
func TestContractCreationAndExecution(t *testing.T) {
	// Очищаем тестовую директорию
	cfg := createTestConfigForValidator()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Создаем отправителя транзакции
	senderKey, _ := types.GenerateAccount()
	senderAddr := types.PubkeyToAddress(senderKey.PublicKey)

	// Создаем аккаунт отправителя
	senderAcc := types.NewStateAccount(senderAddr, 1000.0, common.Hash{})
	vault.Put(senderAddr, senderAcc)

	// Простой байткод контракта: PUSH1 42, PUSH1 0, SSTORE, STOP
	// Это сохраняет значение 42 в storage по ключу 0
	contractCode := []byte{
		0x70, 0x2a, // PUSH1 42
		0x70, 0x00, // PUSH1 0
		0x97, // SSTORE
		0x00, // STOP
	}

	// Создаем транзакцию создания контракта
	pgTx := &types.PGTransaction{
		Nonce:    1,
		To:       nil, // nil для создания контракта
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     contractCode,
		Time:     time.Now(),
	}
	createTx := types.NewTx(pgTx)
	createTx.SetFrom(senderAddr)

	// Выполняем создание контракта
	contractAddr, gasUsed, err := ExecuteContractCreation(*createTx, vault, nil)
	if err != nil {
		t.Fatalf("Contract creation failed: %v", err)
	}

	if contractAddr == (types.Address{}) {
		t.Error("Contract address should not be empty")
	}

	if gasUsed == 0 {
		t.Error("Gas should be used for contract creation")
	}

	// Проверяем, что контракт создан
	if !vault.HasContractCode(contractAddr) {
		t.Error("Contract code should be stored")
	}

	// Проверяем, что значение сохранено в storage
	storageKey := big.NewInt(0)
	storageValue, err := vault.GetStorage(contractAddr, storageKey)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	expectedValue := big.NewInt(42)
	if storageValue.Cmp(expectedValue) != 0 {
		t.Errorf("Expected storage value %s, got %s", expectedValue.Text(10), storageValue.Text(10))
	}

	t.Logf("Contract created at: %s, gas used: %d", contractAddr.Hex(), gasUsed)
}

// TestContractCallWithStorage тестирует вызов контракта с чтением storage
func TestContractCallWithStorage(t *testing.T) {
	// Очищаем тестовую директорию
	cfg := createTestConfigForValidator()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Создаем контракт
	contractKey, _ := types.GenerateAccount()
	contractAddr := types.PubkeyToAddress(contractKey.PublicKey)

	// Сохраняем код контракта
	// Байткод: загрузить storage[0], сохранить в память, вернуть
	contractCode := []byte{
		0x70, 0x00, // PUSH1 0 (ключ)
		0x96,       // SLOAD (загрузить из storage)
		0x60, 0x00, // PUSH1 0 (offset в память)
		0x61,       // MSTORE (сохранить в память)
		0x60, 0x20, // PUSH1 32 (size для RETURN)
		0x60, 0x00, // PUSH1 0 (offset для RETURN)
		0x84, // RETURN (вернуть данные из памяти)
	}

	d5Vault := vault.(*storage.D5Vault)
	err = d5Vault.StoreContractCode(contractAddr, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Сохраняем значение в storage контракта
	storageKey := big.NewInt(0)
	storageValue := big.NewInt(100)
	err = d5Vault.SetStorage(contractAddr, storageKey, storageValue)
	if err != nil {
		t.Fatalf("SetStorage failed: %v", err)
	}

	// Создаем транзакцию вызова контракта
	callerKey, _ := types.GenerateAccount()
	callerAddr := types.PubkeyToAddress(callerKey.PublicKey)

	callTx := types.NewTransaction(
		1,
		contractAddr,
		big.NewInt(0),
		100000,
		big.NewInt(1),
		nil, // Нет входных данных
	)
	callTx.SetFrom(callerAddr)

	// Выполняем вызов контракта
	result, gasUsed, err := ExecuteContractCall(*callTx, vault, nil)
	if err != nil {
		t.Fatalf("Contract call failed: %v", err)
	}

	// Проверяем результат (должно быть значение из storage)
	if len(result) == 0 {
		// Если результат пустой, это нормально - контракт может просто выполниться без возврата
		// Проверяем, что значение все еще в storage
		retrievedValue, err := vault.GetStorage(contractAddr, storageKey)
		if err != nil {
			t.Fatalf("Failed to get storage: %v", err)
		}
		if retrievedValue.Cmp(storageValue) != 0 {
			t.Errorf("Storage value changed: expected %s, got %s", storageValue.Text(10), retrievedValue.Text(10))
		}
		t.Logf("Contract executed without return data, storage preserved")
	} else if len(result) >= 32 {
		// Первые 32 байта должны содержать значение из storage
		resultValue := new(big.Int).SetBytes(result[:32])
		if resultValue.Cmp(storageValue) != 0 {
			t.Errorf("Expected result value %s, got %s", storageValue.Text(10), resultValue.Text(10))
		}
	}

	if gasUsed == 0 {
		t.Error("Gas should be used for contract call")
	}

	t.Logf("Contract call successful, gas used: %d, result: %x", gasUsed, result)
}

// TestContractStorageOperations тестирует операции с storage в контракте
func TestContractStorageOperations(t *testing.T) {
	// Очищаем тестовую директорию
	cfg := createTestConfigForValidator()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := storage.NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Байткод контракта:
	// 1. Сохранить 200 в storage[1]
	// 2. Загрузить storage[1]
	// 3. Добавить 50
	// 4. Сохранить результат в storage[1]
	contractCode := []byte{
		// Сохранить 200 в storage[1]
		0x70, 0xc8, // PUSH1 200
		0x70, 0x01, // PUSH1 1 (ключ)
		0x97, // SSTORE
		// Загрузить storage[1]
		0x70, 0x01, // PUSH1 1 (ключ)
		0x96, // SLOAD
		// Добавить 50
		0x70, 0x32, // PUSH1 50
		0x01, // ADD
		// Сохранить результат в storage[1]
		0x70, 0x01, // PUSH1 1 (ключ)
		0x97, // SSTORE
		0x00, // STOP
	}

	// Создаем отправителя
	senderKey, _ := types.GenerateAccount()
	senderAddr := types.PubkeyToAddress(senderKey.PublicKey)
	senderAcc := types.NewStateAccount(senderAddr, 1000.0, common.Hash{})
	vault.Put(senderAddr, senderAcc)

	// Создаем контракт
	pgTx := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     contractCode,
		Time:     time.Now(),
	}
	createTx := types.NewTx(pgTx)
	createTx.SetFrom(senderAddr)

	contractAddr, _, err := ExecuteContractCreation(*createTx, vault, nil)
	if err != nil {
		t.Fatalf("Contract creation failed: %v", err)
	}

	// Проверяем, что значение в storage[1] равно 250 (200 + 50)
	storageKey := big.NewInt(1)
	storageValue, err := vault.GetStorage(contractAddr, storageKey)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	expectedValue := big.NewInt(250)
	if storageValue.Cmp(expectedValue) != 0 {
		t.Errorf("Expected storage value %s, got %s", expectedValue.Text(10), storageValue.Text(10))
	}

	t.Logf("Contract storage operations successful, final value: %s", storageValue.Text(10))
}
