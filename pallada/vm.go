package pallada

import (
	"fmt"
)

// VM представляет виртуальную машину для выполнения контрактов
// Принцип: Single Responsibility (SRP) - VM только управляет выполнением
type VM struct {
	code         []byte        // Байткод контракта
	ctx          *Context      // Контекст выполнения
	stack        *Stack        // Стек операндов
	memory       *Memory       // Память выполнения
	gasMeter     GasMeter      // Счетчик газа
	pc           int           // Program Counter (указатель на текущую инструкцию)
	returnData   []byte        // Данные для возврата (RETURN/REVERT)
	shouldReturn bool          // Флаг возврата
	shouldRevert bool          // Флаг отката
	logs         []*Log        // Накопленные события (LOG0-LOG4)
	jumpdests    map[int]bool  // Допустимые точки назначения JUMP/JUMPI
}

// NewVM создает новую виртуальную машину с указанным байткодом и контекстом
func NewVM(code []byte, ctx *Context) *VM {
	var gasMeter GasMeter
	if ctx == nil || ctx.GasLimit == 0 {
		gasMeter = NewGasMeter()
	} else {
		gasMeter = NewGasMeterWithLimit(ctx.GasLimit)
	}

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
		logs:         make([]*Log, 0),
		jumpdests:    buildJumpdests(code),
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

// PreCompile вычисляет стоимость газа для транзакции/контракта без выполнения
func (vm *VM) PreCompile(code []byte) (uint64, error) {
	var (
		pc             = 0
		gasUsed uint64 = 0
	)

	codeLen := len(code)
	for pc < codeLen {
		op := Opcode(code[pc])
		pc++

		// Базовый газ за каждую инструкцию
		gasUsed += GasBase

		switch {
		// Арифметика, сравнения и простейшие операции
		case op >= ADD && op <= SIGNEXTEND:
			gasUsed += GasVeryLow
		case op >= LT && op <= EQ:
			gasUsed += GasVeryLow
		case op >= ISZERO && op <= SAR:
			gasUsed += GasVeryLow

		// Хэш-операции (консервативная оценка: базовый газ без учёта данных)
		case op == KECCAK256:
			gasUsed += GasKeccak256
		case op == SHA256:
			gasUsed += GasSHA256
		case op == RIPEMD160:
			gasUsed += GasRIPEMD160
		case op == ECRECOVER:
			gasUsed += GasEcrecover

		// Calldata
		case op == CALLDATALOAD:
			gasUsed += GasCalldataLoad
		case op == CALLDATASIZE:
			gasUsed += GasCalldataSize
		case op == CALLDATACOPY:
			gasUsed += GasCalldataCopy

		// Управление потоком
		case op == JUMP:
			gasUsed += GasJump
		case op == JUMPI:
			gasUsed += GasJumpi
		case op == JUMPDEST:
			gasUsed += GasJumpdest

		// LOG события (консервативная оценка без данных)
		case op >= LOG0 && op <= LOG4:
			topics := uint64(op - LOG0)
			gasUsed += GasLogBase + topics*GasLogTopic

		// Стековые, память, storage
		case op == MLOAD:
			gasUsed += GasVeryLow
		case op == MSTORE || op == MSTORE8:
			gasUsed += GasVeryLow
		case op == SLOAD:
			gasUsed += GasSLoad
		case op == SSTORE:
			gasUsed += GasSStore
		case op == POP || op == MSIZE:
			gasUsed += GasBase

		// DUP / SWAP — дешёвые операции
		case op >= DUP1 && op <= DUP16:
			gasUsed += GasVeryLow
		case op >= SWAP1 && op <= SWAP16:
			gasUsed += GasVeryLow

		// CALL инструкции
		case op == CALL:
			gasUsed += GasCall

		// PUSH
		case op >= PUSH1 && op <= PUSH32:
			pushBytes := int(op - PUSH1 + 1)
			pc += pushBytes

		// RETURN/REVERT
		case op == RETURN || op == REVERT:
			gasUsed += GasReturn
		}
	}

	// Проверка переполнения газа
	if gasUsed > vm.gasMeter.GasLimit() {
		return gasUsed, fmt.Errorf("estimated gas exceeds limit: %d > %d", gasUsed, vm.gasMeter.GasLimit())
	}

	return gasUsed, nil

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

	// Криптографические хэш-операции и ECRECOVER
	case op >= KECCAK256 && op <= ECRECOVER:
		return executeHashOperations(vm, op)

	// Операции со стеком
	case op == POP || op == MSIZE:
		return executeStackOperations(vm, op)

	// Операции с памятью
	case op >= MLOAD && op <= MSTORE8:
		return executeMemoryOperations(vm, op)

	// Операции с storage
	case op == SLOAD || op == SSTORE:
		return executeStorageOperations(vm, op)

	// Calldata операции
	case op >= CALLDATALOAD && op <= CALLDATACOPY:
		return executeCalldataOperations(vm, op)

	// Управление потоком (JUMP, JUMPI, JUMPDEST)
	case op == JUMP || op == JUMPI || op == JUMPDEST:
		return executeJumpOperations(vm, op)

	// LOG события
	case op >= LOG0 && op <= LOG4:
		return executeLogOperations(vm, op)

	// PUSH операции (0x60-0x7F)
	case op >= PUSH1 && op <= PUSH32:
		return vm.executePush(op)

	// DUP операции (0x80-0x8F): DUPn дублирует n-й элемент с вершины
	case op >= DUP1 && op <= DUP16:
		return executeDup(vm, int(op-DUP1))

	// SWAP операции (0x90-0x9F): SWAPn меняет вершину с (n+1)-м элементом
	case op >= SWAP1 && op <= SWAP16:
		return executeSwap(vm, int(op-SWAP1)+1)

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

// GetLogs возвращает накопленные события контракта
func (vm *VM) GetLogs() []*Log {
	return vm.logs
}

// buildJumpdests предварительно вычисляет допустимые цели JUMP/JUMPI.
// Проходит байткод один раз, пропуская аргументы PUSH, и фиксирует
// все позиции JUMPDEST. Это гарантирует O(1) проверку при каждом прыжке.
func buildJumpdests(code []byte) map[int]bool {
	dests := make(map[int]bool)
	for i := 0; i < len(code); {
		op := Opcode(code[i])
		if op == JUMPDEST {
			dests[i] = true
		}
		if op >= PUSH1 && op <= PUSH32 {
			i += int(op-PUSH1) + 2 // опкод + N байт данных
		} else {
			i++
		}
	}
	return dests
}
