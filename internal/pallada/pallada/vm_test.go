package pallada

import (
	"math/big"
	"testing"
)

func TestVM_SimpleArithmetic(t *testing.T) {
	// Байткод: PUSH1 5, PUSH1 3, ADD, STOP
	code := []byte{
		byte(PUSH1), 0x05, // PUSH1 5
		byte(PUSH1), 0x03, // PUSH1 3
		byte(ADD),  // ADD
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	result, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем результат на стеке
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("Expected 8, got %v", stackValue)
	}

	_ = result
}

func TestVM_Subtract(t *testing.T) {
	// Байткод: PUSH1 10, PUSH1 3, SUB, STOP
	code := []byte{
		byte(PUSH1), 0x0a, // PUSH1 10
		byte(PUSH1), 0x03, // PUSH1 3
		byte(SUB),  // SUB
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("Expected 7, got %v", stackValue)
	}
}

func TestVM_Multiply(t *testing.T) {
	// Байткод: PUSH1 4, PUSH1 5, MUL, STOP
	code := []byte{
		byte(PUSH1), 0x04, // PUSH1 4
		byte(PUSH1), 0x05, // PUSH1 5
		byte(MUL),  // MUL
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("Expected 20, got %v", stackValue)
	}
}

func TestVM_Divide(t *testing.T) {
	// Байткод: PUSH1 20, PUSH1 4, DIV, STOP
	code := []byte{
		byte(PUSH1), 0x14, // PUSH1 20
		byte(PUSH1), 0x04, // PUSH1 4
		byte(DIV),  // DIV
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("Expected 5, got %v", stackValue)
	}
}

