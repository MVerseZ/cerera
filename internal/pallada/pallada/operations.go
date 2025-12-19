package pallada

import (
	"fmt"
	"math/big"
)

// Предварительно созданные константы для оптимизации
var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
)

// ========== Арифметические операции ==========

func (vm *VM) opAdd() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	// Оптимизация: используем Add с результатом в новом big.Int
	result := new(big.Int).Add(b, a)
	return vm.stack.Push(result)
}

func (vm *VM) opSub() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	result := new(big.Int).Sub(b, a)
	return vm.stack.Push(result)
}

func (vm *VM) opMul() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	// Оптимизация: используем Mul с результатом в новом big.Int
	result := new(big.Int).Mul(b, a)
	return vm.stack.Push(result)
}

func (vm *VM) opDiv() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if a.Sign() == 0 {
		return vm.stack.Push(bigZero)
	}
	result := new(big.Int).Div(b, a)
	return vm.stack.Push(result)
}

func (vm *VM) opMod() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if a.Sign() == 0 {
		return vm.stack.Push(bigZero)
	}
	result := new(big.Int).Mod(b, a)
	return vm.stack.Push(result)
}

// ========== Логические операции ==========

func (vm *VM) opAnd() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	result := new(big.Int).And(a, b)
	return vm.stack.Push(result)
}

func (vm *VM) opOr() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	result := new(big.Int).Or(a, b)
	return vm.stack.Push(result)
}

func (vm *VM) opXor() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	result := new(big.Int).Xor(a, b)
	return vm.stack.Push(result)
}

func (vm *VM) opNot() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	result := new(big.Int).Not(a)
	// Ограничиваем до 256 бит
	mask := new(big.Int).Lsh(big.NewInt(1), 256)
	mask.Sub(mask, big.NewInt(1))
	result.And(result, mask)
	return vm.stack.Push(result)
}

// ========== Операции сравнения ==========

func (vm *VM) opLt() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if b.Cmp(a) < 0 {
		return vm.stack.Push(bigOne)
	}
	return vm.stack.Push(bigZero)
}

func (vm *VM) opGt() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if b.Cmp(a) > 0 {
		return vm.stack.Push(bigOne)
	}
	return vm.stack.Push(bigZero)
}

func (vm *VM) opEq() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	b, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if a.Cmp(b) == 0 {
		return vm.stack.Push(bigOne)
	}
	return vm.stack.Push(bigZero)
}

func (vm *VM) opIsZero() error {
	a, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	// Оптимизация: используем предварительно созданные константы
	if a.Sign() == 0 {
		return vm.stack.Push(bigZero)
	}
	return vm.stack.Push(bigOne)
}

// ========== Операции со стеком ==========

func (vm *VM) opPop() error {
	_, err := vm.stack.Pop()
	return err
}

func (vm *VM) opDup(n int) error {
	// Оптимизация: инлайним логику Dup для частых случаев
	if n == 1 {
		return vm.stack.Dup(1)
	}
	return vm.stack.Dup(n)
}

func (vm *VM) opSwap(n int) error {
	return vm.stack.Swap(n)
}

// ========== Операции с памятью ==========

func (vm *VM) opMload() error {
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	value, err := vm.memory.Get32(offset.Uint64())
	if err != nil {
		return err
	}
	return vm.stack.Push(value)
}

func (vm *VM) opMstore() error {
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	value, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	return vm.memory.Set32(offset.Uint64(), value)
}

// ========== PUSH операции ==========

func (vm *VM) opPush(op OpCode, code []byte) error {
	size, _ := GetPushSize(op)
	if vm.pc+size > len(code) {
		return ErrInvalidBytecode
	}
	value := new(big.Int).SetBytes(code[vm.pc : vm.pc+size])
	vm.pc += size // PC уже указывает на данные после увеличения в цикле
	return vm.stack.Push(value)
}

// ========== Управление потоком ==========

func (vm *VM) opJump(code []byte) error {
	dest, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if !dest.IsInt64() {
		return ErrInvalidJump
	}
	destInt := int(dest.Int64())
	if destInt < 0 || destInt >= len(code) {
		return ErrInvalidJump
	}
	// Проверяем, что это валидный JUMPDEST
	if OpCode(code[destInt]) != JUMPDEST {
		return ErrInvalidJump
	}
	// Устанавливаем PC на позицию JUMPDEST (в цикле будет continue, так что PC не увеличится)
	vm.pc = destInt
	return nil
}

