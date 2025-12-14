package pallada

import (
	"math/big"
	"testing"
)

func TestMemory_SetGet(t *testing.T) {
	mem := NewMemory()

	// Тест Set
	data := []byte{0x01, 0x02, 0x03, 0x04}
	err := mem.Set(0, uint64(len(data)), data)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Тест Get
	result, err := mem.Get(0, uint64(len(data)))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != len(data) {
		t.Fatalf("Expected length %d, got %d", len(data), len(result))
	}

	for i := range data {
		if result[i] != data[i] {
			t.Fatalf("Expected %x, got %x at index %d", data[i], result[i], i)
		}
	}
}

func TestMemory_Set32Get32(t *testing.T) {
	mem := NewMemory()

	// Тест Set32
	value := big.NewInt(0x1234567890ABCDEF)
	err := mem.Set32(0, value)
	if err != nil {
		t.Fatalf("Set32 failed: %v", err)
	}

	// Тест Get32
	result, err := mem.Get32(0)
	if err != nil {
		t.Fatalf("Get32 failed: %v", err)
	}

	if result.Cmp(value) != 0 {
		t.Fatalf("Expected %v, got %v", value, result)
	}
}

func TestMemory_OutOfBounds(t *testing.T) {
	mem := NewMemory()

	// Попытка записать за пределами памяти
	data := make([]byte, 100)
	err := mem.Set(MaxMemorySize, 100, data)
	if err != ErrMemoryOutOfBounds {
		t.Fatalf("Expected ErrMemoryOutOfBounds, got %v", err)
	}

	// Попытка прочитать за пределами памяти
	_, err = mem.Get(MaxMemorySize, 100)
	if err != ErrMemoryOutOfBounds {
		t.Fatalf("Expected ErrMemoryOutOfBounds, got %v", err)
	}
}

func TestMemory_Expand(t *testing.T) {
	mem := NewMemory()

	// Записываем данные в начало
	data1 := []byte{0xAA, 0xBB}
	err := mem.Set(0, 2, data1)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Записываем данные далеко от начала (память должна расшириться)
	data2 := []byte{0xCC, 0xDD}
	err = mem.Set(1000, 2, data2)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Проверяем, что первая запись не потерялась
	result1, err := mem.Get(0, 2)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result1[0] != 0xAA || result1[1] != 0xBB {
		t.Fatalf("First data corrupted")
	}

	// Проверяем вторую запись
	result2, err := mem.Get(1000, 2)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result2[0] != 0xCC || result2[1] != 0xDD {
		t.Fatalf("Second data corrupted")
	}
}

func TestMemory_ReadUninitialized(t *testing.T) {
	mem := NewMemory()

	// Читаем из неинициализированной области (должны получить нули)
	result, err := mem.Get(100, 10)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(result) != 10 {
		t.Fatalf("Expected length 10, got %d", len(result))
	}

	for i := range result {
		if result[i] != 0 {
			t.Fatalf("Expected 0 at index %d, got %x", i, result[i])
		}
	}
}

func TestMemory_LargeValue(t *testing.T) {
	mem := NewMemory()

	// Создаем большое значение (256 бит)
	largeValue := new(big.Int)
	largeValue.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", 16)

	err := mem.Set32(0, largeValue)
	if err != nil {
		t.Fatalf("Set32 failed: %v", err)
	}

	result, err := mem.Get32(0)
	if err != nil {
		t.Fatalf("Get32 failed: %v", err)
	}

	if result.Cmp(largeValue) != 0 {
		t.Fatalf("Expected %v, got %v", largeValue, result)
	}
}

func TestMemory_Overwrite(t *testing.T) {
	mem := NewMemory()

	// Записываем первое значение
	value1 := big.NewInt(0x11111111)
	err := mem.Set32(0, value1)
	if err != nil {
		t.Fatalf("Set32 failed: %v", err)
	}

	// Перезаписываем тем же адресом
	value2 := big.NewInt(0x22222222)
	err = mem.Set32(0, value2)
	if err != nil {
		t.Fatalf("Set32 failed: %v", err)
	}

	// Проверяем, что записалось второе значение
	result, err := mem.Get32(0)
	if err != nil {
		t.Fatalf("Get32 failed: %v", err)
	}

	if result.Cmp(value2) != 0 {
		t.Fatalf("Expected %v, got %v", value2, result)
	}
}

func TestMemory_Len(t *testing.T) {
	mem := NewMemory()

	// Изначально память пустая
	if mem.Len() != 0 {
		t.Fatalf("Expected length 0, got %d", mem.Len())
	}

	// Записываем данные
	data := []byte{0x01, 0x02, 0x03}
	err := mem.Set(0, 3, data)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if mem.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", mem.Len())
	}

	// Записываем дальше (память расширяется)
	err = mem.Set(100, 5, []byte{0x04, 0x05, 0x06, 0x07, 0x08})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if mem.Len() != 105 {
		t.Fatalf("Expected length 105, got %d", mem.Len())
	}
}
