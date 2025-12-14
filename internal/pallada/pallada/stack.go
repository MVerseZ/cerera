package pallada

import (
	"errors"
	"math/big"
)

// Ошибки стека
var (
	ErrStackUnderflow = errors.New("stack underflow")
	ErrStackOverflow  = errors.New("stack overflow")
)

const (
	// MaxStackDepth - максимальная глубина стека
	MaxStackDepth = 1024
	// WordSize - размер слова в байтах (256 бит = 32 байта)
	WordSize = 32
)

// Stack представляет стек виртуальной машины
type Stack struct {
	data []*big.Int
}

// NewStack создает новый стек
func NewStack() *Stack {
	return &Stack{
		data: make([]*big.Int, 0, 32), // Увеличена начальная емкость для уменьшения реаллокаций
	}
}

// Push добавляет значение на вершину стека
func (s *Stack) Push(value *big.Int) error {
	if len(s.data) >= MaxStackDepth {
		return ErrStackOverflow
	}
	s.data = append(s.data, value)
	return nil
}

// Pop извлекает и возвращает значение с вершины стека
func (s *Stack) Pop() (*big.Int, error) {
	if len(s.data) == 0 {
		return nil, ErrStackUnderflow
	}
	idx := len(s.data) - 1
	value := s.data[idx]
	s.data = s.data[:idx]
	return value, nil
}

// Peek возвращает значение с вершины стека без извлечения
func (s *Stack) Peek() (*big.Int, error) {
	if len(s.data) == 0 {
		return nil, ErrStackUnderflow
	}
	return s.data[len(s.data)-1], nil
}

// Len возвращает текущую глубину стека
func (s *Stack) Len() int {
	return len(s.data)
}

// Dup дублирует n-й элемент с вершины стека (n=1 это верхний элемент)
// Например: Dup(1) дублирует верхний элемент, Dup(2) дублирует второй с верха
func (s *Stack) Dup(n int) error {
	if n < 1 {
		return ErrStackUnderflow
	}
	if len(s.data) < n {
		return ErrStackUnderflow
	}
	// n-й элемент с верха находится на позиции len(s.data) - n
	idx := len(s.data) - n
	// Создаем копию значения
	value := new(big.Int).Set(s.data[idx])
	return s.Push(value)
}

// Swap меняет местами верхний элемент с n-м элементом (n=1 меняет с вторым)
// Например: Swap(1) меняет верхний и второй элемент
func (s *Stack) Swap(n int) error {
	if n < 1 {
		return ErrStackUnderflow
	}
	if len(s.data) < n+1 {
		return ErrStackUnderflow
	}
	// Верхний элемент на позиции len(s.data) - 1
	// n-й элемент с верха на позиции len(s.data) - n - 1
	topIdx := len(s.data) - 1
	targetIdx := len(s.data) - n - 1

	// Меняем местами
	s.data[topIdx], s.data[targetIdx] = s.data[targetIdx], s.data[topIdx]
	return nil
}
