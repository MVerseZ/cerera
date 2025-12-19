package pallada

import "testing"

func TestIsValidOpcode(t *testing.T) {
	// Тестируем валидные опкоды
	validOps := []OpCode{
		STOP,
		ADD, SUB, MUL, DIV, MOD,
		AND, OR, XOR, NOT,
		LT, GT, EQ, ISZERO,
		POP,
		DUP1, DUP2, DUP3, DUP4,
		SWAP1, SWAP2, SWAP3, SWAP4,
		MLOAD, MSTORE,
		PUSH1, PUSH2, PUSH4, PUSH8, PUSH16, PUSH32,
		JUMP, JUMPI, JUMPDEST, PC, RETURN, REVERT,
		ADDRESS, CALLER, CALLVALUE, CALLDATALOAD, CALLDATASIZE, CALLDATACOPY,
		SLOAD, SSTORE, CALL,
	}

	for _, op := range validOps {
		if !IsValidOpcode(op) {
			t.Errorf("Expected %s (0x%02x) to be valid", OpcodeName(op), op)
		}
	}

	// Тестируем невалидные опкоды
	invalidOps := []OpCode{
		0xFF, 0xFE, 0xFD, 0x99, 0xAA, // 0x90-0x98 теперь валидны (ADDRESS-CALL)
	}

	for _, op := range invalidOps {
		if IsValidOpcode(op) {
			t.Errorf("Expected %s (0x%02x) to be invalid", OpcodeName(op), op)
		}
	}
}

func TestIsPush(t *testing.T) {
	// Тестируем PUSH опкоды
	pushOps := []OpCode{PUSH1, PUSH2, PUSH4, PUSH8, PUSH16, PUSH32}

	for _, op := range pushOps {
		if !IsPush(op) {
			t.Errorf("Expected %s (0x%02x) to be a PUSH operation", OpcodeName(op), op)
		}
	}

	// Тестируем не-PUSH опкоды
	nonPushOps := []OpCode{STOP, ADD, SUB, MUL, JUMP, JUMPI}

	for _, op := range nonPushOps {
		if IsPush(op) {
			t.Errorf("Expected %s (0x%02x) to NOT be a PUSH operation", OpcodeName(op), op)
		}
	}
}

func TestGetPushSize(t *testing.T) {
	tests := []struct {
		op       OpCode
		expected int
		valid    bool
	}{
		{PUSH1, 1, true},
		{PUSH2, 2, true},
		{PUSH4, 4, true},
		{PUSH8, 8, true},
		{PUSH16, 16, true},
		{PUSH32, 32, true},
		{STOP, 0, false},
		{ADD, 0, false},
		{JUMP, 0, false},
	}

	for _, tt := range tests {
		size, ok := GetPushSize(tt.op)
		if ok != tt.valid {
			t.Errorf("GetPushSize(%s): expected valid=%v, got valid=%v", OpcodeName(tt.op), tt.valid, ok)
		}
		if ok && size != tt.expected {
			t.Errorf("GetPushSize(%s): expected size=%d, got size=%d", OpcodeName(tt.op), tt.expected, size)
		}
	}
}

func TestOpcodeName(t *testing.T) {
	tests := []struct {
		op       OpCode
		expected string
	}{
		{STOP, "STOP"},
		{ADD, "ADD"},
		{SUB, "SUB"},
		{MUL, "MUL"},
		{DIV, "DIV"},
		{MOD, "MOD"},
		{AND, "AND"},
		{OR, "OR"},
		{XOR, "XOR"},
		{NOT, "NOT"},
		{LT, "LT"},
		{GT, "GT"},
		{EQ, "EQ"},
		{ISZERO, "ISZERO"},
		{POP, "POP"},
		{DUP1, "DUP1"},
		{DUP2, "DUP2"},
		{DUP3, "DUP3"},
		{DUP4, "DUP4"},
		{SWAP1, "SWAP1"},
		{SWAP2, "SWAP2"},
		{SWAP3, "SWAP3"},
		{SWAP4, "SWAP4"},
		{MLOAD, "MLOAD"},
		{MSTORE, "MSTORE"},
		{PUSH1, "PUSH1"},
		{PUSH2, "PUSH2"},
		{PUSH4, "PUSH4"},
		{PUSH8, "PUSH8"},
		{PUSH16, "PUSH16"},
		{PUSH32, "PUSH32"},
		{JUMP, "JUMP"},
		{JUMPI, "JUMPI"},
		{JUMPDEST, "JUMPDEST"},
		{PC, "PC"},
		{RETURN, "RETURN"},
		{REVERT, "REVERT"},
		{ADDRESS, "ADDRESS"},
		{CALLER, "CALLER"},
		{CALLVALUE, "CALLVALUE"},
		{CALLDATALOAD, "CALLDATALOAD"},
		{CALLDATASIZE, "CALLDATASIZE"},
		{CALLDATACOPY, "CALLDATACOPY"},
		{SLOAD, "SLOAD"},
		{SSTORE, "SSTORE"},
		{CALL, "CALL"},
		{0xFF, "UNKNOWN"},
		{0xAA, "UNKNOWN"},
	}

	for _, tt := range tests {
		name := OpcodeName(tt.op)
		if name != tt.expected {
			t.Errorf("OpcodeName(0x%02x): expected %s, got %s", tt.op, tt.expected, name)
		}
	}
}

func TestOpcodeConstants(t *testing.T) {
	// Проверяем, что все константы имеют правильные значения
	if STOP != 0x00 {
		t.Errorf("STOP: expected 0x00, got 0x%02x", STOP)
	}

	if ADD != 0x01 {
		t.Errorf("ADD: expected 0x01, got 0x%02x", ADD)
	}

	if PUSH1 != 0x70 {
		t.Errorf("PUSH1: expected 0x70, got 0x%02x", PUSH1)
	}

	if JUMP != 0x80 {
		t.Errorf("JUMP: expected 0x80, got 0x%02x", JUMP)
	}
}