func TestVM_DivideByZero(t *testing.T) {
	// Байткод: PUSH1 10, PUSH1 0, DIV, STOP
	code := []byte{
		byte(PUSH1), 0x0a, // PUSH1 10
		byte(PUSH1), 0x00, // PUSH1 0
		byte(DIV),  // DIV
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Деление на ноль должно вернуть 0
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Sign() != 0 {
		t.Fatalf("Expected 0, got %v", stackValue)
	}
}

func TestVM_Comparison(t *testing.T) {
	// Байткод: PUSH1 3, PUSH1 5, LT, STOP
	code := []byte{
		byte(PUSH1), 0x03, // PUSH1 3
		byte(PUSH1), 0x05, // PUSH1 5
		byte(LT),   // LT (проверяет 3 < 5 = true)
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	// 3 < 5 должно быть true (1)
	if stackValue.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("Expected 1 (true), got %v", stackValue)
	}
}

func TestVM_LogicalOperations(t *testing.T) {
	// Байткод: PUSH1 0xFF, PUSH1 0x0F, AND, STOP
	code := []byte{
		byte(PUSH1), 0xff, // PUSH1 0xFF
		byte(PUSH1), 0x0f, // PUSH1 0x0F
		byte(AND),  // AND
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	// 0xFF & 0x0F = 0x0F
	if stackValue.Cmp(big.NewInt(0x0F)) != 0 {
		t.Fatalf("Expected 0x0F, got %v", stackValue)
	}
}

func TestVM_MemoryOperations(t *testing.T) {
	// Байткод: PUSH1 0x42, PUSH1 0, MSTORE, PUSH1 0, MLOAD, STOP
	code := []byte{
		byte(PUSH1), 0x42, // PUSH1 0x42
		byte(PUSH1), 0x00, // PUSH1 0 (offset)
		byte(MSTORE),      // MSTORE
		byte(PUSH1), 0x00, // PUSH1 0 (offset)
		byte(MLOAD), // MLOAD
		byte(STOP),  // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что значение загружено обратно
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	// Значение должно быть 0x42, но расширенное до 32 байт
	expected := big.NewInt(0x42)
	if stackValue.Cmp(expected) != 0 {
		t.Fatalf("Expected %v, got %v", expected, stackValue)
	}
}

func TestVM_Return(t *testing.T) {
	// Байткод: PUSH1 4, PUSH1 0, MSTORE, PUSH1 4, PUSH1 0, RETURN
	code := []byte{
		byte(PUSH1), 0x04, // PUSH1 4 (size)
		byte(PUSH1), 0x00, // PUSH1 0 (offset)
		byte(MSTORE),      // MSTORE
		byte(PUSH1), 0x04, // PUSH1 4 (size)
		byte(PUSH1), 0x00, // PUSH1 0 (offset)
		byte(RETURN), // RETURN
	}

	vm := NewVM(code)
	result, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("Expected result length 4, got %d", len(result))
	}
}

func TestVM_InvalidOpcode(t *testing.T) {
	// Байткод с невалидным опкодом
	code := []byte{
		byte(PUSH1), 0x05,
		0xFE, // Невалидный опкод
		byte(STOP),
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err == nil {
		t.Fatal("Expected error for invalid opcode")
	}
}

func TestVM_StackOperations(t *testing.T) {
	// Байткод: PUSH1 10, PUSH1 20, DUP1, STOP
	code := []byte{
		byte(PUSH1), 0x0a, // PUSH1 10
		byte(PUSH1), 0x14, // PUSH1 20
		byte(DUP1), // DUP1 (дублирует 20)
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Стек должен быть: [10, 20, 20]
	if vm.stack.Len() != 3 {
		t.Fatalf("Expected stack length 3, got %d", vm.stack.Len())
	}

	top, _ := vm.stack.Pop()
	if top.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("Expected 20, got %v", top)
	}
}

func TestVM_Jump(t *testing.T) {
	// Байткод: PUSH1 4, JUMP, STOP, JUMPDEST, PUSH1 42, STOP
	// Позиции: 0-1 PUSH1 4, 2 JUMP, 3 STOP, 4 JUMPDEST, 5-6 PUSH1 42, 7 STOP
	code := []byte{
		byte(PUSH1), 0x04, // PUSH1 4 (адрес JUMPDEST)
		byte(JUMP),        // JUMP (использует значение со стека)
		byte(STOP),        // STOP (не должно выполниться)
		byte(JUMPDEST),    // JUMPDEST (позиция 4)
		byte(PUSH1), 0x2a, // PUSH1 42
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// После JUMP стек пуст, но PUSH1 42 должен добавить значение
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("Expected 42, got %v", stackValue)
	}
}

func TestVM_Jumpi(t *testing.T) {
	// Байткод: PUSH1 8, PUSH1 1, JUMPI, PUSH1 0, STOP, JUMPDEST, PUSH1 42, STOP
	// Позиции: 0-1 PUSH1 8, 2-3 PUSH1 1, 4 JUMPI, 5-6 PUSH1 0, 7 STOP, 8 JUMPDEST, 9-10 PUSH1 42, 11 STOP
	code := []byte{
		byte(PUSH1), 0x08, // PUSH1 8 (адрес JUMPDEST)
		byte(PUSH1), 0x01, // PUSH1 1 (условие)
		byte(JUMPI),       // JUMPI (использует оба значения со стека)
		byte(PUSH1), 0x00, // PUSH1 0 (не должно выполниться)
		byte(STOP),        // STOP
		byte(JUMPDEST),    // JUMPDEST (позиция 8)
		byte(PUSH1), 0x2a, // PUSH1 42
		byte(STOP), // STOP
	}

	vm := NewVM(code)
	_, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// После JUMPI стек пуст, но PUSH1 42 должен добавить значение
	stackValue, err := vm.stack.Pop()
	if err != nil {
		t.Fatalf("Stack should contain result: %v", err)
	}

	if stackValue.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("Expected 42, got %v", stackValue)
	}
}

func TestVM_EmptyCode(t *testing.T) {
	code := []byte{}

	vm := NewVM(code)
	result, err := vm.Run()

	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if result != nil {
		t.Fatalf("Expected nil result, got %v", result)
	}
}
