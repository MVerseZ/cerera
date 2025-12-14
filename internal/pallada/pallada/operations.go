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
