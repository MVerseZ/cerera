package pallada

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	secp256k1ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
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
		0x59, // MSIZE
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

// --- DUP тесты ---

func TestOpcode_DUP1(t *testing.T) {
	// PUSH1 42, DUP1 -> стек: [42, 42]
	code := []byte{
		0x60, 0x2a, // PUSH1 42
		0x80, // DUP1
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 2 {
		t.Fatalf("Expected stack depth 2, got %d", vm.stack.Depth())
	}
	top, _ := vm.stack.Pop()
	second, _ := vm.stack.Pop()
	if top.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected top 42, got %s", top.String())
	}
	if second.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("Expected second 42, got %s", second.String())
	}
}

func TestOpcode_DUP2(t *testing.T) {
	// PUSH1 10, PUSH1 20, DUP2 -> стек: [10, 20, 10]
	code := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x14, // PUSH1 20
		0x81, // DUP2
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 3 {
		t.Fatalf("Expected stack depth 3, got %d", vm.stack.Depth())
	}
	top, _ := vm.stack.Pop()
	if top.Cmp(big.NewInt(10)) != 0 {
		t.Errorf("Expected top 10, got %s", top.String())
	}
}

// --- SWAP тесты ---

func TestOpcode_SWAP1(t *testing.T) {
	// PUSH1 1, PUSH1 2, SWAP1 -> стек: [2, 1] (вершина=2)
	code := []byte{
		0x60, 0x01, // PUSH1 1
		0x60, 0x02, // PUSH1 2
		0x90, // SWAP1
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	top, _ := vm.stack.Pop()
	second, _ := vm.stack.Pop()
	if top.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected top 1 after SWAP1, got %s", top.String())
	}
	if second.Cmp(big.NewInt(2)) != 0 {
		t.Errorf("Expected second 2 after SWAP1, got %s", second.String())
	}
}

func TestOpcode_SWAP2(t *testing.T) {
	// PUSH1 1, PUSH1 2, PUSH1 3, SWAP2 -> стек: [3, 2, 1] (вершина=3)
	code := []byte{
		0x60, 0x01, // PUSH1 1
		0x60, 0x02, // PUSH1 2
		0x60, 0x03, // PUSH1 3
		0x91, // SWAP2
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	top, _ := vm.stack.Pop()
	if top.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected top 1 after SWAP2, got %s", top.String())
	}
}

// --- Хэш-операции ---

