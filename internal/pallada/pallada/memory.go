package pallada

import (
	"errors"
	"math/big"
)

// Ошибки памяти
var (
	ErrMemoryOutOfBounds = errors.New("memory out of bounds")
)

const (
	// MaxMemorySize - максимальный размер памяти в байтах (64 KB)
	MaxMemorySize = 64 * 1024
)

// Memory представляет память виртуальной машины
type Memory struct {
	store []byte
}

// NewMemory создает новую память
func NewMemory() *Memory {
	return &Memory{
		store: make([]byte, 0, 2048), // Увеличена начальная емкость для уменьшения реаллокаций
	}
}

// Set устанавливает байты в памяти по адресу offset
func (m *Memory) Set(offset uint64, size uint64, value []byte) error {
	if offset+size > MaxMemorySize {
		return ErrMemoryOutOfBounds
	}

	// Расширяем память если нужно (оптимизация: используем append с предварительным выделением)
	requiredLen := int(offset + size)
	if requiredLen > len(m.store) {
		// Вычисляем новую емкость с запасом для уменьшения реаллокаций
		newCap := requiredLen
		if newCap < 2048 {
			newCap = 2048
		} else if newCap < len(m.store)*2 {
			newCap = len(m.store) * 2
		}
		if newCap > MaxMemorySize {
			newCap = MaxMemorySize
		}
		// Расширяем существующий слайс
		oldLen := len(m.store)
		m.store = append(m.store, make([]byte, requiredLen-oldLen)...)
		// Увеличиваем capacity если нужно
		if cap(m.store) < newCap {
			newStore := make([]byte, len(m.store), newCap)
			copy(newStore, m.store)
			m.store = newStore
		}
	}

	// Копируем данные
	copy(m.store[offset:offset+size], value)
	return nil
}

// Get возвращает байты из памяти
func (m *Memory) Get(offset uint64, size uint64) ([]byte, error) {
	if offset+size > MaxMemorySize {
		return nil, ErrMemoryOutOfBounds
	}

	// Если выходим за границы существующей памяти, возвращаем нули
	if int(offset+size) > len(m.store) {
		return make([]byte, size), nil
	}

	result := make([]byte, size)
	copy(result, m.store[offset:offset+size])
	return result, nil
}

// Set32 устанавливает 32-байтовое значение (слово) в памяти
func (m *Memory) Set32(offset uint64, value *big.Int) error {
	// Преобразуем big.Int в 32 байта (256 бит)
	bytes := make([]byte, WordSize)
	valueBytes := value.Bytes()

	// big.Int.Bytes() возвращает минимальное представление, нужно дополнить до 32 байт
	start := WordSize - len(valueBytes)
	if start < 0 {
		// Если число больше 256 бит, берем младшие 32 байта
		copy(bytes, valueBytes[len(valueBytes)-WordSize:])
	} else {
		copy(bytes[start:], valueBytes)
	}

	return m.Set(offset, WordSize, bytes)
}

// Get32 возвращает 32-байтовое значение (слово) из памяти
func (m *Memory) Get32(offset uint64) (*big.Int, error) {
	data, err := m.Get(offset, WordSize)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

// Len возвращает текущий размер памяти в байтах
func (m *Memory) Len() int {
	return len(m.store)
}
