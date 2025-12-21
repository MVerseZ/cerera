package pallada

import (
	"math/big"
	"testing"
)

func TestStack_PushPop(t *testing.T) {
	stack := NewStack()

	// Тест базовых операций Push/Pop
	val1 := big.NewInt(42)
	val2 := big.NewInt(100)

	if err := stack.Push(val1); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := stack.Push(val2); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if stack.Depth() != 2 {
		t.Errorf("Expected depth 2, got %d", stack.Depth())
	}

	// Проверяем порядок извлечения (LIFO)
	popped2, err := stack.Pop()
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if popped2.Cmp(val2) != 0 {
		t.Errorf("Expected %s, got %s", val2.String(), popped2.String())
	}

	popped1, err := stack.Pop()
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if popped1.Cmp(val1) != 0 {
		t.Errorf("Expected %s, got %s", val1.String(), popped1.String())
	}

	if !stack.IsEmpty() {
		t.Error("Stack should be empty")
	}
}

func TestStack_Peek(t *testing.T) {
	stack := NewStack()

	val1 := big.NewInt(10)
	val2 := big.NewInt(20)

	stack.Push(val1)
	stack.Push(val2)

	// Peek не должен изменять стек
	peeked, err := stack.Peek()
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if peeked.Cmp(val2) != 0 {
		t.Errorf("Expected %s, got %s", val2.String(), peeked.String())
	}

	// Глубина стека не должна измениться
	if stack.Depth() != 2 {
		t.Errorf("Expected depth 2, got %d", stack.Depth())
	}

	// PeekAt для элемента под вершиной
	peekedAt, err := stack.PeekAt(1)
	if err != nil {
		t.Fatalf("PeekAt failed: %v", err)
	}
	if peekedAt.Cmp(val1) != 0 {
		t.Errorf("Expected %s, got %s", val1.String(), peekedAt.String())
	}
}

func TestStack_Swap(t *testing.T) {
	stack := NewStack()

	val1 := big.NewInt(10)
	val2 := big.NewInt(20)
	val3 := big.NewInt(30)

	stack.Push(val1)
	stack.Push(val2)
	stack.Push(val3)

	// Swap(1) меняет вершину (val3) с элементом на глубине 1 (val2)
	if err := stack.Swap(1); err != nil {
		t.Fatalf("Swap failed: %v", err)
	}

	// Проверяем результат
	top, _ := stack.Pop()
	if top.Cmp(val2) != 0 {
		t.Errorf("Expected %s, got %s", val2.String(), top.String())
	}

	second, _ := stack.Pop()
	if second.Cmp(val3) != 0 {
		t.Errorf("Expected %s, got %s", val3.String(), second.String())
	}
}

func TestStack_Dup(t *testing.T) {
	stack := NewStack()

	val1 := big.NewInt(10)
	val2 := big.NewInt(20)

	stack.Push(val1)
	stack.Push(val2)

	// Dup(0) дублирует вершину
	if err := stack.Dup(0); err != nil {
		t.Fatalf("Dup failed: %v", err)
	}

	if stack.Depth() != 3 {
		t.Errorf("Expected depth 3, got %d", stack.Depth())
	}

	// Проверяем, что вершина дублирована
	top, _ := stack.Pop()
	if top.Cmp(val2) != 0 {
		t.Errorf("Expected %s, got %s", val2.String(), top.String())
	}

	// Оригинал должен остаться
	original, _ := stack.Pop()
	if original.Cmp(val2) != 0 {
		t.Errorf("Expected %s, got %s", val2.String(), original.String())
	}
}

func TestStack_Overflow(t *testing.T) {
	stack := NewStack()

	// Заполняем стек до максимума
	for i := 0; i < MaxStackDepth; i++ {
		if err := stack.Push(big.NewInt(int64(i))); err != nil {
			t.Fatalf("Push failed at index %d: %v", i, err)
		}
	}

	// Попытка добавить еще один элемент должна вызвать ошибку
	if err := stack.Push(big.NewInt(9999)); err != ErrStackOverflow {
		t.Errorf("Expected ErrStackOverflow, got %v", err)
	}
}

func TestStack_Underflow(t *testing.T) {
	stack := NewStack()

	// Попытка извлечь из пустого стека
	_, err := stack.Pop()
	if err != ErrStackUnderflow {
		t.Errorf("Expected ErrStackUnderflow, got %v", err)
	}

	// Peek из пустого стека
	_, err = stack.Peek()
	if err != ErrStackInvalidPeek {
		t.Errorf("Expected ErrStackInvalidPeek, got %v", err)
	}
}

func TestStack_Clear(t *testing.T) {
	stack := NewStack()

	stack.Push(big.NewInt(1))
	stack.Push(big.NewInt(2))
	stack.Push(big.NewInt(3))

	stack.Clear()

	if !stack.IsEmpty() {
		t.Error("Stack should be empty after Clear")
	}

	if stack.Depth() != 0 {
		t.Errorf("Expected depth 0, got %d", stack.Depth())
	}
}

