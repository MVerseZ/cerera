package pallada

import (
	"errors"
	"math/big"
)

var (
	// ErrMemoryOutOfBounds возникает при обращении к памяти за пределами допустимого диапазона
	ErrMemoryOutOfBounds = errors.New("memory out of bounds")
)

// Memory представляет линейную память для выполнения контракта
// Принцип: Single Responsibility (SRP) - только управление памятью
// Память расширяется динамически, но имеет ограничение по размеру
const MaxMemorySize = 1024 * 1024 // 1 MB максимум (как в EVM)

type Memory struct {
	data []byte // Данные памяти
	size uint64 // Текущий размер памяти в байтах
}

// NewMemory создает новую память
func NewMemory() *Memory {
	return &Memory{
		data: make([]byte, 0, 1024), // Начальная емкость 1KB для оптимизации
		size: 0,
	}
}

// Resize расширяет память до указанного размера (если нужно)
func (m *Memory) Resize(newSize uint64) error {
	if newSize > MaxMemorySize {
		return ErrMemoryOutOfBounds
	}

	if newSize > uint64(len(m.data)) {
		// Расширяем память, заполняя нулями
		oldLen := len(m.data)
		m.data = append(m.data, make([]byte, int(newSize)-oldLen)...)
	}

	if newSize > m.size {
		m.size = newSize
	}

	return nil
}

// Store сохраняет 32-байтовое значение (word) в памяти по указанному offset
func (m *Memory) Store(offset *big.Int, value *big.Int) error {
	off := offset.Uint64()

	// Проверяем границы (32 байта для word)
	if off+32 < off { // Overflow check
		return ErrMemoryOutOfBounds
	}

	// Расширяем память, если нужно
	if err := m.Resize(off + 32); err != nil {
		return err
	}

	// Конвертируем big.Int в 32 байта (big-endian)
	valueBytes := value.Bytes()
	word := make([]byte, 32)

	// Копируем байты справа налево (big-endian)
	start := 32 - len(valueBytes)
	copy(word[start:], valueBytes)

	// Сохраняем в память
	copy(m.data[off:off+32], word)

	return nil
}

// Load загружает 32-байтовое значение (word) из памяти по указанному offset
func (m *Memory) Load(offset *big.Int) (*big.Int, error) {
	off := offset.Uint64()

	// Проверяем границы
	if off+32 < off || off+32 > m.size {
		return nil, ErrMemoryOutOfBounds
	}

	// Читаем 32 байта
	word := make([]byte, 32)
	copy(word, m.data[off:off+32])

	// Конвертируем в big.Int (big-endian)
	return new(big.Int).SetBytes(word), nil
}

// StoreByte сохраняет один байт в памяти по указанному offset
func (m *Memory) StoreByte(offset *big.Int, value byte) error {
	off := offset.Uint64()

	// Проверяем границы
	if off+1 < off {
		return ErrMemoryOutOfBounds
	}

	// Расширяем память, если нужно
	if err := m.Resize(off + 1); err != nil {
		return err
	}

	// Сохраняем байт
	m.data[off] = value
	return nil
}

// LoadByte загружает один байт из памяти по указанному offset
func (m *Memory) LoadByte(offset *big.Int) (byte, error) {
	off := offset.Uint64()

	// Проверяем границы
	if off >= m.size {
		return 0, ErrMemoryOutOfBounds
	}

	return m.data[off], nil
}

// Copy копирует данные из памяти в указанный буфер
// offset - начальная позиция в памяти
// length - количество байтов для копирования
func (m *Memory) Copy(offset *big.Int, length *big.Int) ([]byte, error) {
	off := offset.Uint64()
	len := length.Uint64()

	// Проверяем границы
	if off+len < off || off+len > m.size {
		return nil, ErrMemoryOutOfBounds
	}

	// Копируем данные
	result := make([]byte, len)
	copy(result, m.data[off:off+len])

	return result, nil
}

// Set копирует данные из буфера в память
// offset - начальная позиция в памяти
// data - данные для копирования
func (m *Memory) Set(offset *big.Int, data []byte) error {
	off := offset.Uint64()
	length := uint64(len(data))

	// Проверяем границы
	if off+length < off {
		return ErrMemoryOutOfBounds
	}

	// Расширяем память, если нужно
	if err := m.Resize(off + length); err != nil {
		return err
	}

	// Копируем данные
	copy(m.data[off:off+length], data)

	return nil
}

// Size возвращает текущий размер памяти в байтах
func (m *Memory) Size() uint64 {
	return m.size
}

// Data возвращает копию всех данных памяти (для отладки)
func (m *Memory) Data() []byte {
	result := make([]byte, m.size)
	copy(result, m.data[:m.size])
	return result
}

// Clear очищает память
func (m *Memory) Clear() {
	m.size = 0
	// Не обрезаем слайс для переиспользования памяти
}
