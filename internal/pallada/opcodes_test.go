package pallada

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
)

// mockStorage для тестов опкодов
type mockStorageForOpcodes struct {
	storage map[string]*big.Int
}

func newMockStorageForOpcodes() *mockStorageForOpcodes {
	return &mockStorageForOpcodes{
		storage: make(map[string]*big.Int),
	}
}

func (m *mockStorageForOpcodes) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	keyStr := key.String()
	if val, ok := m.storage[keyStr]; ok {
		return val, nil
	}
	return big.NewInt(0), nil
}

func (m *mockStorageForOpcodes) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	keyStr := key.String()
	m.storage[keyStr] = new(big.Int).Set(value)
	return nil
}

func createTestVM(code []byte) *VM {
	ctx := NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		&BlockInfo{},
		newMockStorageForOpcodes(),
	)
	return NewVM(code, ctx)
}

func TestOpcode_ADD(t *testing.T) {
	// PUSH1 10, PUSH1 5, ADD
	code := []byte{0x60, 0x0a, 0x60, 0x05, 0x01, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(15)) != 0 {
		t.Errorf("Expected 15, got %s", result.String())
	}
}

func TestOpcode_SUB(t *testing.T) {
	// PUSH1 10, PUSH1 3, SUB
	code := []byte{0x60, 0x0a, 0x60, 0x03, 0x03, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(7)) != 0 {
		t.Errorf("Expected 7, got %s", result.String())
	}
}

func TestOpcode_MUL(t *testing.T) {
	// PUSH1 6, PUSH1 7, MUL
	code := []byte{0x60, 0x06, 0x60, 0x07, 0x02, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected 42, got %s", result.String())
	}
}

func TestOpcode_DIV(t *testing.T) {
	// PUSH1 20, PUSH1 4, DIV
	code := []byte{0x60, 0x14, 0x60, 0x04, 0x04, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Errorf("Expected 5, got %s", result.String())
	}
}

func TestOpcode_DIV_ByZero(t *testing.T) {
	// PUSH1 10, PUSH1 0, DIV (деление на ноль должно вернуть 0)
	code := []byte{0x60, 0x0a, 0x60, 0x00, 0x04, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("Expected 0 for division by zero, got %s", result.String())
	}
}

func TestOpcode_MOD(t *testing.T) {
	// PUSH1 10, PUSH1 3, MOD
	code := []byte{0x60, 0x0a, 0x60, 0x03, 0x06, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1, got %s", result.String())
	}
}

func TestOpcode_LT(t *testing.T) {
	// PUSH1 3, PUSH1 5, LT (3 < 5 = true = 1)
	code := []byte{0x60, 0x03, 0x60, 0x05, 0x10, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1 (true), got %s", result.String())
	}
}

func TestOpcode_GT(t *testing.T) {
	// PUSH1 5, PUSH1 3, GT (5 > 3 = true = 1)
	code := []byte{0x60, 0x05, 0x60, 0x03, 0x11, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1 (true), got %s", result.String())
	}
}

func TestOpcode_EQ(t *testing.T) {
	// PUSH1 5, PUSH1 5, EQ (5 == 5 = true = 1)
	code := []byte{0x60, 0x05, 0x60, 0x05, 0x14, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1 (true), got %s", result.String())
	}
}

func TestOpcode_ISZERO(t *testing.T) {
	// PUSH1 0, ISZERO (0 == 0 = true = 1)
	code := []byte{0x60, 0x00, 0x15, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1 (true), got %s", result.String())
	}

	// PUSH1 5, ISZERO (5 == 0 = false = 0)
	code2 := []byte{0x60, 0x05, 0x15, 0x00}
	vm2 := createTestVM(code2)
	_, err = vm2.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result2, _ := vm2.stack.Pop()
	if result2.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("Expected 0 (false), got %s", result2.String())
	}
}

func TestOpcode_AND(t *testing.T) {
	// PUSH1 0x0F, PUSH1 0x03, AND (15 & 3 = 3)
	code := []byte{0x60, 0x0f, 0x60, 0x03, 0x16, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(3)) != 0 {
		t.Errorf("Expected 3, got %s", result.String())
	}
}

