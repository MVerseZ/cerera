package pallada

import (
	"errors"
	"math/big"

	"github.com/cerera/internal/cerera/types"
)

var (
	// ErrExecutionStopped возникает при выполнении STOP
	ErrExecutionStopped = errors.New("execution stopped")
	// ErrExecutionReturned возникает при выполнении RETURN
	ErrExecutionReturned = errors.New("execution returned")
	// ErrExecutionReverted возникает при выполнении REVERT
	ErrExecutionReverted = errors.New("execution reverted")
)

// executeStackOperations выполняет операции со стеком
func executeStackOperations(vm *VM, op Opcode) error {
	switch op {
	case POP:
		// Удалить элемент со стека
		_, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		// Потребляем газ
		return vm.gasMeter.ConsumeGas(GasBase, "POP")

	case MSIZE:
		// Размер памяти в байтах
		size := vm.memory.Size()
		// Потребляем газ
		if err := vm.gasMeter.ConsumeGas(GasBase, "MSIZE"); err != nil {
			return err
		}
		// Помещаем размер на стек
		return vm.stack.Push(big.NewInt(int64(size)))

	default:
		return ErrInvalidOpcode
	}
}

// executePush выполняет операции PUSH (PUSH1-PUSH32)
// pushOpcode - опкод PUSH (0x60-0x7F)
// bytes - байты для помещения на стек
func executePush(vm *VM, pushOpcode Opcode, bytes []byte) error {
	// Количество байтов определяется опкодом: PUSH1 = 0x60, PUSH32 = 0x7F
	byteCount := int(pushOpcode) - int(PUSH1) + 1

	if len(bytes) < byteCount {
		return errors.New("insufficient bytes for PUSH")
	}

	// Берем только нужное количество байтов
	pushBytes := bytes[:byteCount]

	// Конвертируем в big.Int (big-endian)
	value := new(big.Int).SetBytes(pushBytes)

	// Потребляем газ (PUSH очень дешевый)
	if err := vm.gasMeter.ConsumeGas(GasVeryLow, "PUSH"); err != nil {
		return err
	}

	// Помещаем на стек
	return vm.stack.Push(value)
}

// executeDup выполняет операции DUP (дублирование элементов стека)
// dupOpcode - опкод DUP (DUP1-DUP16 в EVM, у нас упрощенная версия)
func executeDup(vm *VM, depth int) error {
	// Потребляем газ
	if err := vm.gasMeter.ConsumeGas(GasVeryLow, "DUP"); err != nil {
		return err
	}

	// Дублируем элемент на указанной глубине
	return vm.stack.Dup(depth)
}

// executeSwap выполняет операции SWAP (обмен элементов стека)
// swapOpcode - опкод SWAP (SWAP1-SWAP16 в EVM)
func executeSwap(vm *VM, depth int) error {
	// Потребляем газ
	if err := vm.gasMeter.ConsumeGas(GasVeryLow, "SWAP"); err != nil {
		return err
	}

	// Меняем местами элементы
	return vm.stack.Swap(depth)
}

// executeStop выполняет операцию STOP
func executeStop(vm *VM) error {
	// Потребляем базовый газ
	if err := vm.gasMeter.ConsumeGas(GasZero, "STOP"); err != nil {
		return err
	}

	// Останавливаем выполнение (возвращаем специальную ошибку)
	return ErrExecutionStopped
}

