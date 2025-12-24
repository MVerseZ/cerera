package pallada

import (
	"errors"
	"math/big"
)

var (
	// ErrStackUnderflow возникает при попытке извлечь элемент из пустого стека
	ErrStackUnderflow = errors.New("stack underflow")
	// ErrStackOverflow возникает при попытке добавить элемент в переполненный стек
	ErrStackOverflow = errors.New("stack overflow")
	// ErrStackInvalidPeek возникает при попытке получить элемент с недопустимым индексом
	ErrStackInvalidPeek = errors.New("invalid stack peek index")
)

// Stack представляет стек операндов для виртуальной машины
// Принцип: Single Responsibility (SRP) - только управление стеком
// Максимальная глубина стека: 1024 элемента (как в EVM)
const MaxStackDepth = 1024

type Stack struct {
	data []*big.Int // Данные стека
	ptr  int        // Указатель на вершину стека (индекс следующего элемента)
}

// NewStack создает новый стек операндов
func NewStack() *Stack {
	return &Stack{
		data: make([]*big.Int, 0, 16), // Начальная емкость 16 для оптимизации
		ptr:  0,
	}
}

// Push добавляет значение на вершину стека
func (s *Stack) Push(value *big.Int) error {
	if s.ptr >= MaxStackDepth {
		return ErrStackOverflow
	}

	// Если есть место в слайсе, используем его
	if s.ptr < len(s.data) {
		// Переиспользуем существующий элемент (оптимизация памяти)
		s.data[s.ptr].Set(value)
	} else {
		// Добавляем новый элемент
		s.data = append(s.data, new(big.Int).Set(value))
	}
	s.ptr++
	return nil
}

// Pop извлекает и возвращает значение с вершины стека
func (s *Stack) Pop() (*big.Int, error) {
	if s.ptr == 0 {
		return nil, ErrStackUnderflow
	}
	s.ptr--
	return s.data[s.ptr], nil
}

// Peek возвращает значение с вершины стека без извлечения
func (s *Stack) Peek() (*big.Int, error) {
	return s.PeekAt(0)
}

// PeekAt возвращает значение на указанной глубине от вершины стека
// depth=0 означает вершину стека, depth=1 - элемент под вершиной и т.д.
func (s *Stack) PeekAt(depth int) (*big.Int, error) {
	if depth < 0 || depth >= s.ptr {
		return nil, ErrStackInvalidPeek
	}
	index := s.ptr - 1 - depth
	return s.data[index], nil
}

// Swap меняет местами элементы на вершине стека
// swap(n) меняет местами элемент на вершине с элементом на глубине n
func (s *Stack) Swap(n int) error {
	if n < 1 || s.ptr < n+1 {
		return ErrStackInvalidPeek
	}

	top := s.ptr - 1
	target := s.ptr - 1 - n

	// Меняем местами
	s.data[top], s.data[target] = s.data[target], s.data[top]
	return nil
}

// Dup дублирует элемент на вершине стека
// dup(n) дублирует элемент на глубине n и помещает его на вершину
func (s *Stack) Dup(n int) error {
	if n < 0 || n >= s.ptr {
		return ErrStackInvalidPeek
	}

	if s.ptr >= MaxStackDepth {
		return ErrStackOverflow
	}

	index := s.ptr - 1 - n
	value := s.data[index]

	// Добавляем копию на вершину
	if s.ptr < len(s.data) {
		s.data[s.ptr].Set(value)
	} else {
		s.data = append(s.data, new(big.Int).Set(value))
	}
	s.ptr++
	return nil
}

// Depth возвращает текущую глубину стека
func (s *Stack) Depth() int {
	return s.ptr
}

// IsEmpty проверяет, пуст ли стек
func (s *Stack) IsEmpty() bool {
	return s.ptr == 0
}

// Clear очищает стек
func (s *Stack) Clear() {
	s.ptr = 0
	// Не обрезаем слайс для переиспользования памяти
}

// Back returns the underlying slice (for debugging/inspection)
// Используется только для отладки и тестирования
func (s *Stack) Back() []*big.Int {
	return s.data[:s.ptr]
}