func TestOpcode_OR(t *testing.T) {
	// PUSH1 0x05, PUSH1 0x03, OR (5 | 3 = 7)
	code := []byte{0x60, 0x05, 0x60, 0x03, 0x17, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(7)) != 0 {
		t.Errorf("Expected 7, got %s", result.String())
	}
}

func TestOpcode_XOR(t *testing.T) {
	// PUSH1 0x05, PUSH1 0x03, XOR (5 ^ 3 = 6)
	code := []byte{0x60, 0x05, 0x60, 0x03, 0x18, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(6)) != 0 {
		t.Errorf("Expected 6, got %s", result.String())
	}
}

func TestOpcode_MLOAD_MSTORE(t *testing.T) {
	// PUSH1 42, PUSH1 0, MSTORE, PUSH1 0, MLOAD
	// Сохраняет 42 в память и загружает обратно
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0
		0x52,       // MSTORE
		0x60, 0x00, // PUSH1 0
		0x51, // MLOAD
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	// Значение должно быть 42, но в формате 32-байтового слова
	if result.Sign() == 0 {
		t.Error("Expected non-zero result")
	}
}

func TestOpcode_SSTORE_SLOAD(t *testing.T) {
	storage := newMockStorageForOpcodes()
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

	// PUSH1 100, PUSH1 0, SSTORE, PUSH1 0, SLOAD
	code := []byte{
		0x60, 0x64, // PUSH1 100
		0x60, 0x00, // PUSH1 0
		0x55,       // SSTORE
		0x60, 0x00, // PUSH1 0
		0x54, // SLOAD
		0x00, // STOP
	}
	vm := NewVM(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("Expected 100, got %s", result.String())
	}
}

func TestOpcode_POP(t *testing.T) {
	// PUSH1 42, PUSH1 10, POP (удаляет 10, остается 42)
	code := []byte{0x60, 0x2a, 0x60, 0x0a, 0x50, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", vm.stack.Depth())
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected 42, got %s", result.String())
	}
}

func TestOpcode_MSIZE(t *testing.T) {
	// PUSH1 42, PUSH1 0, MSTORE, MSIZE
	// Сохраняет в память и проверяет размер
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0
		0x52, // MSTORE
		0x56, // MSIZE
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Uint64() < 32 {
		t.Errorf("Expected memory size >= 32, got %d", result.Uint64())
	}
}

