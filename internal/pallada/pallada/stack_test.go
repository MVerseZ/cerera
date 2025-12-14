package pallada

import (
	"math/big"
	"testing"
)

func TestStack_PushPop(t *testing.T) {
	stack := NewStack()

	// Тест Push
	value := big.NewInt(42)
	err := stack.Push(value)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", stack.Len())
	}

	// Тест Pop
	popped, err := stack.Pop()
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}

	if popped.Cmp(value) != 0 {
		t.Fatalf("Expected %v, got %v", value, popped)
	}

	if stack.Len() != 0 {
		t.Fatalf("Expected stack length 0, got %d", stack.Len())
	}
}

func TestStack_Peek(t *testing.T) {
	stack := NewStack()

	value := big.NewInt(100)
	stack.Push(value)

	// Peek не должен удалять элемент
	peeked, err := stack.Peek()
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}

	if peeked.Cmp(value) != 0 {
		t.Fatalf("Expected %v, got %v", value, peeked)
	}

	// Стек должен остаться с одним элементом
	if stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", stack.Len())
	}
}

func TestStack_Underflow(t *testing.T) {
	stack := NewStack()

	// Попытка Pop из пустого стека
	_, err := stack.Pop()
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}

	// Попытка Peek из пустого стека
	_, err = stack.Peek()
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}
}

func TestStack_Overflow(t *testing.T) {
	stack := NewStack()

	// Заполняем стек до лимита
	for i := 0; i < MaxStackDepth; i++ {
		err := stack.Push(big.NewInt(int64(i)))
		if err != nil {
			t.Fatalf("Push failed at %d: %v", i, err)
		}
	}

	// Попытка добавить еще один элемент должна вызвать ошибку
	err := stack.Push(big.NewInt(999))
	if err != ErrStackOverflow {
		t.Fatalf("Expected ErrStackOverflow, got %v", err)
	}
}

func TestStack_Dup(t *testing.T) {
	stack := NewStack()

	// Добавляем два элемента
	stack.Push(big.NewInt(10))
	stack.Push(big.NewInt(20))

	// Дублируем верхний элемент (20)
	err := stack.Dup(1)
	if err != nil {
		t.Fatalf("Dup failed: %v", err)
	}

	// Теперь стек: [10, 20, 20]
	if stack.Len() != 3 {
		t.Fatalf("Expected stack length 3, got %d", stack.Len())
	}

	// Проверяем верхний элемент
	top, _ := stack.Pop()
	if top.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("Expected 20, got %v", top)
	}

	// Дублируем второй элемент (10)
	err = stack.Dup(2)
	if err != nil {
		t.Fatalf("Dup failed: %v", err)
	}

	// Теперь стек: [10, 20, 10]
	top, _ = stack.Pop()
	if top.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("Expected 10, got %v", top)
	}
}

func TestStack_Swap(t *testing.T) {
	stack := NewStack()

	// Добавляем элементы: [10, 20, 30]
	stack.Push(big.NewInt(10))
	stack.Push(big.NewInt(20))
	stack.Push(big.NewInt(30))

	// Меняем местами верхний (30) и второй (20)
	err := stack.Swap(1)
	if err != nil {
		t.Fatalf("Swap failed: %v", err)
	}

	// Теперь стек: [10, 30, 20]
	top, _ := stack.Pop()
	if top.Cmp(big.NewInt(20)) != 0 {
		t.Fatalf("Expected 20, got %v", top)
	}

	second, _ := stack.Pop()
	if second.Cmp(big.NewInt(30)) != 0 {
		t.Fatalf("Expected 30, got %v", second)
	}
}

func TestStack_DupUnderflow(t *testing.T) {
	stack := NewStack()

	// Попытка дублировать элемент из пустого стека
	err := stack.Dup(1)
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}

	// Добавляем один элемент
	stack.Push(big.NewInt(10))

	// Попытка дублировать второй элемент (которого нет)
	err = stack.Dup(2)
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}
}

func TestStack_SwapUnderflow(t *testing.T) {
	stack := NewStack()

	// Попытка поменять местами из пустого стека
	err := stack.Swap(1)
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}

	// Добавляем один элемент
	stack.Push(big.NewInt(10))

	// Попытка поменять местами (нужно минимум 2 элемента)
	err = stack.Swap(1)
	if err != ErrStackUnderflow {
		t.Fatalf("Expected ErrStackUnderflow, got %v", err)
	}
}
