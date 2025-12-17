package pallada

import (
	"math/big"
	"testing"

	"github.com/cerera/internal/cerera/types"
)

// MockStorage реализует StorageInterface для тестирования
type MockStorage struct {
	storage map[string]*big.Int
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		storage: make(map[string]*big.Int),
	}
}

func (m *MockStorage) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	storageKey := address.Hex() + ":" + key.Text(16)
	if value, ok := m.storage[storageKey]; ok {
		return new(big.Int).Set(value), nil
	}
	return big.NewInt(0), nil
}

func (m *MockStorage) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	storageKey := address.Hex() + ":" + key.Text(16)
	if value == nil {
		value = big.NewInt(0)
	}
	m.storage[storageKey] = new(big.Int).Set(value)
	return nil
}

// TestVM_Sload тестирует опкод SLOAD
func TestVM_Sload(t *testing.T) {
	// Создаем тестовый адрес контракта
	contractAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	// Создаем mock storage
	mockStorage := NewMockStorage()

	// Сохраняем тестовое значение
	testKey := big.NewInt(42)
	testValue := big.NewInt(100)
	mockStorage.SetStorage(contractAddr, testKey, testValue)

	// Байткод: PUSH32 <key>, SLOAD, STOP
	keyBytes := make([]byte, 32)
	testKey.FillBytes(keyBytes)

	code := make([]byte, 0)
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	code = append(code, byte(SLOAD))
	code = append(code, byte(STOP))

	// Создаем Context с storage
	ctx := NewContextWithStorage(
		types.Address{}, // Caller
		contractAddr,    // Address
		big.NewInt(0),   // Value
		nil,             // Input
		100000,          // GasLimit
		big.NewInt(1),   // GasPrice
		nil,             // BlockInfo
		mockStorage,     // Storage
	)

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что значение загружено на стек
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack is empty: %v", err)
	}

	if stackValue.Cmp(testValue) != 0 {
		t.Errorf("Expected value %s, got %s", testValue.Text(10), stackValue.Text(10))
	}

	// Проверяем газ
	if vm.GasUsed() == 0 {
		t.Error("Gas should be used for SLOAD")
	}
}

// TestVM_Sstore тестирует опкод SSTORE
func TestVM_Sstore(t *testing.T) {
	// Создаем тестовый адрес контракта
	contractAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	// Создаем mock storage
	mockStorage := NewMockStorage()

	// Байткод: PUSH32 <value>, PUSH32 <key>, SSTORE, STOP
	testKey := big.NewInt(42)
	testValue := big.NewInt(200)

	keyBytes := make([]byte, 32)
	testKey.FillBytes(keyBytes)
	valueBytes := make([]byte, 32)
	testValue.FillBytes(valueBytes)

	code := make([]byte, 0)
	code = append(code, byte(PUSH32))
	code = append(code, valueBytes...)
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	code = append(code, byte(SSTORE))
	code = append(code, byte(STOP))

	// Создаем Context с storage
	ctx := NewContextWithStorage(
		types.Address{}, // Caller
		contractAddr,    // Address
		big.NewInt(0),   // Value
		nil,             // Input
		100000,          // GasLimit
		big.NewInt(1),   // GasPrice
		nil,             // BlockInfo
		mockStorage,     // Storage
	)

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что значение сохранено
	retrievedValue, err := mockStorage.GetStorage(contractAddr, testKey)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	if retrievedValue.Cmp(testValue) != 0 {
		t.Errorf("Expected value %s, got %s", testValue.Text(10), retrievedValue.Text(10))
	}

	// Проверяем газ (SSTORE стоит много)
	if vm.GasUsed() < 20000 {
		t.Errorf("Expected at least 20000 gas for SSTORE, got %d", vm.GasUsed())
	}
}