// executeMemoryOperations выполняет операции с памятью
func executeMemoryOperations(vm *VM, op Opcode) error {
	switch op {
	case MLOAD:
		// Загрузить 32 байта из памяти
		// Стек: offset -> value
		offset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Загружаем значение из памяти
		value, err := vm.memory.Load(offset)
		if err != nil {
			return err
		}

		// Вычисляем стоимость газа для расширения памяти
		currentSize := vm.memory.Size()
		newSize := offset.Uint64() + 32
		if newSize > currentSize {
			gasCost := CalculateMemoryGas(currentSize, newSize)
			if err := vm.gasMeter.ConsumeGas(gasCost, "MLOAD memory expansion"); err != nil {
				return err
			}
		}

		// Потребляем базовый газ для MLOAD
		if err := vm.gasMeter.ConsumeGas(GasVeryLow, "MLOAD"); err != nil {
			return err
		}

		// Помещаем значение на стек
		return vm.stack.Push(value)

	case MSTORE:
		// Сохранить 32 байта в память
		// Стек: offset, value -> (пусто)
		offset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Вычисляем стоимость газа для расширения памяти
		currentSize := vm.memory.Size()
		newSize := offset.Uint64() + 32
		if newSize > currentSize {
			gasCost := CalculateMemoryGas(currentSize, newSize)
			if err := vm.gasMeter.ConsumeGas(gasCost, "MSTORE memory expansion"); err != nil {
				return err
			}
		}

		// Потребляем базовый газ для MSTORE
		if err := vm.gasMeter.ConsumeGas(GasVeryLow, "MSTORE"); err != nil {
			return err
		}

		// Сохраняем значение в память
		return vm.memory.Store(offset, value)

	case MSTORE8:
		// Сохранить 1 байт в память
		// Стек: offset, value -> (пусто)
		offset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Берем младший байт значения
		byteValue := byte(value.Uint64() & 0xFF)

		// Вычисляем стоимость газа для расширения памяти
		currentSize := vm.memory.Size()
		newSize := offset.Uint64() + 1
		if newSize > currentSize {
			gasCost := CalculateMemoryGas(currentSize, newSize)
			if err := vm.gasMeter.ConsumeGas(gasCost, "MSTORE8 memory expansion"); err != nil {
				return err
			}
		}

		// Потребляем базовый газ для MSTORE8
		if err := vm.gasMeter.ConsumeGas(GasVeryLow, "MSTORE8"); err != nil {
			return err
		}

		// Сохраняем байт в память
		return vm.memory.StoreByte(offset, byteValue)

	default:
		return ErrInvalidOpcode
	}
}

// executeStorageOperations выполняет операции с storage контракта
func executeStorageOperations(vm *VM, op Opcode) error {
	if vm.ctx == nil || vm.ctx.Storage == nil {
		return errors.New("storage not available")
	}

	switch op {
	case SLOAD:
		// Загрузить значение из storage
		// Стек: key -> value
		key, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Потребляем газ для SLOAD
		if err := vm.gasMeter.ConsumeGas(GasSLoad, "SLOAD"); err != nil {
			return err
		}

		// Загружаем значение из storage
		value, err := vm.ctx.Storage.GetStorage(vm.ctx.Address, key)
		if err != nil {
			// Если ключ не существует, возвращаем 0
			value = big.NewInt(0)
		}

		// Помещаем значение на стек
		return vm.stack.Push(value)

	case SSTORE:
		// Сохранить значение в storage
		// Стек: key, value -> (пусто)
		key, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Получаем текущее значение для расчета стоимости газа
		currentValue, err := vm.ctx.Storage.GetStorage(vm.ctx.Address, key)
		if err != nil {
			currentValue = big.NewInt(0) // Ключ не существует
		}

		// Вычисляем стоимость газа для SSTORE
		gasCost := calculateSStoreGas(currentValue, value)

		// Потребляем газ
		if err := vm.gasMeter.ConsumeGas(gasCost, "SSTORE"); err != nil {
			return err
		}

		// Сохраняем значение в storage
		return vm.ctx.Storage.SetStorage(vm.ctx.Address, key, value)

	default:
		return ErrInvalidOpcode
	}
}

// calculateSStoreGas вычисляет стоимость газа для операции SSTORE
// Логика основана на EVM:
// - Если значение меняется с 0 на не-0: GasSStoreSet (20000)
// - Если значение меняется с не-0 на 0: возврат 15000 (refund)
// - Если значение меняется с не-0 на не-0: GasSStoreReset (5000)
func calculateSStoreGas(currentValue, newValue *big.Int) uint64 {
	currentIsZero := currentValue.Sign() == 0
	newIsZero := newValue.Sign() == 0

	// Если значения одинаковые, минимальная стоимость
	if currentValue.Cmp(newValue) == 0 {
		return GasSStore
	}

	// Если меняем с 0 на не-0: дорогая операция
	if currentIsZero && !newIsZero {
		return GasSStoreSet
	}

	// Если меняем с не-0 на 0: возврат (но все равно списываем базовую стоимость)
	// В реальной EVM это возврат газа, но для упрощения просто используем базовую стоимость
	if !currentIsZero && newIsZero {
		return GasSStoreReset
	}

	// Если меняем с не-0 на не-0: средняя стоимость
	return GasSStoreReset
}

