package pallada

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
)

// mockStorage реализует StorageInterface для тестов
type mockStorage struct {
	storage map[string]*big.Int
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		storage: make(map[string]*big.Int),
	}
}

func (m *mockStorage) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	keyStr := key.String()
	if val, ok := m.storage[keyStr]; ok {
		return val, nil
	}
	return big.NewInt(0), nil
}

func (m *mockStorage) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	keyStr := key.String()
	m.storage[keyStr] = new(big.Int).Set(value)
	return nil
}

func TestVM_SimpleExecution(t *testing.T) {
	// Простой байткод: PUSH1 42, PUSH1 0, MSTORE, STOP
	// Сохраняет 42 в память по адресу 0
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0
		0x52, // MSTORE
		0x00, // STOP
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	result, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result for STOP, got %x", result)
	}

	// Проверяем, что значение сохранено в памяти
	memValue, err := vm.memory.Load(big.NewInt(0))
	if err != nil {
		t.Fatalf("Memory.Load failed: %v", err)
	}

	if memValue.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected memory value 42, got %s", memValue.String())
	}
}

func TestVM_Arithmetic(t *testing.T) {
	// Байткод: PUSH1 10, PUSH1 5, ADD, STOP
	// Вычисляет 10 + 5 = 15
	code := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x01, // ADD
		0x00, // STOP
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Проверяем результат на стеке (должен быть 15)
	if vm.stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", vm.stack.Depth())
	}

	result, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack.Pop failed: %v", err)
	}

	if result.Cmp(big.NewInt(15)) != 0 {
		t.Errorf("Expected result 15, got %s", result.String())
	}
}

func TestVM_Return(t *testing.T) {
	// Байткод: PUSH1 42, PUSH1 0, MSTORE, PUSH1 32, PUSH1 0, RETURN
	// Сохраняет 42 в память и возвращает 32 байта
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0
		0x52,       // MSTORE
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xF3, // RETURN
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	result, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil || len(result) != 32 {
		t.Errorf("Expected 32 bytes result, got %v", result)
	}

	// Проверяем, что первые байты содержат 42 (big-endian)
	expected := big.NewInt(42)
	actual := new(big.Int).SetBytes(result)
	if actual.Cmp(expected) != 0 {
		t.Errorf("Expected result value 42, got %s", actual.String())
	}
}

func TestVM_OutOfGas(t *testing.T) {
	// Байткод с множеством операций
	code := []byte{
		0x60, 0x01, // PUSH1 1
		0x60, 0x02, // PUSH1 2
		0x01,       // ADD
		0x60, 0x03, // PUSH1 3
		0x01, // ADD
		// ... много операций
	}

	// Очень маленький лимит газа
	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10, // Очень маленький лимит
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err == nil {
		t.Error("Expected out of gas error")
	}
}

func TestVM_StorageOperations(t *testing.T) {
	// Байткод: PUSH1 100, PUSH1 0, SSTORE, PUSH1 0, SLOAD, STOP
	// Сохраняет 100 в storage[0] и загружает обратно
	code := []byte{
		0x60, 0x64, // PUSH1 100
		0x60, 0x00, // PUSH1 0
		0x55,       // SSTORE
		0x60, 0x00, // PUSH1 0
		0x54, // SLOAD
		0x00, // STOP
	}

	storage := newMockStorage()
	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		&BlockInfo{},
		storage,
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Проверяем, что значение загружено на стек
	if vm.stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", vm.stack.Depth())
	}

	result, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack.Pop failed: %v", err)
	}

	if result.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("Expected result 100, got %s", result.String())
	}
}

func TestVM_Comparison(t *testing.T) {
	// Байткод: PUSH1 10, PUSH1 5, GT, STOP
	// Проверяет 10 > 5 (должно вернуть 1)
	code := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x11, // GT
		0x00, // STOP
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	result, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack.Pop failed: %v", err)
	}

	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected result 1 (true), got %s", result.String())
	}
}

func TestVM_InvalidOpcode(t *testing.T) {
	// Байткод с недопустимым опкодом
	code := []byte{
		0xFF, // Неизвестный опкод
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err == nil {
		t.Error("Expected error for invalid opcode")
	}
}

func TestVM_GasUsed(t *testing.T) {
	code := []byte{
		0x60, 0x01, // PUSH1 1
		0x60, 0x02, // PUSH1 2
		0x01, // ADD
		0x00, // STOP
	}

	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorage(),
	)

	vm := NewVM(code, ctx)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	gasUsed := vm.GasUsed()
	if gasUsed == 0 {
		t.Error("Expected gas to be used")
	}

	if gasUsed > ctx.GasLimit {
		t.Errorf("Gas used (%d) exceeds limit (%d)", gasUsed, ctx.GasLimit)
	}
}