// TestVM_SloadSstore тестирует последовательность SLOAD и SSTORE
func TestVM_SloadSstore(t *testing.T) {
	// Создаем тестовый адрес контракта
	contractAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	// Создаем mock storage
	mockStorage := NewMockStorage()

	// Сначала сохраняем значение
	testKey := big.NewInt(1)
	testValue := big.NewInt(50)
	mockStorage.SetStorage(contractAddr, testKey, testValue)

	// Байткод: PUSH32 <key>, SLOAD, PUSH1 10, ADD, PUSH32 <key>, SSTORE, STOP
	keyBytes := make([]byte, 32)
	testKey.FillBytes(keyBytes)

	code := make([]byte, 0)
	// Загружаем ключ
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	// SLOAD
	code = append(code, byte(SLOAD))
	// Добавляем 10
	code = append(code, byte(PUSH1))
	code = append(code, 10)
	code = append(code, byte(ADD))
	// Снова загружаем ключ для SSTORE
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	// SSTORE (value, key)
	code = append(code, byte(SSTORE))
	code = append(code, byte(STOP))

	// Создаем Context с storage
	ctx := NewContextWithStorage(
		types.Address{}, // Caller
		contractAddr,    // Address
		big.NewInt(0),   // Value
		nil,             // Input
		100000,          // GasLimit
		big.NewInt(1),   // GasPrice
		nil,             // BlockInfo
		mockStorage,     // Storage
	)

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что значение обновлено (50 + 10 = 60)
	expectedValue := big.NewInt(60)
	retrievedValue, err := mockStorage.GetStorage(contractAddr, testKey)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	if retrievedValue.Cmp(expectedValue) != 0 {
		t.Errorf("Expected value %s, got %s", expectedValue.Text(10), retrievedValue.Text(10))
	}
}

// TestVM_Sload_EmptyStorage тестирует SLOAD из пустого storage
func TestVM_Sload_EmptyStorage(t *testing.T) {
	// Создаем тестовый адрес контракта
	contractAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	// Создаем пустой mock storage
	mockStorage := NewMockStorage()

	// Байткод: PUSH32 <key>, SLOAD, STOP
	testKey := big.NewInt(999)
	keyBytes := make([]byte, 32)
	testKey.FillBytes(keyBytes)

	code := make([]byte, 0)
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	code = append(code, byte(SLOAD))
	code = append(code, byte(STOP))

	// Создаем Context с storage
	ctx := NewContextWithStorage(
		types.Address{}, // Caller
		contractAddr,    // Address
		big.NewInt(0),   // Value
		nil,             // Input
		100000,          // GasLimit
		big.NewInt(1),   // GasPrice
		nil,             // BlockInfo
		mockStorage,     // Storage
	)

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что возвращается 0 для несуществующего ключа
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack is empty: %v", err)
	}

	if stackValue.Sign() != 0 {
		t.Errorf("Expected 0 for empty storage, got %s", stackValue.Text(10))
	}
}

// TestVM_Sstore_ZeroValue тестирует SSTORE с нулевым значением (удаление)
func TestVM_Sstore_ZeroValue(t *testing.T) {
	// Создаем тестовый адрес контракта
	contractAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")

	// Создаем mock storage
	mockStorage := NewMockStorage()

	// Сначала сохраняем ненулевое значение
	testKey := big.NewInt(1)
	testValue := big.NewInt(100)
	mockStorage.SetStorage(contractAddr, testKey, testValue)

	// Байткод: PUSH1 0, PUSH32 <key>, SSTORE, STOP
	keyBytes := make([]byte, 32)
	testKey.FillBytes(keyBytes)

	code := make([]byte, 0)
	code = append(code, byte(PUSH1))
	code = append(code, 0)
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes...)
	code = append(code, byte(SSTORE))
	code = append(code, byte(STOP))

	// Создаем Context с storage
	ctx := NewContextWithStorage(
		types.Address{}, // Caller
		contractAddr,    // Address
		big.NewInt(0),   // Value
		nil,             // Input
		100000,          // GasLimit
		big.NewInt(1),   // GasPrice
		nil,             // BlockInfo
		mockStorage,     // Storage
	)

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что значение установлено в 0
	retrievedValue, err := mockStorage.GetStorage(contractAddr, testKey)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	if retrievedValue.Sign() != 0 {
		t.Errorf("Expected 0 after storing zero, got %s", retrievedValue.Text(10))
	}
}
