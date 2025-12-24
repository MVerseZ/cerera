package pallada

import (
	"errors"
	"math/big"
)

var (
	// ErrInvalidOpcode возникает при встрече неизвестного опкода
	ErrInvalidOpcode = errors.New("invalid opcode")
	// ErrStackUnderflowOpcode возникает когда опкоду не хватает элементов в стеке
	ErrStackUnderflowOpcode = errors.New("stack underflow for opcode")
)

// Opcode представляет код операции виртуальной машины
type Opcode byte

// Базовые опкоды (0x00-0x0F)
const (
	STOP       Opcode = 0x00 // Остановка выполнения
	ADD        Opcode = 0x01 // Сложение
	MUL        Opcode = 0x02 // Умножение
	SUB        Opcode = 0x03 // Вычитание
	DIV        Opcode = 0x04 // Деление
	SDIV       Opcode = 0x05 // Знаковое деление
	MOD        Opcode = 0x06 // Остаток от деления
	SMOD       Opcode = 0x07 // Знаковый остаток
	ADDMOD     Opcode = 0x08 // (a + b) mod n
	MULMOD     Opcode = 0x09 // (a * b) mod n
	EXP        Opcode = 0x0A // Возведение в степень
	SIGNEXTEND Opcode = 0x0B // Расширение знака
)

// Опкоды сравнения (0x10-0x1F)
const (
	LT     Opcode = 0x10 // Меньше
	GT     Opcode = 0x11 // Больше
	SLT    Opcode = 0x12 // Знаковое меньше
	SGT    Opcode = 0x13 // Знаковое больше
	EQ     Opcode = 0x14 // Равно
	ISZERO Opcode = 0x15 // Проверка на ноль
	AND    Opcode = 0x16 // Побитовое И
	OR     Opcode = 0x17 // Побитовое ИЛИ
	XOR    Opcode = 0x18 // Побитовое исключающее ИЛИ
	NOT    Opcode = 0x19 // Побитовое НЕ
	BYTE   Opcode = 0x1A // Извлечение байта
	SHL    Opcode = 0x1B // Логический сдвиг влево
	SHR    Opcode = 0x1C // Логический сдвиг вправо
	SAR    Opcode = 0x1D // Арифметический сдвиг вправо
)

// Опкоды стека (0x50-0x5F)
const (
	POP     Opcode = 0x50 // Удалить элемент со стека
	MLOAD   Opcode = 0x51 // Загрузить из памяти
	MSTORE  Opcode = 0x52 // Сохранить в память
	MSTORE8 Opcode = 0x53 // Сохранить байт в память
	SLOAD   Opcode = 0x54 // Загрузить из storage
	SSTORE  Opcode = 0x55 // Сохранить в storage
	MSIZE   Opcode = 0x56 // Размер памяти
	PUSH1   Opcode = 0x60 // Push 1 байт
	PUSH32  Opcode = 0x7F // Push 32 байта (максимум)
)

// Опкоды управления потоком
const (
	CALL   Opcode = 0xF1 // Межконтрактный вызов
	RETURN Opcode = 0xF3 // Возврат данных
	REVERT Opcode = 0xFD // Откат с данными
)

// OpcodeHandler представляет обработчик опкода
// Принцип: Strategy Pattern - каждый опкод имеет свою стратегию выполнения
type OpcodeHandler func(vm *VM) error

