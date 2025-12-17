package validator

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
)

// createTestConfigForValidator создает тестовую конфигурацию для валидатора
func createTestConfigForValidator() *config.Config {
	privateKey, _ := types.GenerateAccount()
	privKeyString := types.EncodePrivateKeyToToString(privateKey)

	return &config.Config{
		Vault: config.VaultConfig{
			MEM:  false,
			PATH: filepath.Join(os.TempDir(), "test_vault_validator"),
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.PubkeyToAddress(privateKey.PublicKey),
			PRIV: privKeyString,
		},
		IN_MEM: false,
	}
}

// TestIsContractCreation тестирует определение создания контракта
func TestIsContractCreation(t *testing.T) {
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

	// Создаем транзакцию создания контракта (To == nil, Data != nil)
	// Используем PGTransaction с nil To
	pgTx := &types.PGTransaction{
		Nonce:    1,
		To:       nil, // nil для создания контракта
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x60, 0x00, 0x52},
		Time:     time.Now(),
	}
	contractTx := types.NewTx(pgTx)

	// Проверяем, что IsContractCreation правильно определяет создание контракта
	if !IsContractCreation(*contractTx) {
		t.Error("Expected IsContractCreation to return true for To == nil && Data != nil")
	}

	// Транзакция с To != nil не должна быть созданием контракта
	// Создаем обычную транзакцию
	normalKey, _ := types.GenerateAccount()
	normalAddress := types.PubkeyToAddress(normalKey.PublicKey)
	txNormal := types.NewTransaction(
		1,
		normalAddress,
		big.NewInt(0),
		100000,
		big.NewInt(1),
		[]byte{0x60, 0x00, 0x52},
	)
	if IsContractCreation(*txNormal) {
		t.Error("Expected IsContractCreation to return false for To != nil")
	}

	// Транзакция с To == nil && Data == nil не должна быть созданием контракта
	pgTxNoData := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     nil,
		Time:     time.Now(),
	}
	txNoData := types.NewTx(pgTxNoData)
	if IsContractCreation(*txNoData) {
		t.Error("Expected IsContractCreation to return false for To == nil && Data == nil")
	}
}

// TestIsContractCall тестирует определение вызова контракта
func TestIsContractCall(t *testing.T) {
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

	// Создаем контракт (сохраняем код)
	privateKey, _ := types.GenerateAccount()
	contractAddress := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3}

	d5Vault := vault.(*storage.D5Vault)
	err = d5Vault.StoreContractCode(contractAddress, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Создаем транзакцию вызова контракта (To != nil && HasContractCode)
	callerKey, _ := types.GenerateAccount()
	callerAddress := types.PubkeyToAddress(callerKey.PublicKey)

	tx := types.NewTransaction(
		1,
		contractAddress, // Адрес контракта
		big.NewInt(100),
		100000,
		big.NewInt(1),
		[]byte{0x01, 0x02, 0x03}, // Данные для вызова
	)

	// Устанавливаем From
	tx.SetFrom(callerAddress)

	if !IsContractCall(*tx) {
		t.Error("Expected IsContractCall to return true for To with contract code")
	}

	// Транзакция к адресу без кода не должна быть вызовом контракта
	normalKey, _ := types.GenerateAccount()
	normalAddress := types.PubkeyToAddress(normalKey.PublicKey)

	txNormal := types.NewTransaction(
		1,
		normalAddress, // Обычный адрес без кода
		big.NewInt(100),
		100000,
		big.NewInt(1),
		nil, // Нет данных
	)
	txNormal.SetFrom(callerAddress)

	if IsContractCall(*txNormal) {
		t.Error("Expected IsContractCall to return false for To without contract code")
	}

	// Транзакция с To == nil не должна быть вызовом контракта
	pgTxNil := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x01, 0x02},
	}
	txNil := types.NewTx(pgTxNil)
	if IsContractCall(*txNil) {
		t.Error("Expected IsContractCall to return false for To == nil")
	}
}

// TestIsContractTransaction тестирует общую функцию определения контрактных транзакций
func TestIsContractTransaction(t *testing.T) {
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

	// Создание контракта
	pgTxCreate := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x60, 0x00, 0x52},
	}
	txCreate := types.NewTx(pgTxCreate)

	if !IsContractTransaction(*txCreate) {
		t.Error("Expected IsContractTransaction to return true for contract creation")
	}

	// Вызов контракта
	privateKey, _ := types.GenerateAccount()
	contractAddress := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52}

	d5Vault := vault.(*storage.D5Vault)
	err = d5Vault.StoreContractCode(contractAddress, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	callerKey, _ := types.GenerateAccount()
	callerAddress := types.PubkeyToAddress(callerKey.PublicKey)

	txCall := types.NewTransaction(
		1,
		contractAddress,
		big.NewInt(0),
		100000,
		big.NewInt(1),
		[]byte{0x01},
	)
	txCall.SetFrom(callerAddress)

	if !IsContractTransaction(*txCall) {
		t.Error("Expected IsContractTransaction to return true for contract call")
	}

	// Обычная транзакция
	normalKey, _ := types.GenerateAccount()
	normalAddress := types.PubkeyToAddress(normalKey.PublicKey)

	txNormal := types.NewTransaction(
		1,
		normalAddress,
		big.NewInt(100),
		100000,
		big.NewInt(1),
		nil,
	)
	txNormal.SetFrom(callerAddress)

	if IsContractTransaction(*txNormal) {
		t.Error("Expected IsContractTransaction to return false for normal transaction")
	}
}

// TestShouldExecuteVM тестирует функцию определения необходимости выполнения VM
func TestShouldExecuteVM(t *testing.T) {
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

	// Транзакция типа AppTxType должна требовать выполнения VM
	// Но для этого нужно создать транзакцию с типом AppTxType
	// Пока проверяем по содержимому

	// Создание контракта
	pgTxCreate := &types.PGTransaction{
		Nonce:    1,
		To:       nil,
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1),
		Data:     []byte{0x60, 0x00, 0x52},
	}
	txCreate := types.NewTx(pgTxCreate)

	if !ShouldExecuteVM(*txCreate) {
		t.Error("Expected ShouldExecuteVM to return true for contract creation")
	}

	// Вызов контракта
	privateKey, _ := types.GenerateAccount()
	contractAddress := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52}

	d5Vault := vault.(*storage.D5Vault)
	err = d5Vault.StoreContractCode(contractAddress, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	callerKey, _ := types.GenerateAccount()
	callerAddress := types.PubkeyToAddress(callerKey.PublicKey)

	txCall := types.NewTransaction(
		1,
		contractAddress,
		big.NewInt(0),
		100000,
		big.NewInt(1),
		[]byte{0x01},
	)
	txCall.SetFrom(callerAddress)

	if !ShouldExecuteVM(*txCall) {
		t.Error("Expected ShouldExecuteVM to return true for contract call")
	}

	// Обычная транзакция
	normalKey, _ := types.GenerateAccount()
	normalAddress := types.PubkeyToAddress(normalKey.PublicKey)

	txNormal := types.NewTransaction(
		1,
		normalAddress,
		big.NewInt(100),
		100000,
		big.NewInt(1),
		nil,
	)
	txNormal.SetFrom(callerAddress)

	if ShouldExecuteVM(*txNormal) {
		t.Error("Expected ShouldExecuteVM to return false for normal transaction")
	}
}