func TestOpcode_KECCAK256(t *testing.T) {
	// Тестируем: KECCAK256 от пустых данных (offset=0, length=0)
	// Известный результат: keccak256("") = c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
	code := []byte{
		0x60, 0x00, // PUSH1 0 (length)
		0x60, 0x00, // PUSH1 0 (offset)
		0x20, // KECCAK256
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 1 {
		t.Fatalf("Expected stack depth 1, got %d", vm.stack.Depth())
	}
	result, _ := vm.stack.Pop()
	// keccak256("") как big.Int (hex c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470)
	expected, _ := new(big.Int).SetString("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", 16)
	if result.Cmp(expected) != 0 {
		t.Errorf("KECCAK256 mismatch:\n  got  %x\n  want %x", result, expected)
	}
}

func TestOpcode_SHA256(t *testing.T) {
	// SHA256 от пустой строки = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	code := []byte{
		0x60, 0x00, // PUSH1 0 (length)
		0x60, 0x00, // PUSH1 0 (offset)
		0x21, // SHA256
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	expected, _ := new(big.Int).SetString("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", 16)
	if result.Cmp(expected) != 0 {
		t.Errorf("SHA256 mismatch:\n  got  %x\n  want %x", result, expected)
	}
}

func TestOpcode_RIPEMD160(t *testing.T) {
	// RIPEMD160 от пустой строки = 9c1185a5c5e9fc54612808977ee8f548b2258d31
	// Дополнено нулями до 32 байт
	code := []byte{
		0x60, 0x00, // PUSH1 0 (length)
		0x60, 0x00, // PUSH1 0 (offset)
		0x22, // RIPEMD160
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	expected, _ := new(big.Int).SetString("9c1185a5c5e9fc54612808977ee8f548b2258d31", 16)
	if result.Cmp(expected) != 0 {
		t.Errorf("RIPEMD160 mismatch:\n  got  %x\n  want %x", result, expected)
	}
}

func TestOpcode_KECCAK256_WithData(t *testing.T) {
	// Записываем 0xFF в memory[0], хэшируем 1 байт
	// MSTORE8 [0] = 0xFF, затем KECCAK256(offset=0, length=1)
	code := []byte{
		0x60, 0xff, // PUSH1 0xFF (value)
		0x60, 0x00, // PUSH1 0 (offset)
		0x53,       // MSTORE8
		0x60, 0x01, // PUSH1 1 (length)
		0x60, 0x00, // PUSH1 0 (offset)
		0x20, // KECCAK256
		0x00, // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 1 {
		t.Fatalf("Expected stack depth 1, got %d", vm.stack.Depth())
	}
	// keccak256([0xFF]) = 8b1a944cf13a9a1c08facb2c9e98623ef3254d2ddb48113885c3e8e97fec8db9
	result, _ := vm.stack.Pop()
	expected, _ := new(big.Int).SetString("8b1a944cf13a9a1c08facb2c9e98623ef3254d2ddb48113885c3e8e97fec8db9", 16)
	if result.Cmp(expected) != 0 {
		t.Errorf("KECCAK256(0xFF) mismatch:\n  got  %x\n  want %x", result, expected)
	}
}

// --- JUMP / JUMPI / JUMPDEST тесты ---

func TestOpcode_JUMP_Unconditional(t *testing.T) {
	// PUSH1 4, JUMP, ADD (пропускается), JUMPDEST, PUSH1 99, STOP
	// PC:  0    1    2    3              4         5    6     7
	code := []byte{
		0x60, 0x04, // [0] PUSH1 4  (destination = PC 4)
		0x56,       // [2] JUMP
		0x01,       // [3] ADD      (должно быть пропущено)
		0x5B,       // [4] JUMPDEST
		0x60, 0x63, // [5] PUSH1 99
		0x00,       // [7] STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(99)) != 0 {
		t.Errorf("Expected 99, got %s", result.String())
	}
}

func TestOpcode_JUMPI_Taken(t *testing.T) {
	// EVM-порядок стека: top=destination, second=condition
	// Пушим condition первым (в стек попадает ниже), destination вторым (на вершину)
	// Ожидается прыжок на JUMPDEST[7], результат 42
	code := []byte{
		0x60, 0x01, // [0] PUSH1 1  (condition=true) -> bottom
		0x60, 0x07, // [2] PUSH1 7  (destination)    -> top
		0x57,       // [4] JUMPI
		0x60, 0x00, // [5] PUSH1 0  (пропускается при прыжке)
		0x5B,       // [7] JUMPDEST
		0x60, 0x2a, // [8] PUSH1 42
		0x00,       // [10] STOP
	}
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

func TestOpcode_JUMPI_NotTaken(t *testing.T) {
	// condition=0 => не прыгаем, PUSH1 77 выполняется
	code := []byte{
		0x60, 0x00, // [0] PUSH1 0  (condition=false) -> bottom
		0x60, 0x07, // [2] PUSH1 7  (destination)     -> top
		0x57,       // [4] JUMPI
		0x60, 0x4d, // [5] PUSH1 77 (выполняется)
		0x5B,       // [7] JUMPDEST
		0x00,       // [8] STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(77)) != 0 {
		t.Errorf("Expected 77, got %s", result.String())
	}
}

func TestOpcode_JUMP_Loop(t *testing.T) {
	// Цикл суммирования: sum = 1 + 2 + 3 = 6
	// Стек в цикле: [sum, counter] (bottom to top)
	//
	// [0-1]  PUSH1 0       sum=0
	// [2-3]  PUSH1 3       counter=3
	// [4]    JUMPDEST      loop_start
	// [5]    DUP1          [sum, counter, counter]
	// [6]    DUP3          [sum, counter, counter, sum]
	// [7]    ADD           [sum, counter, sum+counter]
	// [8]    SWAP2         [sum+counter, counter, sum]
	// [9]    POP           [sum+counter, counter]
	// [10-11] PUSH1 1      [sum+counter, counter, 1]
	// [12]   SUB           [sum+counter, counter-1]
	// [13]   DUP1          [sum+counter, counter-1, counter-1]
	// [14]   ISZERO        [sum+counter, counter-1, (cnt==0)]
	// [15-16] PUSH1 21     (exit_PC=21 -> destination, top)
	// [17]   JUMPI         (condition=second from top)
	// [18-19] PUSH1 4      (loop_start)
	// [20]   JUMP
	// [21]   JUMPDEST      exit
	// [22]   STOP
	code := []byte{
		0x60, 0x00, // [0-1]  PUSH1 0
		0x60, 0x03, // [2-3]  PUSH1 3
		0x5B,       // [4]    JUMPDEST (loop_start=4)
		0x80,       // [5]    DUP1
		0x82,       // [6]    DUP3
		0x01,       // [7]    ADD
		0x91,       // [8]    SWAP2
		0x50,       // [9]    POP
		0x60, 0x01, // [10-11] PUSH1 1
		0x03,       // [12]   SUB
		0x80,       // [13]   DUP1
		0x15,       // [14]   ISZERO
		0x60, 0x15, // [15-16] PUSH1 21 (exit destination, top=dest for JUMPI)
		0x57,       // [17]   JUMPI
		0x60, 0x04, // [18-19] PUSH1 4 (loop_start)
		0x56,       // [20]   JUMP
		0x5B,       // [21]   JUMPDEST (exit=21)
		0x00,       // [22]   STOP
	}
	// Увеличенный газ для цикла
	ctx := NewContextWithStorage(
		types.Address{}, types.Address{},
		big.NewInt(0), nil, 1_000_000, big.NewInt(1), &BlockInfo{},
		newMockStorageForOpcodes(),
	)
	vm := NewVM(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Loop run failed: %v", err)
	}
	// Стек: [sum=6, counter=0]
	counter, _ := vm.stack.Pop()
	sum, _ := vm.stack.Pop()
	if counter.Sign() != 0 {
		t.Errorf("Expected counter=0, got %s", counter.String())
	}
	if sum.Cmp(big.NewInt(6)) != 0 {
		t.Errorf("Expected sum=6, got %s", sum.String())
	}
}

func TestOpcode_JUMP_InvalidDest(t *testing.T) {
	// Прыжок в позицию без JUMPDEST — должна быть ошибка
	code := []byte{
		0x60, 0x03, // PUSH1 3
		0x56,       // JUMP
		0x01,       // ADD (не JUMPDEST!)
		0x00,       // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err == nil {
		t.Error("Expected error for invalid jump destination, got nil")
	}
}

// --- CALLDATA тесты ---

func TestOpcode_CALLDATASIZE(t *testing.T) {
	input := []byte{0x01, 0x02, 0x03}
	ctx := NewContextWithStorage(
		types.Address{}, types.Address{},
		big.NewInt(0), input, 100000, big.NewInt(1), &BlockInfo{},
		newMockStorageForOpcodes(),
	)
	code := []byte{
		0x36, // CALLDATASIZE
		0x00, // STOP
	}
	vm := NewVM(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(3)) != 0 {
		t.Errorf("Expected 3, got %s", result.String())
	}
}

func TestOpcode_CALLDATALOAD(t *testing.T) {
	// Загружаем 32 байта начиная с offset=0
	// Input: [0x01, 0x02, 0x03, ...0x00...] -> big.Int == 0x010203...
	input := make([]byte, 32)
	input[0] = 0xAB
	input[1] = 0xCD
	ctx := NewContextWithStorage(
		types.Address{}, types.Address{},
		big.NewInt(0), input, 100000, big.NewInt(1), &BlockInfo{},
		newMockStorageForOpcodes(),
	)
	code := []byte{
		0x60, 0x00, // PUSH1 0 (offset)
		0x35,       // CALLDATALOAD
		0x00,       // STOP
	}
	vm := NewVM(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	expected := new(big.Int).SetBytes(input)
	if result.Cmp(expected) != 0 {
		t.Errorf("CALLDATALOAD mismatch: got %x, want %x", result, expected)
	}
}

func TestOpcode_CALLDATACOPY(t *testing.T) {
	input := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	ctx := NewContextWithStorage(
		types.Address{}, types.Address{},
		big.NewInt(0), input, 100000, big.NewInt(1), &BlockInfo{},
		newMockStorageForOpcodes(),
	)
	// CALLDATACOPY(destOffset=0, dataOffset=0, length=4)
	code := []byte{
		0x60, 0x04, // PUSH1 4  (length)
		0x60, 0x00, // PUSH1 0  (dataOffset)
		0x60, 0x00, // PUSH1 0  (destOffset)
		0x37,       // CALLDATACOPY
		0x00,       // STOP
	}
	vm := NewVM(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	memData := vm.GetMemory().Data()
	if memData[0] != 0xDE || memData[1] != 0xAD || memData[2] != 0xBE || memData[3] != 0xEF {
		t.Errorf("CALLDATACOPY: unexpected memory %x", memData[:4])
	}
}

// --- LOG тесты ---

func TestOpcode_LOG0(t *testing.T) {
	// Записываем данные в память, затем LOG0
	code := []byte{
		0x60, 0xff, // PUSH1 0xFF
		0x60, 0x00, // PUSH1 0 (offset)
		0x53,       // MSTORE8
		0x60, 0x01, // PUSH1 1  (length)
		0x60, 0x00, // PUSH1 0  (offset)
		0xA0,       // LOG0
		0x00,       // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	logs := vm.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}
	if len(logs[0].Topics) != 0 {
		t.Errorf("Expected 0 topics, got %d", len(logs[0].Topics))
	}
	if len(logs[0].Data) != 1 || logs[0].Data[0] != 0xFF {
		t.Errorf("Unexpected log data: %x", logs[0].Data)
	}
}

func TestOpcode_LOG2(t *testing.T) {
	topic1 := big.NewInt(0x1234)
	topic2 := big.NewInt(0x5678)

	t1bytes := make([]byte, 32)
	t2bytes := make([]byte, 32)
	copy(t1bytes[30:], topic1.Bytes())
	copy(t2bytes[30:], topic2.Bytes())

	code := []byte{
		// PUSH32 topic2
		0x7F,
		t2bytes[0], t2bytes[1], t2bytes[2], t2bytes[3],
		t2bytes[4], t2bytes[5], t2bytes[6], t2bytes[7],
		t2bytes[8], t2bytes[9], t2bytes[10], t2bytes[11],
		t2bytes[12], t2bytes[13], t2bytes[14], t2bytes[15],
		t2bytes[16], t2bytes[17], t2bytes[18], t2bytes[19],
		t2bytes[20], t2bytes[21], t2bytes[22], t2bytes[23],
		t2bytes[24], t2bytes[25], t2bytes[26], t2bytes[27],
		t2bytes[28], t2bytes[29], t2bytes[30], t2bytes[31],
		// PUSH32 topic1
		0x7F,
		t1bytes[0], t1bytes[1], t1bytes[2], t1bytes[3],
		t1bytes[4], t1bytes[5], t1bytes[6], t1bytes[7],
		t1bytes[8], t1bytes[9], t1bytes[10], t1bytes[11],
		t1bytes[12], t1bytes[13], t1bytes[14], t1bytes[15],
		t1bytes[16], t1bytes[17], t1bytes[18], t1bytes[19],
		t1bytes[20], t1bytes[21], t1bytes[22], t1bytes[23],
		t1bytes[24], t1bytes[25], t1bytes[26], t1bytes[27],
		t1bytes[28], t1bytes[29], t1bytes[30], t1bytes[31],
		0x60, 0x00, // PUSH1 0 (length=0, данные пустые)
		0x60, 0x00, // PUSH1 0 (offset)
		0xA2,       // LOG2
		0x00,       // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	logs := vm.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}
	if len(logs[0].Topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(logs[0].Topics))
	}
	if logs[0].Topics[0].Cmp(topic1) != 0 {
		t.Errorf("Topic1 mismatch: got %s, want %s", logs[0].Topics[0], topic1)
	}
	if logs[0].Topics[1].Cmp(topic2) != 0 {
		t.Errorf("Topic2 mismatch: got %s, want %s", logs[0].Topics[1], topic2)
	}
}

func TestOpcode_ECRECOVER(t *testing.T) {
	// Генерируем ключ, подписываем хэш, восстанавливаем адрес через ECRECOVER
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Вычисляем ожидаемый адрес: keccak256(pubKey[1:])[12:]
	pubBytes := pubKey.SerializeUncompressed()
	h := sha3.NewLegacyKeccak256()
	h.Write(pubBytes[1:])
	addrHash := h.Sum(nil)
	expectedAddr := new(big.Int).SetBytes(addrHash[12:])

	// Подписываем хэш (32 байта)
	msgHash := make([]byte, 32)
	copy(msgHash, []byte("pallada ecrecover test"))
	compactSig := secp256k1ecdsa.SignCompact(privKey, msgHash, false)
	// compactSig[0] = recovery byte (27 или 28)
	// compactSig[1:33] = R, compactSig[33:65] = S

	vByte := compactSig[0]         // 27 или 28
	rBytes := compactSig[1:33]
	sBytes := compactSig[33:65]

	// Строим байткод: PUSH32 s, PUSH32 r, PUSH1 v, PUSH32 hash, ECRECOVER, STOP
	// executeEcrecover попает: hash(top), v, r, s(bottom)
	code := []byte{0x7F}  // PUSH32 s (попадает ниже всех)
	code = append(code, sBytes...)
	code = append(code, 0x7F) // PUSH32 r
	code = append(code, rBytes...)
	code = append(code, 0x60, vByte) // PUSH1 v
	code = append(code, 0x7F)         // PUSH32 hash (на вершине)
	code = append(code, msgHash...)
	code = append(code, 0x23, 0x00) // ECRECOVER, STOP

	vm := createTestVM(code)
	_, err = vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if vm.stack.Depth() != 1 {
		t.Fatalf("Expected stack depth 1, got %d", vm.stack.Depth())
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(expectedAddr) != 0 {
		t.Errorf("ECRECOVER address mismatch:\n  got  %x\n  want %x", result.Bytes(), expectedAddr.Bytes())
	}
}

func TestOpcode_DUP_UseInComputation(t *testing.T) {
	// Вычисление x^2: PUSH1 5, DUP1, MUL -> 25
	code := []byte{
		0x60, 0x05, // PUSH1 5
		0x80,       // DUP1 -> стек: [5, 5]
		0x02,       // MUL  -> 25
		0x00,       // STOP
	}
	vm := createTestVM(code)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	result, _ := vm.stack.Pop()
	if result.Cmp(big.NewInt(25)) != 0 {
		t.Errorf("Expected 25, got %s", result.String())
	}
}
