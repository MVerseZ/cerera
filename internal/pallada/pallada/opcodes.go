package pallada

// OpCode представляет код операции (инструкцию)
type OpCode byte

// Константы опкодов
const (
	// Остановка выполнения
	STOP OpCode = 0x00

	// Арифметические операции
	ADD OpCode = 0x01 // Сложение: a + b
	SUB OpCode = 0x02 // Вычитание: a - b
	MUL OpCode = 0x03 // Умножение: a * b
	DIV OpCode = 0x04 // Деление: a / b
	MOD OpCode = 0x05 // Остаток от деления: a % b

	// Логические операции
	AND OpCode = 0x10 // Побитовое И: a & b
	OR  OpCode = 0x11 // Побитовое ИЛИ: a | b
	XOR OpCode = 0x12 // Побитовое исключающее ИЛИ: a ^ b
	NOT OpCode = 0x13 // Побитовое НЕ: ~a

	// Операции сравнения
	LT     OpCode = 0x20 // Меньше: a < b (возвращает 1 или 0)
	GT     OpCode = 0x21 // Больше: a > b (возвращает 1 или 0)
	EQ     OpCode = 0x22 // Равно: a == b (возвращает 1 или 0)
	ISZERO OpCode = 0x23 // Ноль: a == 0 (возвращает 1 или 0)

	// Операции со стеком
	POP OpCode = 0x30 // Удалить верхний элемент стека

	// DUP операции (дублирование)
	DUP1 OpCode = 0x40 // Дублировать 1-й элемент с верха
	DUP2 OpCode = 0x41 // Дублировать 2-й элемент с верха
	DUP3 OpCode = 0x42 // Дублировать 3-й элемент с верха
	DUP4 OpCode = 0x43 // Дублировать 4-й элемент с верха

	// SWAP операции (обмен)
	SWAP1 OpCode = 0x50 // Поменять местами 1-й и 2-й элементы
	SWAP2 OpCode = 0x51 // Поменять местами 1-й и 3-й элементы
	SWAP3 OpCode = 0x52 // Поменять местами 1-й и 4-й элементы
	SWAP4 OpCode = 0x53 // Поменять местами 1-й и 5-й элементы

	// Операции с памятью
	MLOAD  OpCode = 0x60 // Загрузить слово из памяти: stack[offset] -> stack
	MSTORE OpCode = 0x61 // Сохранить слово в память: stack[value], stack[offset] -> memory[offset]

	// PUSH операции (загрузка констант)
	PUSH1  OpCode = 0x70 // Загрузить 1 байт на стек
	PUSH2  OpCode = 0x71 // Загрузить 2 байта на стек
	PUSH4  OpCode = 0x72 // Загрузить 4 байта на стек
	PUSH8  OpCode = 0x73 // Загрузить 8 байт на стек
	PUSH16 OpCode = 0x74 // Загрузить 16 байт на стек
	PUSH32 OpCode = 0x75 // Загрузить 32 байта на стек (полное слово)

	// Управление потоком
	JUMP     OpCode = 0x80 // Безусловный переход: stack[dest] -> pc
	JUMPI    OpCode = 0x81 // Условный переход: stack[cond], stack[dest] -> pc (если cond != 0)
	JUMPDEST OpCode = 0x82 // Метка для перехода (NOP)
	PC       OpCode = 0x83 // Получить текущий счетчик команд: -> stack[pc]
	RETURN   OpCode = 0x84 // Вернуть данные: stack[offset], stack[size] -> return data
	REVERT   OpCode = 0x85 // Откатить выполнение: stack[offset], stack[size] -> error

	// Блокчейн-опкоды (контекст выполнения)
	ADDRESS      OpCode = 0x90 // Получить адрес контракта: -> stack[address]
	CALLER       OpCode = 0x91 // Получить адрес вызывающего: -> stack[caller]
	CALLVALUE    OpCode = 0x92 // Получить значение транзакции: -> stack[value]
	CALLDATALOAD OpCode = 0x93 // Загрузить данные вызова: stack[offset] -> stack[data]
	CALLDATASIZE OpCode = 0x94 // Размер данных вызова: -> stack[size]
	CALLDATACOPY OpCode = 0x95 // Копировать данные вызова: stack[destOffset], stack[offset], stack[size] -> memory

	// Storage опкоды
	SLOAD  OpCode = 0x96 // Загрузить из storage: stack[key] -> stack[value]
	SSTORE OpCode = 0x97 // Сохранить в storage: stack[key], stack[value] -> storage
)

// GetPushSize возвращает размер данных для PUSH операции
func GetPushSize(op OpCode) (int, bool) {
	switch op {
	case PUSH1:
		return 1, true
	case PUSH2:
		return 2, true
	case PUSH4:
		return 4, true
	case PUSH8:
		return 8, true
	case PUSH16:
		return 16, true
	case PUSH32:
		return 32, true
	default:
		return 0, false
	}
}

// IsPush проверяет, является ли опкод PUSH операцией
func IsPush(op OpCode) bool {
	_, ok := GetPushSize(op)
	return ok
}

// IsValidOpcode проверяет, является ли опкод валидным
func IsValidOpcode(op OpCode) bool {
	switch op {
	case STOP,
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
		SLOAD, SSTORE:
		return true
	default:
		return false
	}
}

// OpcodeName возвращает имя опкода для отладки
func OpcodeName(op OpCode) string {
	switch op {
	case STOP:
		return "STOP"
	case ADD:
		return "ADD"
	case SUB:
		return "SUB"
	case MUL:
		return "MUL"
	case DIV:
		return "DIV"
	case MOD:
		return "MOD"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case XOR:
		return "XOR"
	case NOT:
		return "NOT"
	case LT:
		return "LT"
	case GT:
		return "GT"
	case EQ:
		return "EQ"
	case ISZERO:
		return "ISZERO"
	case POP:
		return "POP"
	case DUP1:
		return "DUP1"
	case DUP2:
		return "DUP2"
	case DUP3:
		return "DUP3"
	case DUP4:
		return "DUP4"
	case SWAP1:
		return "SWAP1"
	case SWAP2:
		return "SWAP2"
	case SWAP3:
		return "SWAP3"
	case SWAP4:
		return "SWAP4"
	case MLOAD:
		return "MLOAD"
	case MSTORE:
		return "MSTORE"
	case PUSH1:
		return "PUSH1"
	case PUSH2:
		return "PUSH2"
	case PUSH4:
		return "PUSH4"
	case PUSH8:
		return "PUSH8"
	case PUSH16:
		return "PUSH16"
	case PUSH32:
		return "PUSH32"
	case JUMP:
		return "JUMP"
	case JUMPI:
		return "JUMPI"
	case JUMPDEST:
		return "JUMPDEST"
	case PC:
		return "PC"
	case RETURN:
		return "RETURN"
	case REVERT:
		return "REVERT"
	case ADDRESS:
		return "ADDRESS"
	case CALLER:
		return "CALLER"
	case CALLVALUE:
		return "CALLVALUE"
	case CALLDATALOAD:
		return "CALLDATALOAD"
	case CALLDATASIZE:
		return "CALLDATASIZE"
	case CALLDATACOPY:
		return "CALLDATACOPY"
	case SLOAD:
		return "SLOAD"
	case SSTORE:
		return "SSTORE"
	default:
		return "UNKNOWN"
	}
}