func (vm *VM) opJumpi(code []byte) error {
	cond, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	dest, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	if cond.Sign() != 0 {
		if !dest.IsInt64() {
			return ErrInvalidJump
		}
		destInt := int(dest.Int64())
		if destInt < 0 || destInt >= len(code) {
			return ErrInvalidJump
		}
		if OpCode(code[destInt]) != JUMPDEST {
			return ErrInvalidJump
		}
		// Устанавливаем PC на позицию JUMPDEST (в цикле будет continue, так что PC не увеличится)
		vm.pc = destInt
	}
	// Если условие false, PC уже увеличен в основном цикле
	return nil
}

func (vm *VM) opPc() error {
	return vm.stack.Push(big.NewInt(int64(vm.pc)))
}

func (vm *VM) opReturn() error {
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	size, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	data, err := vm.memory.Get(offset.Uint64(), size.Uint64())
	if err != nil {
		return err
	}
	vm.returnData = data
	vm.stopped = true
	return nil
}

func (vm *VM) opRevert() error {
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	size, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	data, err := vm.memory.Get(offset.Uint64(), size.Uint64())
	if err != nil {
		return err
	}
	vm.returnData = data
	vm.stopped = true
	vm.err = fmt.Errorf("revert: %x", data)
	return nil
}

// ========== Блокчейн-опкоды (контекст выполнения) ==========

// opAddress возвращает адрес текущего контракта
func (vm *VM) opAddress() error {
	if vm.ctx == nil {
		return fmt.Errorf("ADDRESS: context not available")
	}
	// Расходуем газ
	if vm.gas != nil {
		if err := vm.gas.UseGas(vm.gas.Cost.Base); err != nil {
			return err
		}
	}
	// Конвертируем адрес в big.Int и пушим на стек
	addrInt := AddressToBigInt(vm.ctx.Address)
	return vm.stack.Push(addrInt)
}

// opCaller возвращает адрес вызывающего (отправителя транзакции)
func (vm *VM) opCaller() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALLER: context not available")
	}
	// Расходуем газ
	if vm.gas != nil {
		if err := vm.gas.UseGas(vm.gas.Cost.Base); err != nil {
			return err
		}
	}
	// Конвертируем адрес вызывающего в big.Int и пушим на стек
	callerInt := AddressToBigInt(vm.ctx.Caller)
	return vm.stack.Push(callerInt)
}

// opCallValue возвращает значение транзакции (wei)
func (vm *VM) opCallValue() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALLVALUE: context not available")
	}
	// Расходуем газ
	if vm.gas != nil {
		if err := vm.gas.UseGas(vm.gas.Cost.Base); err != nil {
			return err
		}
	}
	// Пушим значение на стек
	return vm.stack.Push(new(big.Int).Set(vm.ctx.Value))
}

// opCallDataLoad загружает 32 байта из данных вызова по указанному смещению
func (vm *VM) opCallDataLoad() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALLDATALOAD: context not available")
	}
	// Расходуем газ
	if vm.gas != nil {
		if err := vm.gas.UseGas(vm.gas.Cost.VeryLow); err != nil {
			return err
		}
	}
	// Получаем смещение из стека
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	offsetUint := offset.Uint64()

	// Читаем 32 байта из Input (или меньше, если данных недостаточно)
	// CALLDATALOAD загружает 32 байта, начиная с offset, дополняя нулями слева если данных недостаточно
	// В EVM данные копируются в младшие байты (справа), поэтому для big.Int (big-endian)
	// мы копируем данные в конец массива
	var data [32]byte
	inputLen := uint64(len(vm.ctx.Input))
	if offsetUint < inputLen {
		copyLen := uint64(32)
		if offsetUint+copyLen > inputLen {
			copyLen = inputLen - offsetUint
		}
		// Копируем данные в конец массива (младшие байты для big-endian)
		// Это соответствует поведению EVM, где данные идут в младшие позиции
		startPos := 32 - copyLen
		copy(data[startPos:], vm.ctx.Input[offsetUint:offsetUint+copyLen])
	}
	// Конвертируем в big.Int и пушим на стек
	result := new(big.Int).SetBytes(data[:])
	return vm.stack.Push(result)
}

// opCallDataSize возвращает размер данных вызова
func (vm *VM) opCallDataSize() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALLDATASIZE: context not available")
	}
	// Расходуем газ
	if vm.gas != nil {
		if err := vm.gas.UseGas(vm.gas.Cost.Base); err != nil {
			return err
		}
	}
	// Пушим размер Input на стек
	size := big.NewInt(int64(len(vm.ctx.Input)))
	return vm.stack.Push(size)
}

