package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cerera/core/types"
	"github.com/cerera/config"
)

// createTestConfig создает тестовую конфигурацию
func createTestConfigForContract() *config.Config {
	privateKey, _ := types.GenerateAccount()
	privKeyString := types.EncodePrivateKeyToToString(privateKey)

	return &config.Config{
		Vault: config.VaultConfig{
			MEM:  false,
			PATH: filepath.Join(os.TempDir(), "test_vault_contract"),
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.PubkeyToAddress(privateKey.PublicKey),
			PRIV: privKeyString,
		},
		IN_MEM: false,
	}
}

// TestD5Vault_StoreContractCode тестирует сохранение кода контракта
func TestD5Vault_StoreContractCode(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес и код контракта
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3} // Простой байткод

	// Сохраняем код контракта
	err = d5Vault.StoreContractCode(address, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Проверяем, что аккаунт создан
	account := d5Vault.Get(address)
	if account == nil {
		t.Fatal("Account was not created")
	}

	// Проверяем, что HasContractCode возвращает true
	if !d5Vault.HasContractCode(address) {
		t.Error("HasContractCode should return true")
	}
}

// TestD5Vault_GetContractCode тестирует получение кода контракта
func TestD5Vault_GetContractCode(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес и код контракта
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3}

	// Сохраняем код
	err = d5Vault.StoreContractCode(address, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Получаем код
	retrievedCode, err := d5Vault.GetContractCode(address)
	if err != nil {
		t.Fatalf("GetContractCode failed: %v", err)
	}

	// Проверяем, что код совпадает
	if len(retrievedCode) != len(contractCode) {
		t.Errorf("Code length mismatch: expected %d, got %d", len(contractCode), len(retrievedCode))
	}
	for i := 0; i < len(contractCode); i++ {
		if retrievedCode[i] != contractCode[i] {
			t.Errorf("Code byte mismatch at position %d: expected 0x%02x, got 0x%02x", i, contractCode[i], retrievedCode[i])
		}
	}
}

// TestD5Vault_GetContractCode_NotFound тестирует получение несуществующего кода
func TestD5Vault_GetContractCode_NotFound(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем несуществующий адрес
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)

	// Пытаемся получить код
	_, err = d5Vault.GetContractCode(address)
	if err == nil {
		t.Error("Expected error when getting non-existent contract code")
	}
}

// TestD5Vault_HasContractCode тестирует проверку наличия кода контракта
func TestD5Vault_HasContractCode(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)

	// Проверяем, что кода нет
	if d5Vault.HasContractCode(address) {
		t.Error("HasContractCode should return false for address without code")
	}

	// Сохраняем код
	contractCode := []byte{0x60, 0x00, 0x52}
	err = d5Vault.StoreContractCode(address, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Проверяем, что код есть
	if !d5Vault.HasContractCode(address) {
		t.Error("HasContractCode should return true for address with code")
	}
}

// TestD5Vault_StoreContractCode_EmptyCode тестирует сохранение пустого кода
func TestD5Vault_StoreContractCode_EmptyCode(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)

	// Пытаемся сохранить пустой код
	err = d5Vault.StoreContractCode(address, []byte{})
	if err == nil {
		t.Error("Expected error when storing empty contract code")
	}
}

// TestD5Vault_StoreContractCode_UpdateExisting тестирует обновление существующего кода
func TestD5Vault_StoreContractCode_UpdateExisting(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)

	// Сохраняем первый код
	firstCode := []byte{0x60, 0x00, 0x52}
	err = d5Vault.StoreContractCode(address, firstCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	firstCodeRetrieved, err := d5Vault.GetContractCode(address)
	if err != nil || len(firstCodeRetrieved) == 0 {
		t.Fatalf("GetContractCode failed or returned empty: %v", err)
	}

	// Сохраняем второй код
	secondCode := []byte{0x60, 0x01, 0x52, 0x60, 0x20}
	err = d5Vault.StoreContractCode(address, secondCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed on update: %v", err)
	}

	secondCodeRetrieved, err := d5Vault.GetContractCode(address)
	if err != nil || len(secondCodeRetrieved) == 0 {
		t.Fatalf("GetContractCode failed or returned empty after update: %v", err)
	}

	// Проверяем, что код изменился
	if string(firstCodeRetrieved) == string(secondCodeRetrieved) {
		t.Error("Contract code should change when code is updated")
	}

	// Проверяем, что полученный код соответствует второму
	retrievedCode, err := d5Vault.GetContractCode(address)
	if err != nil {
		t.Fatalf("GetContractCode failed: %v", err)
	}
	if len(retrievedCode) != len(secondCode) {
		t.Errorf("Retrieved code length mismatch: expected %d, got %d", len(secondCode), len(retrievedCode))
	}
}

// TestD5Vault_DeleteContractCode тестирует удаление кода контракта
func TestD5Vault_DeleteContractCode(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем тестовый адрес и код контракта
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)
	contractCode := []byte{0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3}

	// Сохраняем код
	err = d5Vault.StoreContractCode(address, contractCode)
	if err != nil {
		t.Fatalf("StoreContractCode failed: %v", err)
	}

	// Проверяем, что код есть
	if !d5Vault.HasContractCode(address) {
		t.Error("HasContractCode should return true before deletion")
	}

	// Удаляем код
	err = d5Vault.DeleteContractCode(address)
	if err != nil {
		t.Fatalf("DeleteContractCode failed: %v", err)
	}

	// Проверяем, что код удален
	if d5Vault.HasContractCode(address) {
		t.Error("HasContractCode should return false after deletion")
	}

	// Проверяем, что получение кода возвращает ошибку
	_, err = d5Vault.GetContractCode(address)
	if err == nil {
		t.Error("Expected error when getting deleted contract code")
	}
}

// TestD5Vault_DeleteContractCode_NonExistent тестирует удаление несуществующего кода
func TestD5Vault_DeleteContractCode_NonExistent(t *testing.T) {
	// Очищаем тестовую директорию перед тестом
	cfg := createTestConfigForContract()
	os.RemoveAll(cfg.Vault.PATH)

	vault, err := NewD5Vault(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewD5Vault failed: %v", err)
	}
	defer func() {
		vault.Close()
		os.RemoveAll(cfg.Vault.PATH)
	}()

	// Приводим к *D5Vault
	d5Vault := vault.(*D5Vault)

	// Создаем несуществующий адрес
	privateKey, _ := types.GenerateAccount()
	address := types.PubkeyToAddress(privateKey.PublicKey)

	// Удаление несуществующего кода не должно вызывать ошибку
	err = d5Vault.DeleteContractCode(address)
	if err != nil {
		t.Errorf("DeleteContractCode should not fail for non-existent code: %v", err)
	}
}
