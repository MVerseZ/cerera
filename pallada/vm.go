package pallada

import (
	"fmt"
)

// VM представляет виртуальную машину для выполнения контрактов
// Принцип: Single Responsibility (SRP) - VM только управляет выполнением
type VM struct {
	code         []byte   // Байткод контракта
	ctx          *Context // Контекст выполнения
	stack        *Stack   // Стек операндов
	memory       *Memory  // Память выполнения
	gasMeter     GasMeter // Счетчик газа
	pc           int      // Program Counter (указатель на текущую инструкцию)
	returnData   []byte   // Данные для возврата (RETURN/REVERT)
	shouldReturn bool     // Флаг возврата
	shouldRevert bool     // Флаг отката
}

// NewVM создает новую виртуальную машину с указанным байткодом и контекстом
func NewVM(code []byte, ctx *Context) *VM {
	gasMeter := NewGasMeter(ctx.GasLimit)

	return &VM{
		code:         code,
		ctx:          ctx,
		stack:        NewStack(),
		memory:       NewMemory(),
		gasMeter:     gasMeter,
		pc:           0,
		returnData:   nil,
		shouldReturn: false,
		shouldRevert: false,
	}
}

// NewVMWithContext - алиас для NewVM (для обратной совместимости)
func NewVMWithContext(code []byte, ctx *Context) *VM {
	return NewVM(code, ctx)
}

// GetGasMeter возвращает счетчик газа (для внешнего доступа)
func (vm *VM) GetGasMeter() GasMeter {
	return vm.gasMeter
}

// GasUsed возвращает использованное количество газа
func (vm *VM) GasUsed() uint64 {
	if vm.gasMeter == nil {
		return 0
	}
	return vm.gasMeter.GasUsed()
}

// Run выполняет байткод контракта
// Возвращает результат выполнения и ошибку (если есть)
func (vm *VM) Run() ([]byte, error) {
	vm.pc = 0
	vm.returnData = nil
	vm.shouldReturn = false
	vm.shouldRevert = false

	// Основной цикл выполнения
	for vm.pc < len(vm.code) {
		// Потребляем базовый газ за каждую инструкцию
		if err := vm.gasMeter.ConsumeGas(GasBase, "instruction"); err != nil {
			return nil, fmt.Errorf("out of gas at PC %d: %w", vm.pc, err)
		}

		// Читаем опкод
		opcode := Opcode(vm.code[vm.pc])
		vm.pc++

		// Выполняем опкод
		err := vm.executeOpcode(opcode)

		// Обрабатываем специальные случаи
		if err != nil {
			// Проверяем, это нормальная остановка или ошибка
			if err == ErrExecutionStopped {
				// STOP - нормальная остановка
				return nil, nil
			}
			if err == ErrExecutionReturned || vm.shouldReturn {
				// RETURN - возврат данных
				return vm.returnData, nil
			}
			if err == ErrExecutionReverted || vm.shouldRevert {
				// REVERT - откат с данными
				return vm.returnData, fmt.Errorf("revert: %x", vm.returnData)
			}
			// Другая ошибка
			return nil, fmt.Errorf("execution error at PC %d (opcode 0x%02X): %w", vm.pc-1, opcode, err)
		}
	}

	// Выполнение завершено без явного RETURN/STOP
	return vm.returnData, nil
}

// executeOpcode выполняет один опкод
func (vm *VM) executeOpcode(op Opcode) error {
	// Диспетчер опкодов
	switch {
	// STOP
	case op == STOP:
		return executeStop(vm)

	// Арифметические операции
	case op >= ADD && op <= SIGNEXTEND:
		return executeArithmetic(vm, op)

	// Операции сравнения
	case op >= LT && op <= EQ:
		return executeComparison(vm, op)

	// Побитовые операции
	case op >= ISZERO && op <= SAR:
		return executeBitwise(vm, op)

	// Операции со стеком
	case op == POP || op == MSIZE:
		return executeStackOperations(vm, op)

	// Операции с памятью
	case op >= MLOAD && op <= MSTORE8:
		return executeMemoryOperations(vm, op)

	// Операции с storage
	case op == SLOAD || op == SSTORE:
		return executeStorageOperations(vm, op)

	// PUSH операции (0x60-0x7F)
	case op >= PUSH1 && op <= PUSH32:
		return vm.executePush(op)

	// Операции вызовов
	case op == RETURN || op == REVERT || op == CALL:
		return executeCallOperations(vm, op)

	default:
		return fmt.Errorf("%w: 0x%02X", ErrInvalidOpcode, op)
	}
}

// executePush выполняет PUSH операцию
func (vm *VM) executePush(op Opcode) error {
	// Количество байтов для PUSH
	byteCount := int(op) - int(PUSH1) + 1

	// Проверяем, достаточно ли байтов в коде
	if vm.pc+byteCount > len(vm.code) {
		return fmt.Errorf("insufficient bytes for PUSH%d at PC %d", byteCount, vm.pc-1)
	}

	// Берем байты из кода
	pushBytes := vm.code[vm.pc : vm.pc+byteCount]
	vm.pc += byteCount

	// Выполняем PUSH
	return executePush(vm, op, pushBytes)
}

// GetReturnData возвращает данные для возврата (для отладки)
func (vm *VM) GetReturnData() []byte {
	return vm.returnData
}

// GetMemory возвращает доступ к памяти VM (для отладки и тестирования)
func (vm *VM) GetMemory() *Memory {
	return vm.memory
}