// opCallDataCopy копирует данные вызова в память
func (vm *VM) opCallDataCopy() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALLDATACOPY: context not available")
	}
	// Получаем параметры из стека: destOffset, offset, size
	destOffset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	offset, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	size, err := vm.stack.Pop()
	if err != nil {
		return err
	}

	destOffsetUint := destOffset.Uint64()
	offsetUint := offset.Uint64()
	sizeUint := size.Uint64()

	// Расходуем газ: базовая стоимость + стоимость памяти
	if vm.gas != nil {
		baseCost := vm.gas.Cost.VeryLow
		// Рассчитываем стоимость расширения памяти
		oldMemSize := uint64(vm.memory.Len())
		newMemSize := destOffsetUint + sizeUint
		memCost := vm.gas.CalculateMemoryGas(oldMemSize, newMemSize)
		// Стоимость копирования данных
		copyCost := (sizeUint + 31) / 32 * vm.gas.Cost.VeryLow
		totalCost := baseCost + memCost + copyCost
		if err := vm.gas.UseGas(totalCost); err != nil {
			return err
		}
	}

	// Копируем данные из Input в память
	inputLen := uint64(len(vm.ctx.Input))
	if offsetUint < inputLen {
		copyLen := sizeUint
		if offsetUint+copyLen > inputLen {
			copyLen = inputLen - offsetUint
		}
		data := make([]byte, copyLen)
		copy(data, vm.ctx.Input[offsetUint:offsetUint+copyLen])
		// Если размер больше, чем доступно, дополняем нулями
		if copyLen < sizeUint {
			fullData := make([]byte, sizeUint)
			copy(fullData, data)
			data = fullData
		}
		return vm.memory.Set(destOffsetUint, sizeUint, data)
	} else {
		// Если offset за пределами Input, записываем нули
		zeroData := make([]byte, sizeUint)
		return vm.memory.Set(destOffsetUint, sizeUint, zeroData)
	}
}

// opSload загружает значение из storage контракта
func (vm *VM) opSload() error {
	if vm.ctx == nil {
		return fmt.Errorf("SLOAD: context not available")
	}
	if vm.ctx.Storage == nil {
		return fmt.Errorf("SLOAD: storage not available")
	}

	// Получаем ключ из стека
	key, err := vm.stack.Pop()
	if err != nil {
		return err
	}

	// Расходуем газ: SLOAD стоит 100 газа (как в EVM)
	if vm.gas != nil {
		// Используем Low стоимость для SLOAD (100 газа)
		if err := vm.gas.UseGas(100); err != nil {
			return err
		}
	}

	// Получаем значение из storage
	value, err := vm.ctx.Storage.GetStorage(vm.ctx.Address, key)
	if err != nil {
		// При ошибке возвращаем 0 (как в EVM)
		return vm.stack.Push(big.NewInt(0))
	}

	// Кладем значение на стек
	return vm.stack.Push(value)
}

// opSstore сохраняет значение в storage контракта
func (vm *VM) opSstore() error {
	if vm.ctx == nil {
		return fmt.Errorf("SSTORE: context not available")
	}
	if vm.ctx.Storage == nil {
		return fmt.Errorf("SSTORE: storage not available")
	}

	// Получаем ключ и значение из стека
	key, err := vm.stack.Pop()
	if err != nil {
		return err
	}
	value, err := vm.stack.Pop()
	if err != nil {
		return err
	}

	// Получаем старое значение для расчета газа
	oldValue, _ := vm.ctx.Storage.GetStorage(vm.ctx.Address, key)
	if oldValue == nil {
		oldValue = big.NewInt(0)
	}

	// Расходуем газ: SSTORE имеет сложную логику расчета газа
	// Упрощенная версия: базовая стоимость + стоимость изменения
	if vm.gas != nil {
		baseCost := uint64(20000) // Базовая стоимость SSTORE

		// Если старое значение было 0, а новое не 0 - это создание (дополнительные 20000)
		if oldValue.Sign() == 0 && value.Sign() != 0 {
			baseCost += 20000 // Дополнительная стоимость создания
		}
		// Если новое значение равно старому - возвращаем газ (refund)
		if oldValue.Cmp(value) == 0 {
			// Ничего не делаем, но можно вернуть часть газа
		}
		// Если старое значение не 0, а новое 0 - это удаление (возврат 15000)
		if oldValue.Sign() != 0 && value.Sign() == 0 {
			// Возвращаем часть газа (refund)
			vm.gas.RefundGas(15000) // Упрощенная версия
		}

		if err := vm.gas.UseGas(baseCost); err != nil {
			return err
		}
	}

	// Сохраняем значение в storage
	return vm.ctx.Storage.SetStorage(vm.ctx.Address, key, value)
}