// executeArithmetic выполняет арифметические операции
func executeArithmetic(vm *VM, op Opcode) error {
	var result *big.Int

	// ADDMOD и MULMOD требуют три операнда
	if op == ADDMOD || op == MULMOD {
		// Порядок на стеке (снизу вверх): a, b, n
		// Pop() извлекает сверху: сначала n, потом b, потом a
		n, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		b, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		a, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		switch op {
		case ADDMOD:
			// (a + b) mod n
			if n.Sign() == 0 {
				result = big.NewInt(0)
			} else {
				sum := new(big.Int).Add(a, b)
				result = new(big.Int).Mod(sum, n)
			}
		case MULMOD:
			// (a * b) mod n
			if n.Sign() == 0 {
				result = big.NewInt(0)
			} else {
				prod := new(big.Int).Mul(a, b)
				result = new(big.Int).Mod(prod, n)
			}
		}
	} else {
		// Остальные операции требуют два операнда
		b, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		a, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		switch op {
		case ADD:
			result = new(big.Int).Add(a, b)
		case MUL:
			result = new(big.Int).Mul(a, b)
		case SUB:
			result = new(big.Int).Sub(a, b)
		case DIV:
			if b.Sign() == 0 {
				result = big.NewInt(0) // Деление на ноль возвращает 0
			} else {
				result = new(big.Int).Div(a, b)
			}
		case SDIV:
			if b.Sign() == 0 {
				result = big.NewInt(0)
			} else {
				// Знаковое деление
				aSign := a.Sign()
				bSign := b.Sign()
				absA := new(big.Int).Abs(a)
				absB := new(big.Int).Abs(b)
				result = new(big.Int).Div(absA, absB)
				if aSign != bSign {
					result.Neg(result)
				}
			}
		case MOD:
			if b.Sign() == 0 {
				result = big.NewInt(0)
			} else {
				result = new(big.Int).Mod(a, b)
			}
		case SMOD:
			if b.Sign() == 0 {
				result = big.NewInt(0)
			} else {
				// Знаковый остаток
				aSign := a.Sign()
				absA := new(big.Int).Abs(a)
				absB := new(big.Int).Abs(b)
				result = new(big.Int).Mod(absA, absB)
				if aSign < 0 {
					result.Neg(result)
				}
			}
		case EXP:
			// a^b (только для небольших степеней, иначе очень дорого)
			expBitLen := b.BitLen()
			if expBitLen > 256 {
				return errors.New("exponent too large")
			}
			result = new(big.Int).Exp(a, b, nil)
			// Вычисляем газ для EXP (используем длину экспоненты)
			gasCost := GasMid + uint64(expBitLen)*GasExtByte
			if err := vm.gasMeter.ConsumeGas(gasCost, "EXP"); err != nil {
				return err
			}
			return vm.stack.Push(result)
		default:
			return ErrInvalidOpcode
		}
	}

	// Потребляем газ (EXP уже обработан выше)
	if op != EXP {
		gasCost := GasVeryLow
		if op == ADDMOD || op == MULMOD {
			gasCost = GasMid
		}

		if err := vm.gasMeter.ConsumeGas(gasCost, op.String()); err != nil {
			return err
		}
	}

	// Помещаем результат на стек
	return vm.stack.Push(result)
}

