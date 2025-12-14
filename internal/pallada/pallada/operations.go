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