// executeCallOperations выполняет операции вызовов и управления потоком
func executeCallOperations(vm *VM, op Opcode) error {
	switch op {
	case RETURN:
		// Возврат данных из контракта
		// Стек: offset, length -> (пусто)
		// Возвращает данные из памяти [offset:offset+length]
		offset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		length, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Потребляем газ для RETURN
		if err := vm.gasMeter.ConsumeGas(GasReturn, "RETURN"); err != nil {
			return err
		}

		// Копируем данные из памяти
		returnData, err := vm.memory.Copy(offset, length)
		if err != nil {
			return err
		}

		// Сохраняем данные для возврата (VM должен обработать это)
		vm.returnData = returnData
		vm.shouldReturn = true

		// Останавливаем выполнение
		return ErrExecutionReturned

	case REVERT:
		// Откат транзакции с данными
		// Стек: offset, length -> (пусто)
		// Аналогично RETURN, но с ошибкой
		offset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		length, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Потребляем газ для REVERT
		if err := vm.gasMeter.ConsumeGas(GasRevert, "REVERT"); err != nil {
			return err
		}

		// Копируем данные из памяти
		revertData, err := vm.memory.Copy(offset, length)
		if err != nil {
			return err
		}

		// Сохраняем данные для отката
		vm.returnData = revertData
		vm.shouldRevert = true

		// Останавливаем выполнение с ошибкой
		return ErrExecutionReverted

	case CALL:
		// Межконтрактный вызов
		// Стек: gas, address, value, inputOffset, inputLength, outputOffset, outputLength -> success
		if vm.ctx == nil || vm.ctx.Call == nil {
			return errors.New("call interface not available")
		}

		// Извлекаем параметры со стека (в обратном порядке)
		outputLength, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		outputOffset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		inputLength, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		inputOffset, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		value, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		addressBig, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}
		gasLimit, err := vm.stack.Pop()
		if err != nil {
			return ErrStackUnderflowOpcode
		}

		// Конвертируем address из big.Int в Address
		// Address это 32 байта, берем последние байты
		addressBytes := addressBig.Bytes()
		// Если байтов больше 32, берем последние 32
		if len(addressBytes) > 32 {
			addressBytes = addressBytes[len(addressBytes)-32:]
		}
		// Дополняем до 32 байт слева нулями, если нужно
		if len(addressBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(addressBytes):], addressBytes)
			addressBytes = padded
		}
		address := types.BytesToAddress(addressBytes)

		// Копируем входные данные из памяти
		inputData, err := vm.memory.Copy(inputOffset, inputLength)
		if err != nil {
			return err
		}

		// Вычисляем стоимость газа для CALL
		gasCost := GasCall
		if value.Sign() > 0 {
			gasCost += GasCallValue
		}

		// Потребляем газ
		if err := vm.gasMeter.ConsumeGas(gasCost, "CALL"); err != nil {
			return err
		}

		// Проверяем, достаточно ли газа для вызова
		callGasLimit := gasLimit.Uint64()
		if callGasLimit > vm.gasMeter.GasRemaining() {
			callGasLimit = vm.gasMeter.GasRemaining()
		}

		// Вызываем контракт через CallInterface
		result, success, gasUsed := vm.ctx.Call.Call(
			vm.ctx.Caller,
			address,
			value,
			inputData,
			callGasLimit,
		)

		// Потребляем использованный газ
		if err := vm.gasMeter.ConsumeGas(gasUsed, "CALL execution"); err != nil {
			return err
		}

		// Сохраняем результат в память
		if success && len(result) > 0 {
			outputLen := outputLength.Uint64()
			if outputLen > uint64(len(result)) {
				outputLen = uint64(len(result))
			}

			// Вычисляем стоимость газа для расширения памяти
			currentSize := vm.memory.Size()
			newSize := outputOffset.Uint64() + outputLen
			if newSize > currentSize {
				gasCost := CalculateMemoryGas(currentSize, newSize)
				if err := vm.gasMeter.ConsumeGas(gasCost, "CALL memory expansion"); err != nil {
					return err
				}
			}

			// Сохраняем результат в память
			if err := vm.memory.Set(outputOffset, result[:outputLen]); err != nil {
				return err
			}
		}

		// Помещаем результат (1 = успех, 0 = неудача) на стек
		if success {
			return vm.stack.Push(big.NewInt(1))
		}
		return vm.stack.Push(big.NewInt(0))

	default:
		return ErrInvalidOpcode
	}
}