// executeComparison выполняет операции сравнения
func executeComparison(vm *VM, op Opcode) error {
	b, err := vm.stack.Pop()
	if err != nil {
		return ErrStackUnderflowOpcode
	}
	a, err := vm.stack.Pop()
	if err != nil {
		return ErrStackUnderflowOpcode
	}

	var result *big.Int

	switch op {
	case LT:
		if a.Cmp(b) < 0 {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	case GT:
		if a.Cmp(b) > 0 {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	case SLT:
		// Знаковое сравнение
		if (a.Sign() < 0 && b.Sign() >= 0) ||
			(a.Sign() < 0 && b.Sign() < 0 && a.Cmp(b) > 0) ||
			(a.Sign() >= 0 && b.Sign() >= 0 && a.Cmp(b) < 0) {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	case SGT:
		// Знаковое сравнение (больше)
		if (a.Sign() >= 0 && b.Sign() < 0) ||
			(a.Sign() < 0 && b.Sign() < 0 && a.Cmp(b) < 0) ||
			(a.Sign() >= 0 && b.Sign() >= 0 && a.Cmp(b) > 0) {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	case EQ:
		if a.Cmp(b) == 0 {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	default:
		return ErrInvalidOpcode
	}

	// Потребляем газ
	if err := vm.gasMeter.ConsumeGas(GasVeryLow, op.String()); err != nil {
		return err
	}

	return vm.stack.Push(result)
}

// executeBitwise выполняет побитовые операции
func executeBitwise(vm *VM, op Opcode) error {
	var result *big.Int

	switch op {
	case ISZERO:
		// Проверка на ноль (один операнд)
		a, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		if a.Sign() == 0 {
			result = big.NewInt(1)
		} else {
			result = big.NewInt(0)
		}
	case AND, OR, XOR:
		// Два операнда
		b, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		a, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		switch op {
		case AND:
			result = new(big.Int).And(a, b)
		case OR:
			result = new(big.Int).Or(a, b)
		case XOR:
			result = new(big.Int).Xor(a, b)
		}
	case NOT:
		// Один операнд
		a, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		// Побитовое НЕ для 256-битного числа
		allOnes := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
		result = new(big.Int).Xor(a, allOnes)
	case BYTE:
		// Извлечение i-го байта из value
		i, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		byteIdx := i.Uint64()
		if byteIdx >= 32 {
			result = big.NewInt(0)
		} else {
			bytes := value.Bytes()
			// big.Int.Bytes() возвращает минимальное представление, нужно дополнить до 32 байт
			fullBytes := make([]byte, 32)
			start := 32 - len(bytes)
			copy(fullBytes[start:], bytes)
			result = big.NewInt(int64(fullBytes[byteIdx]))
		}
	case SHL:
		// Логический сдвиг влево: value << shift
		shift, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		if shift.Uint64() > 256 {
			result = big.NewInt(0)
		} else {
			result = new(big.Int).Lsh(value, uint(shift.Uint64()))
		}
	case SHR:
		// Логический сдвиг вправо: value >> shift
		shift, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		if shift.Uint64() > 256 {
			result = big.NewInt(0)
		} else {
			result = new(big.Int).Rsh(value, uint(shift.Uint64()))
		}
	case SAR:
		// Арифметический сдвиг вправо (сохраняет знак)
		shift, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		if shift.Uint64() > 256 {
			if value.Sign() < 0 {
				result = new(big.Int).SetInt64(-1) // Все единицы
			} else {
				result = big.NewInt(0)
			}
		} else {
			// Арифметический сдвиг: для отрицательных чисел заполняет единицами
			if value.Sign() < 0 {
				absValue := new(big.Int).Abs(value)
				shifted := new(big.Int).Rsh(absValue, uint(shift.Uint64()))
				// Восстанавливаем знак и заполняем старшие биты единицами
				allOnes := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256-uint(shift.Uint64())), big.NewInt(1))
				allOnes.Lsh(allOnes, uint(shift.Uint64()))
				result = new(big.Int).Or(shifted, allOnes)
				result.Neg(result)
			} else {
				result = new(big.Int).Rsh(value, uint(shift.Uint64()))
			}
		}
	default:
		return ErrInvalidOpcode
	}

	// Потребляем газ
	if err := vm.gasMeter.ConsumeGas(GasVeryLow, op.String()); err != nil {
		return err
	}

	return vm.stack.Push(result)
}

// String возвращает строковое представление опкода
func (op Opcode) String() string {
	switch op {
	case STOP:
		return "STOP"
	case ADD:
		return "ADD"
	case MUL:
		return "MUL"
	case SUB:
		return "SUB"
	case DIV:
		return "DIV"
	case MOD:
		return "MOD"
	case LT:
		return "LT"
	case GT:
		return "GT"
	case EQ:
		return "EQ"
	case ISZERO:
		return "ISZERO"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case XOR:
		return "XOR"
	case NOT:
		return "NOT"
	case POP:
		return "POP"
	case MLOAD:
		return "MLOAD"
	case MSTORE:
		return "MSTORE"
	case SLOAD:
		return "SLOAD"
	case SSTORE:
		return "SSTORE"
	case CALL:
		return "CALL"
	case RETURN:
		return "RETURN"
	case REVERT:
		return "REVERT"
	default:
		return "UNKNOWN"
	}
}
