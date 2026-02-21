package pallada

import (
	"math/big"
	"testing"
)

func TestMemory_StoreLoad(t *testing.T) {
	mem := NewMemory()

	// Тест базовых операций Store/Load
	offset := big.NewInt(0)
	value := big.NewInt(0x1234567890ABCDEF)

	// Сохраняем значение
	if err := mem.Store(offset, value); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Проверяем размер памяти
	if mem.Size() != 32 {
		t.Errorf("Expected size 32, got %d", mem.Size())
	}

	// Загружаем значение
	loaded, err := mem.Load(offset)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Cmp(value) != 0 {
		t.Errorf("Expected %s, got %s", value.String(), loaded.String())
	}
}

func TestMemory_StoreLoadMultiple(t *testing.T) {
	mem := NewMemory()

	// Сохраняем несколько значений
	values := []*big.Int{
		big.NewInt(0x1111),
		big.NewInt(0x2222),
		big.NewInt(0x3333),
	}

	for i, val := range values {
		offset := big.NewInt(int64(i * 32))
		if err := mem.Store(offset, val); err != nil {
			t.Fatalf("Store failed at index %d: %v", i, err)
		}
	}

	// Проверяем размер
	if mem.Size() != 96 {
		t.Errorf("Expected size 96, got %d", mem.Size())
	}

	// Загружаем и проверяем
	for i, expectedVal := range values {
		offset := big.NewInt(int64(i * 32))
		loaded, err := mem.Load(offset)
		if err != nil {
			t.Fatalf("Load failed at index %d: %v", i, err)
		}
		if loaded.Cmp(expectedVal) != 0 {
			t.Errorf("At index %d: expected %s, got %s", i, expectedVal.String(), loaded.String())
		}
	}
}

func TestMemory_StoreByteLoadByte(t *testing.T) {
	mem := NewMemory()

	// Сохраняем байты
	offset := big.NewInt(0)
	if err := mem.StoreByte(offset, 0x42); err != nil {
		t.Fatalf("StoreByte failed: %v", err)
	}

	offset2 := big.NewInt(1)
	if err := mem.StoreByte(offset2, 0xFF); err != nil {
		t.Fatalf("StoreByte failed: %v", err)
	}

	// Проверяем размер
	if mem.Size() != 2 {
		t.Errorf("Expected size 2, got %d", mem.Size())
	}

	// Загружаем байты
	byte1, err := mem.LoadByte(offset)
	if err != nil {
		t.Fatalf("LoadByte failed: %v", err)
	}
	if byte1 != 0x42 {
		t.Errorf("Expected 0x42, got 0x%02X", byte1)
	}

	byte2, err := mem.LoadByte(offset2)
	if err != nil {
		t.Fatalf("LoadByte failed: %v", err)
	}
	if byte2 != 0xFF {
		t.Errorf("Expected 0xFF, got 0x%02X", byte2)
	}
}

func TestMemory_Copy(t *testing.T) {
	mem := NewMemory()

	// Заполняем память данными
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	offset := big.NewInt(0)

	if err := mem.Set(offset, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Копируем часть данных
	copyOffset := big.NewInt(1)
	copyLength := big.NewInt(3)

	copied, err := mem.Copy(copyOffset, copyLength)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	expected := []byte{0x02, 0x03, 0x04}
	if len(copied) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(copied))
	}

	for i, b := range expected {
		if copied[i] != b {
			t.Errorf("At index %d: expected 0x%02X, got 0x%02X", i, b, copied[i])
		}
	}
}

func TestMemory_Set(t *testing.T) {
	mem := NewMemory()

	data := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	offset := big.NewInt(10)

	if err := mem.Set(offset, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Проверяем размер
	if mem.Size() != 14 {
		t.Errorf("Expected size 14, got %d", mem.Size())
	}

	// Проверяем данные через Copy
	copied, err := mem.Copy(offset, big.NewInt(int64(len(data))))
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if len(copied) != len(data) {
		t.Fatalf("Expected length %d, got %d", len(data), len(copied))
	}

	for i, b := range data {
		if copied[i] != b {
			t.Errorf("At index %d: expected 0x%02X, got 0x%02X", i, b, copied[i])
		}
	}
}

func TestMemory_OutOfBounds(t *testing.T) {
	mem := NewMemory()

	// Попытка загрузить из пустой памяти
	offset := big.NewInt(0)
	_, err := mem.Load(offset)
	if err != ErrMemoryOutOfBounds {
		t.Errorf("Expected ErrMemoryOutOfBounds, got %v", err)
	}

	// Попытка загрузить за пределами памяти
	mem.Store(offset, big.NewInt(42))
	largeOffset := big.NewInt(1000)
	_, err = mem.Load(largeOffset)
	if err != ErrMemoryOutOfBounds {
		t.Errorf("Expected ErrMemoryOutOfBounds, got %v", err)
	}
}

func TestMemory_Resize(t *testing.T) {
	mem := NewMemory()

	// Расширяем память
	if err := mem.Resize(100); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	if mem.Size() != 100 {
		t.Errorf("Expected size 100, got %d", mem.Size())
	}

	// Расширяем еще больше
	if err := mem.Resize(200); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	if mem.Size() != 200 {
		t.Errorf("Expected size 200, got %d", mem.Size())
	}

	// Попытка расширить за максимум
	largeSize := MaxMemorySize + 1
	if err := mem.Resize(uint64(largeSize)); err != ErrMemoryOutOfBounds {
		t.Errorf("Expected ErrMemoryOutOfBounds, got %v", err)
	}
}

func TestMemory_Clear(t *testing.T) {
	mem := NewMemory()

	// Заполняем память
	mem.Store(big.NewInt(0), big.NewInt(42))
	mem.Store(big.NewInt(32), big.NewInt(100))

	// Очищаем
	mem.Clear()

	if mem.Size() != 0 {
		t.Errorf("Expected size 0, got %d", mem.Size())
	}
}