// opCall вызывает другой контракт
// Стек: [gas, address, value, inputOffset, inputSize, outputOffset, outputSize]
// Результат: [success] (1 = успех, 0 = ошибка)
// CALL опкод в EVM: CALL(gas, address, value, inputOffset, inputSize, outputOffset, outputSize)
func (vm *VM) opCall() error {
	if vm.ctx == nil {
		return fmt.Errorf("CALL: context not available")
	}

	if vm.ctx.CallerInt == nil {
		return fmt.Errorf("CALL: call interface not available")
	}

	// Проверяем, что на стеке достаточно элементов
	if vm.stack.Len() < 7 {
		return fmt.Errorf("CALL: stack underflow (need 7 elements)")
	}

	// Извлекаем параметры со стека (в обратном порядке)
	outputSize, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	outputOffset, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	inputSize, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	inputOffset, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	value, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	addressBig, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}
	gasLimit, err := vm.stack.Pop()
	if err != nil {
		return fmt.Errorf("CALL: %w", err)
	}

	// Конвертируем gasLimit в uint64
	if !gasLimit.IsUint64() {
		return fmt.Errorf("CALL: gas limit too large")
	}
	gasLimitUint := gasLimit.Uint64()

	// Конвертируем address из big.Int в Address
	address := BigIntToAddress(addressBig)

	// Читаем input данные из памяти
	inputData := make([]byte, 0)
	if inputSize.Sign() > 0 {
		if !inputSize.IsUint64() {
			return fmt.Errorf("CALL: input size too large")
		}
		inputSizeUint := inputSize.Uint64()
		if !inputOffset.IsUint64() {
			return fmt.Errorf("CALL: input offset too large")
		}
		inputOffsetUint := inputOffset.Uint64()

		// Читаем данные из памяти (Get автоматически расширяет память нулями)
		inputData, err = vm.memory.Get(inputOffsetUint, inputSizeUint)
		if err != nil {
			return fmt.Errorf("CALL: failed to read input from memory: %w", err)
		}
	}

	// Расходуем газ на вызов (базовая стоимость CALL)
	baseGasCost := uint64(700) // Базовая стоимость CALL (как в EVM)
	if err := vm.gas.UseGas(baseGasCost); err != nil {
		return err
	}

	// Расходуем газ на value (если value > 0)
	if value.Sign() > 0 {
		valueGasCost := uint64(9000) // Дополнительная стоимость при переводе value
		if err := vm.gas.UseGas(valueGasCost); err != nil {
			return err
		}
	}

	// Расходуем газ на память для output
	if outputSize.Sign() > 0 {
		if !outputSize.IsUint64() {
			return fmt.Errorf("CALL: output size too large")
		}
		outputSizeUint := outputSize.Uint64()
		if !outputOffset.IsUint64() {
			return fmt.Errorf("CALL: output offset too large")
		}
		outputOffsetUint := outputOffset.Uint64()

		// Расходуем газ на память
		currentMemSize := uint64(vm.memory.Len())
		requiredMemSize := outputOffsetUint + outputSizeUint
		if requiredMemSize > currentMemSize {
			memGas := vm.gas.CalculateMemoryGas(currentMemSize, requiredMemSize)
			if err := vm.gas.UseGas(memGas); err != nil {
				return err
			}
		}
	}

	// Ограничиваем gasLimit доступным газом
	availableGas := vm.gas.GasRemaining()
	if gasLimitUint > availableGas {
		gasLimitUint = availableGas
	}

	// Вызываем контракт через CallInterface
	result, success, gasUsed := vm.ctx.CallerInt.Call(
		vm.ctx.Address, // caller (текущий контракт)
		address,        // address (вызываемый контракт)
		value,          // value
		inputData,      // input
		gasLimitUint,   // gasLimit
	)

	// Расходуем газ, использованный при вызове
	if err := vm.gas.UseGas(gasUsed); err != nil {
		return err
	}

	// Записываем результат в память (если есть место)
	if outputSize.Sign() > 0 && success {
		outputSizeUint := outputSize.Uint64()
		outputOffsetUint := outputOffset.Uint64()

		// Копируем результат в память (обрезаем или дополняем нулями)
		resultSize := uint64(len(result))
		if resultSize > outputSizeUint {
			resultSize = outputSizeUint
		}

		// Подготавливаем данные для записи (дополняем нулями до outputSizeUint)
		outputData := make([]byte, outputSizeUint)
		if resultSize > 0 {
			copy(outputData, result[:resultSize])
		}

		// Записываем в память (Set автоматически расширяет память)
		if err := vm.memory.Set(outputOffsetUint, outputSizeUint, outputData); err != nil {
			return fmt.Errorf("CALL: failed to write output to memory: %w", err)
		}
	}

	// Помещаем результат на стек (1 = успех, 0 = ошибка)
	if success {
		if err := vm.stack.Push(big.NewInt(1)); err != nil {
			return fmt.Errorf("CALL: failed to push result: %w", err)
		}
	} else {
		if err := vm.stack.Push(big.NewInt(0)); err != nil {
			return fmt.Errorf("CALL: failed to push result: %w", err)
		}
	}

	return nil
}
