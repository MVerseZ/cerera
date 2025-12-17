package pallada

import (
	"errors"
	"fmt"
)

// Ошибки VM
var (
	ErrInvalidOpcode         = errors.New("invalid opcode")
	ErrInvalidJump           = errors.New("invalid jump destination")
	ErrInvalidBytecode       = errors.New("invalid bytecode format")
	ErrReturnDataOutOfBounds = errors.New("return data out of bounds")
)

// VM представляет виртуальную машину
type VM struct {
	stack      *Stack
	memory     *Memory
	code       []byte // Байткод для выполнения
	pc         int    // Program counter (счетчик команд)
	returnData []byte // Данные для возврата
	stopped    bool   // Флаг остановки выполнения
	err        error  // Ошибка выполнения

	// Контекст блокчейна
	ctx *Context // Контекст выполнения (может быть nil для простых операций)

	// Система газа
	gas *GasMeter // Счетчик газа (может быть nil, если газ не используется)
}

// NewVM создает новую виртуальную машину
func NewVM(code []byte) *VM {
	return &VM{
		stack:      NewStack(),
		memory:     NewMemory(),
		code:       code,
		pc:         0,
		returnData: nil,
		stopped:    false,
		err:        nil,
		ctx:        nil,
		gas:        nil,
	}
}

// NewVMWithContext создает новую VM с контекстом блокчейна
func NewVMWithContext(code []byte, ctx *Context) *VM {
	vm := NewVM(code)
	vm.ctx = ctx
	if ctx != nil && ctx.GasLimit > 0 {
		vm.gas = NewGasMeter(ctx.GasLimit)
	}
	return vm
}

// Run выполняет байткод
func (vm *VM) Run() ([]byte, error) {
	if len(vm.code) == 0 {
		return nil, nil
	}

	vm.pc = 0

	codeLen := len(vm.code)
	for vm.pc < codeLen && !vm.stopped && vm.err == nil {
		op := OpCode(vm.code[vm.pc])

		// Проверяем валидность опкода (оптимизация: проверка перед switch)
		if op > 0x97 { // Максимальный опкод SSTORE = 0x97
			vm.err = fmt.Errorf("%w: 0x%02x at position %d", ErrInvalidOpcode, op, vm.pc)
			break
		}

		// Для PUSH операций нужно обработать специально
		if op >= PUSH1 && op <= PUSH32 {
			size, _ := GetPushSize(op)
			if vm.pc+1+size > codeLen {
				vm.err = ErrInvalidBytecode
				break
			}
			// PC указывает на опкод, увеличиваем для чтения данных
			vm.pc++
			if err := vm.executeOpcode(op, vm.code); err != nil {
				vm.err = err
				break
			}
			// opPush уже скорректировал PC на размер данных
			continue
		}

		// Увеличиваем PC для следующей инструкции
		vm.pc++

		// Выполняем инструкцию
		if err := vm.executeOpcode(op, vm.code); err != nil {
			vm.err = err
			break
		}

		// После JUMP/JUMPI PC уже установлен на нужную позицию, продолжаем цикл
		if op == JUMP || op == JUMPI {
			continue
		}
	}

	if vm.err != nil {
		return nil, vm.err
	}

	return vm.returnData, nil
}

// executeOpcode выполняет одну инструкцию
func (vm *VM) executeOpcode(op OpCode, code []byte) error {
	switch op {
	// Остановка
	case STOP:
		vm.stopped = true
		return nil

	// Арифметические операции
	case ADD:
		return vm.opAdd()
	case SUB:
		return vm.opSub()
	case MUL:
		return vm.opMul()
	case DIV:
		return vm.opDiv()
	case MOD:
		return vm.opMod()

	// Логические операции
	case AND:
		return vm.opAnd()
	case OR:
		return vm.opOr()
	case XOR:
		return vm.opXor()
	case NOT:
		return vm.opNot()

	// Операции сравнения
	case LT:
		return vm.opLt()
	case GT:
		return vm.opGt()
	case EQ:
		return vm.opEq()
	case ISZERO:
		return vm.opIsZero()

	// Операции со стеком
	case POP:
		return vm.opPop()
	case DUP1, DUP2, DUP3, DUP4:
		return vm.opDup(int(op - DUP1 + 1))
	case SWAP1, SWAP2, SWAP3, SWAP4:
		return vm.opSwap(int(op - SWAP1 + 1))

	// Операции с памятью
	case MLOAD:
		return vm.opMload()
	case MSTORE:
		return vm.opMstore()

	// PUSH операции
	case PUSH1, PUSH2, PUSH4, PUSH8, PUSH16, PUSH32:
		return vm.opPush(op, code)

	// Управление потоком
	case JUMP:
		return vm.opJump(code)
	case JUMPI:
		return vm.opJumpi(code)
	case JUMPDEST:
		return nil // NOP, но валидируется при парсинге
	case PC:
		return vm.opPc()
	case RETURN:
		return vm.opReturn()
	case REVERT:
		return vm.opRevert()

	// Блокчейн-опкоды
	case ADDRESS:
		return vm.opAddress()
	case CALLER:
		return vm.opCaller()
	case CALLVALUE:
		return vm.opCallValue()
	case CALLDATALOAD:
		return vm.opCallDataLoad()
	case CALLDATASIZE:
		return vm.opCallDataSize()
	case CALLDATACOPY:
		return vm.opCallDataCopy()
	case SLOAD:
		return vm.opSload()
	case SSTORE:
		return vm.opSstore()

	default:
		return fmt.Errorf("%w: 0x%02x", ErrInvalidOpcode, op)
	}
}

// GetStack возвращает стек (для отладки)
func (vm *VM) GetStack() *Stack {
	return vm.stack
}

// GetMemory возвращает память (для отладки)
func (vm *VM) GetMemory() *Memory {
	return vm.memory
}

// GetContext возвращает контекст выполнения
func (vm *VM) GetContext() *Context {
	return vm.ctx
}

// GetGasMeter возвращает счетчик газа
func (vm *VM) GetGasMeter() *GasMeter {
	return vm.gas
}

// GasUsed возвращает использованный газ
func (vm *VM) GasUsed() uint64 {
	if vm.gas == nil {
		return 0
	}
	return vm.gas.GasUsed()
}
