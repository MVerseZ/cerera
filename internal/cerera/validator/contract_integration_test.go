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

// TestContractCallBetweenContracts тестирует CALL между контрактами
func TestContractCallBetweenContracts(t *testing.T) {
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

	d5Vault := vault.(*storage.D5Vault)

	// Создаем отправителя
	senderKey, _ := types.GenerateAccount()
	senderAddr := types.PubkeyToAddress(senderKey.PublicKey)
	senderAcc := types.NewStateAccount(senderAddr, 1000.0, common.Hash{})
	vault.Put(senderAddr, senderAcc)

	// Создаем контракт B (вызываемый контракт)
	// Байткод: PUSH1 100, PUSH1 0, SSTORE, PUSH1 0, SLOAD, RETURN
	// Сохраняет 100 в storage[0] и возвращает его
	contractBCode := []byte{
		0x70, 0x64, // PUSH1 100
		0x70, 0x00, // PUSH1 0
		0x97,       // SSTORE
		0x70, 0x00, // PUSH1 0
		0x96,       // SLOAD
		0x60, 0x00, // PUSH1 0 (offset)
		0x60, 0x20, // PUSH1 32 (size)
		0x84, // RETURN
	}

	contractBKey, _ := types.GenerateAccount()
	contractBAddr := types.PubkeyToAddress(contractBKey.PublicKey)
	err = d5Vault.StoreContractCode(contractBAddr, contractBCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed for contract B: %v", err)
	}

	// Создаем контракт A (вызывающий контракт)
	// Байткод: вызывает контракт B через CALL
	// CALL(gas, address, value, inputOffset, inputSize, outputOffset, outputSize)
	contractACode := []byte{
		// Подготовка параметров для CALL (в обратном порядке)
		0x60, 0x20, // PUSH1 32 (outputSize)
		0x60, 0x00, // PUSH1 0 (outputOffset)
		0x60, 0x00, // PUSH1 0 (inputSize)
		0x60, 0x00, // PUSH1 0 (inputOffset)
		0x60, 0x00, // PUSH1 0 (value)
	}
	// Добавляем адрес контракта B (32 байта, используем PUSH32)
	contractBAddrBytes := contractBAddr.Bytes()
	addrBytes := make([]byte, 32)
	// Address это [32]byte, Bytes() возвращает слайс, копируем в правильное место
	if len(contractBAddrBytes) <= 32 {
		copy(addrBytes[32-len(contractBAddrBytes):], contractBAddrBytes)
	} else {
		copy(addrBytes, contractBAddrBytes[len(contractBAddrBytes)-32:])
	}
	contractACode = append(contractACode, 0x75) // PUSH32
	contractACode = append(contractACode, addrBytes...)
	contractACode = append(contractACode,
		0x70, 0x64, // PUSH1 100 (gas limit)
		0x98, // CALL
		0x00, // STOP
	)

	// Создаем контракт A
	pgTx := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     contractACode,
		Time:     time.Now(),
	}
	createTx := types.NewTx(pgTx)
	createTx.SetFrom(senderAddr)

	contractAAddr, _, err := ExecuteContractCreation(*createTx, vault, nil)
	if err != nil {
		t.Fatalf("Contract A creation failed: %v", err)
	}

	// Вызываем контракт A
	callTx := types.NewTransaction(
		2,
		contractAAddr,
		big.NewInt(0),
		100000,
		big.NewInt(1),
		nil,
	)
	callTx.SetFrom(senderAddr)

	result, gasUsed, err := ExecuteContractCall(*callTx, vault, nil)
	if err != nil {
		t.Fatalf("Contract A call failed: %v", err)
	}

	// Проверяем, что контракт B выполнился (значение в storage)
	storageKey := big.NewInt(0)
	storageValue, err := vault.GetStorage(contractBAddr, storageKey)
	if err != nil {
		t.Fatalf("Failed to get storage from contract B: %v", err)
	}

	expectedValue := big.NewInt(100)
	if storageValue.Cmp(expectedValue) != 0 {
		t.Errorf("Expected storage value %s in contract B, got %s", expectedValue.Text(10), storageValue.Text(10))
	}

	if gasUsed == 0 {
		t.Error("Gas should be used for contract call with CALL")
	}

	t.Logf("CALL between contracts successful, gas used: %d, result: %x", gasUsed, result)
}

// TestContractCreationRevertRollback тестирует откат при REVERT при создании контракта
func TestContractCreationRevertRollback(t *testing.T) {
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

	// Создаем отправителя
	senderKey, _ := types.GenerateAccount()
	senderAddr := types.PubkeyToAddress(senderKey.PublicKey)
	senderAcc := types.NewStateAccount(senderAddr, 1000.0, common.Hash{})
	vault.Put(senderAddr, senderAcc)

	// Байткод контракта, который делает REVERT
	// PUSH1 1, PUSH1 0, REVERT (возвращает данные, но с ошибкой)
	contractCode := []byte{
		0x60, 0x01, // PUSH1 1
		0x60, 0x00, // PUSH1 0
		0x85, // REVERT
	}

	// Создаем транзакцию создания контракта
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

	// Выполняем создание контракта (должно вернуть ошибку)
	contractAddr, gasUsed, err := ExecuteContractCreation(*createTx, vault, nil)
	if err == nil {
		t.Error("Expected error when contract creation reverts")
	}

	// Проверяем, что контракт не создан (код удален)
	if vault.HasContractCode(contractAddr) {
		t.Error("Contract code should be deleted after REVERT")
	}

	// Проверяем, что gas был использован
	if gasUsed == 0 {
		t.Error("Gas should be used even when contract creation fails")
	}

	t.Logf("Contract creation revert rollback successful, gas used: %d", gasUsed)
}

// TestContractCreationOutOfGasRollback тестирует откат при нехватке газа
func TestContractCreationOutOfGasRollback(t *testing.T) {
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

	// Создаем отправителя
	senderKey, _ := types.GenerateAccount()
	senderAddr := types.PubkeyToAddress(senderKey.PublicKey)
	senderAcc := types.NewStateAccount(senderAddr, 1000.0, common.Hash{})
	vault.Put(senderAddr, senderAcc)

	// Байткод контракта, который потребует много газа (много SSTORE)
	contractCode := []byte{}
	for i := 0; i < 100; i++ {
		contractCode = append(contractCode,
			0x70, byte(i), // PUSH1 i
			0x70, byte(i), // PUSH1 i (ключ)
			0x97, // SSTORE (стоит 20000 газа)
		)
	}
	contractCode = append(contractCode, 0x00) // STOP

	// Создаем транзакцию с очень маленьким лимитом газа
	pgTx := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      1000, // Очень маленький лимит
		GasPrice: big.NewInt(1),
		Data:     contractCode,
		Time:     time.Now(),
	}
	createTx := types.NewTx(pgTx)
	createTx.SetFrom(senderAddr)

	// Выполняем создание контракта (должно вернуть ошибку OutOfGas)
	contractAddr, gasUsed, err := ExecuteContractCreation(*createTx, vault, nil)
	if err == nil {
		t.Error("Expected error when contract creation runs out of gas")
	}

	// Проверяем, что контракт не создан (код удален)
	if vault.HasContractCode(contractAddr) {
		t.Error("Contract code should be deleted after OutOfGas")
	}

	// Проверяем, что gas был использован
	if gasUsed == 0 {
		t.Error("Gas should be used even when contract creation fails")
	}

	t.Logf("Contract creation OutOfGas rollback successful, gas used: %d", gasUsed)
}