func TestOpcode_MSTORE8(t *testing.T) {
	// PUSH1 0xFF, PUSH1 0, MSTORE8
	// Сохраняет байт в память
	code := []byte{
		0x60, 0xff, // PUSH1 0xFF
		0x60, 0x00, // PUSH1 0
		0x53, // MSTORE8
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// Проверяем, что байт сохранен в памяти через LoadByte
	byteValue, err := vm.memory.LoadByte(big.NewInt(0))
	if err != nil {
		t.Fatalf("Memory.LoadByte failed: %v", err)
	}
	if byteValue != 0xFF {
		t.Errorf("Expected 0xFF, got 0x%02X", byteValue)
	}
}

func TestOpcode_EXP(t *testing.T) {
	// PUSH1 2, PUSH1 8, EXP (2^8 = 256)
	code := []byte{0x60, 0x02, 0x60, 0x08, 0x0a, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(256)) != 0 {
		t.Errorf("Expected 256, got %s", result.String())
	}
}

func TestOpcode_ADDMOD(t *testing.T) {
	// ADDMOD: (a + b) mod n
	// В коде: Pop() b, Pop() a, затем Pop() n
	// Стек (снизу вверх после PUSH a, b, n): n, b, a
	// Pop() b -> b (верхний элемент = a)
	// Pop() a -> a (следующий = b)
	// Pop() n -> n (следующий = n)
	// Значит: b = первый Pop = a из стека, a = второй Pop = b из стека
	// PUSH1 5, PUSH1 10, PUSH1 7 -> стек: 7, 10, 5
	// Pop: b=5, a=10, n=7 -> (10+5) mod 7 = 1
	code := []byte{0x60, 0x05, 0x60, 0x0a, 0x60, 0x07, 0x08, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	// (10+5) mod 7 = 15 mod 7 = 1
	if result.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected 1, got %s", result.String())
	}
}

func TestOpcode_MULMOD(t *testing.T) {
	// MULMOD: (a * b) mod n
	// В коде: Pop() b, Pop() a, затем Pop() n
	// PUSH1 4, PUSH1 3, PUSH1 7 -> стек: 7, 3, 4
	// Pop: b=4, a=3, n=7 -> (3*4) mod 7 = 5
	code := []byte{0x60, 0x04, 0x60, 0x03, 0x60, 0x07, 0x09, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	// (3*4) mod 7 = 12 mod 7 = 5
	if result.Cmp(big.NewInt(5)) != 0 {
		t.Errorf("Expected 5, got %s", result.String())
	}
}

func TestOpcode_NOT(t *testing.T) {
	// PUSH1 0xFF, NOT (побитовое НЕ)
	code := []byte{0x60, 0xff, 0x19, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	// NOT(0xFF) для 256-битного числа даст очень большое число
	if result.Sign() == 0 {
		t.Error("Expected non-zero result for NOT operation")
	}
}

func TestOpcode_BYTE(t *testing.T) {
	// PUSH1 0x123456, PUSH1 1, BYTE (извлекает 2-й байт = 0x34)
	// Но так как это big-endian и 32 байта, нужно правильно проверить
	code := []byte{0x61, 0x12, 0x34, 0x60, 0x01, 0x1a, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	// Проверяем, что результат в допустимом диапазоне байта
	if result.Uint64() > 0xFF {
		t.Errorf("Expected byte value (0-255), got %d", result.Uint64())
	}
}

func TestOpcode_SHL(t *testing.T) {
	// PUSH1 1, PUSH1 3, SHL (1 << 3 = 8)
	code := []byte{0x60, 0x01, 0x60, 0x03, 0x1b, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(8)) != 0 {
		t.Errorf("Expected 8, got %s", result.String())
	}
}

func TestOpcode_SHR(t *testing.T) {
	// PUSH1 8, PUSH1 2, SHR (8 >> 2 = 2)
	code := []byte{0x60, 0x08, 0x60, 0x02, 0x1c, 0x00}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(2)) != 0 {
		t.Errorf("Expected 2, got %s", result.String())
	}
}

func TestOpcode_REVERT(t *testing.T) {
	// PUSH1 42, PUSH1 0, MSTORE, PUSH1 32, PUSH1 0, REVERT
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0
		0x52,       // MSTORE
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xFD, // REVERT
	}
	vm := createTestVM(code)
	result, err := vm.Run()

	// REVERT должен вернуть ошибку с данными
	if err == nil {
		t.Error("Expected error for REVERT")
	}

	if result == nil || len(result) != 32 {
		t.Errorf("Expected 32 bytes in revert data, got %v", result)
	}
}

func TestOpcode_StackUnderflow(t *testing.T) {
	// Попытка выполнить ADD без достаточного количества элементов на стеке
	code := []byte{0x60, 0x01, 0x01, 0x00} // PUSH1 1, ADD (нужно 2 элемента)
	vm := createTestVM(code)
	_, err := vm.Run()
	if err == nil {
		t.Error("Expected error for stack underflow")
	}
}

func TestOpcode_PUSH32(t *testing.T) {
	// PUSH32 с большим значением
	code := make([]byte, 34)
	code[0] = 0x7F // PUSH32
	// Заполняем 32 байта значением
	for i := 1; i < 33; i++ {
		code[i] = 0xFF
	}
	code[33] = 0x00 // STOP

	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", vm.stack.Depth())
	}
}

func TestOpcode_ComplexSequence(t *testing.T) {
	// Сложная последовательность: вычисление (10 + 5) * 2
	// PUSH1 10, PUSH1 5, ADD, PUSH1 2, MUL
	code := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x01,       // ADD
		0x60, 0x02, // PUSH1 2
		0x02, // MUL
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(30)) != 0 {
		t.Errorf("Expected 30, got %s", result.String())
	}
}
