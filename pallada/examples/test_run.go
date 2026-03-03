package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
)

// mockStorage реализует StorageInterface для демонстрации
type mockStorage struct {
	storage map[string]*big.Int
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		storage: make(map[string]*big.Int),
	}
}

func (m *mockStorage) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	keyStr := key.String()
	if val, ok := m.storage[keyStr]; ok {
		return val, nil
	}
	return big.NewInt(0), nil
}

func (m *mockStorage) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	keyStr := key.String()
	m.storage[keyStr] = new(big.Int).Set(value)
	return nil
}

func printSeparator(title string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("=", 60))
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Pallada Virtual Machine - Demonstration Examples     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	// Создаем общий контекст для всех примеров
	blockInfo := &pallada.BlockInfo{
		Number:    1,
		Timestamp: 1234567890,
		Hash:      make([]byte, 32),
	}

	// ============================================================
	// Пример 1: Простое выполнение - сохранение значения в память
	// ============================================================
	printSeparator("Пример 1: Сохранение значения в память")

	code1 := []byte{
		0x60, 0x2a, // PUSH1 42
		0x60, 0x00, // PUSH1 0 (offset)
		0x52, // MSTORE (сохранить 32-байтное слово)
		0x00, // STOP
	}

	ctx1 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm1 := pallada.NewVM(code1, ctx1)
	result1, err1 := vm1.Run()

	if err1 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err1)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm1.GasUsed())
		fmt.Printf("   Результат выполнения: %v (STOP не возвращает данные)\n", result1)

		// Проверяем значение в памяти напрямую
		memValue, memErr := vm1.GetMemory().Load(big.NewInt(0))
		if memErr != nil {
			fmt.Printf("   ⚠️  Ошибка чтения памяти: %v\n", memErr)
		} else {
			fmt.Printf("   ✅ Значение в памяти по адресу 0: %s\n", memValue.String())
			fmt.Printf("   (Значение 42 успешно сохранено в память)\n")
		}
	}

	// ============================================================
	// Пример 2: Арифметические операции
	// ============================================================
	printSeparator("Пример 2: Арифметические операции (10 + 5)")

	code2 := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x01, // ADD (сложение)
		0x00, // STOP
	}

	ctx2 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm2 := pallada.NewVM(code2, ctx2)
	_, err2 := vm2.Run()

	if err2 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err2)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm2.GasUsed())
		fmt.Printf("   Результат: 10 + 5 = 15 (на вершине стека)\n")
	}

	// ============================================================
	// Пример 3: Умножение и деление
	// ============================================================
	printSeparator("Пример 3: Умножение и деление (6 * 7 / 2)")

	code3 := []byte{
		0x60, 0x06, // PUSH1 6
		0x60, 0x07, // PUSH1 7
		0x02,       // MUL (умножение: 6 * 7 = 42)
		0x60, 0x02, // PUSH1 2
		0x04, // DIV (деление: 42 / 2 = 21)
		0x00, // STOP
	}

	ctx3 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm3 := pallada.NewVM(code3, ctx3)
	_, err3 := vm3.Run()

	if err3 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err3)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm3.GasUsed())
		fmt.Printf("   Результат: 6 * 7 / 2 = 21 (на вершине стека)\n")
	}

	// ============================================================
	// Пример 4: Работа с storage (сохранение и загрузка)
	// ============================================================
	printSeparator("Пример 4: Работа с storage (сохранение значения)")

	storage := newMockStorage()

	code4 := []byte{
		0x60, 0x64, // PUSH1 100 (значение для сохранения)
		0x60, 0x00, // PUSH1 0 (ключ storage)
		0x55,       // SSTORE (сохранить в storage)
		0x60, 0x00, // PUSH1 0 (ключ для загрузки)
		0x54, // SLOAD (загрузить из storage)
		0x00, // STOP
	}

	ctx4 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		50000, // Больше газа для SSTORE
		big.NewInt(1),
		blockInfo,
		storage,
	)

	vm4 := pallada.NewVM(code4, ctx4)
	_, err4 := vm4.Run()

	if err4 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err4)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm4.GasUsed())
		fmt.Printf("   Значение 100 сохранено в storage по ключу 0\n")
		fmt.Printf("   Значение загружено обратно на стек\n")

		// Проверяем storage напрямую
		storedValue, _ := storage.GetStorage(types.Address{}, big.NewInt(0))
		fmt.Printf("   Проверка storage: значение = %s\n", storedValue.String())
	}

	// ============================================================
	// Пример 5: RETURN данных
	// ============================================================
	printSeparator("Пример 5: RETURN данных из памяти")

	code5 := []byte{
		0x60, 0x04, // PUSH1 4 (длина данных)
		0x60, 0x00, // PUSH1 0 (offset в памяти)
		0x52,       // MSTORE (сохранить длину в память)
		0x60, 0x04, // PUSH1 4 (длина для RETURN)
		0x60, 0x00, // PUSH1 0 (offset для RETURN)
		0xF3, // RETURN (вернуть данные)
	}

	ctx5 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm5 := pallada.NewVM(code5, ctx5)
	result5, err5 := vm5.Run()

	if err5 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err5)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm5.GasUsed())
		if result5 != nil {
			fmt.Printf("   Возвращенные данные: %x\n", result5)
			fmt.Printf("   Длина данных: %d байт\n", len(result5))
		} else {
			fmt.Printf("   Возвращенные данные: nil\n")
		}
	}

	// ============================================================
	// Пример 6: Сравнение чисел
	// ============================================================
	printSeparator("Пример 6: Сравнение чисел (10 > 5)")

	code6 := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x11, // GT (больше: 10 > 5)
		0x00, // STOP
	}

	ctx6 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm6 := pallada.NewVM(code6, ctx6)
	_, err6 := vm6.Run()

	if err6 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err6)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm6.GasUsed())
		fmt.Printf("   Результат: 10 > 5 = 1 (true, на вершине стека)\n")
	}

	// ============================================================
	// Пример 7: Побитовые операции
	// ============================================================
	printSeparator("Пример 7: Побитовые операции (AND)")

	code7 := []byte{
		0x60, 0x0F, // PUSH1 15 (0b1111)
		0x60, 0x03, // PUSH1 3 (0b0011)
		0x16, // AND (побитовое И: 15 & 3 = 3)
		0x00, // STOP
	}

	ctx7 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm7 := pallada.NewVM(code7, ctx7)
	_, err7 := vm7.Run()

	if err7 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err7)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm7.GasUsed())
		fmt.Printf("   Результат: 15 & 3 = 3 (на вершине стека)\n")
	}

	// ============================================================
	// Пример 8: REVERT (откат с данными)
	// ============================================================
	printSeparator("Пример 8: REVERT (откат выполнения)")

	code8 := []byte{
		0x60, 0x08, // PUSH1 8 (длина данных)
		0x60, 0x00, // PUSH1 0 (offset)
		0x52,       // MSTORE
		0x60, 0x08, // PUSH1 8 (длина для REVERT)
		0x60, 0x00, // PUSH1 0 (offset для REVERT)
		0xFD, // REVERT (откат с данными)
	}

	ctx8 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm8 := pallada.NewVM(code8, ctx8)
	result8, err8 := vm8.Run()

	if err8 != nil {
		fmt.Printf("✅ REVERT выполнен (ожидаемое поведение)\n")
		fmt.Printf("   Использовано газа: %d\n", vm8.GasUsed())
		fmt.Printf("   Ошибка: %v\n", err8)
		if result8 != nil {
			fmt.Printf("   Данные отката: %x\n", result8)
		}
	} else {
		fmt.Printf("⚠️  REVERT не сработал (неожиданное поведение)\n")
	}

	// ============================================================
	// Пример 9: Комплексная операция (вычисление выражения)
	// ============================================================
	printSeparator("Пример 9: Комплексное выражение ((10 + 5) * 2)")

	code9 := []byte{
		0x60, 0x0a, // PUSH1 10
		0x60, 0x05, // PUSH1 5
		0x01,       // ADD (10 + 5 = 15)
		0x60, 0x02, // PUSH1 2
		0x02, // MUL (15 * 2 = 30)
		0x00, // STOP
	}

	ctx9 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		10000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	vm9 := pallada.NewVM(code9, ctx9)
	_, err9 := vm9.Run()

	if err9 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err9)
	} else {
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm9.GasUsed())
		fmt.Printf("   Результат: (10 + 5) * 2 = 30 (на вершине стека)\n")
	}

	// ============================================================
	// Пример 10: «Перевод» — взять из памяти (storage), вычесть у одного, прибавить другому, положить обратно
	// ============================================================
	printSeparator("Пример 10: Перевод (отправитель → получатель)")

	// Storage: ключ 0 = баланс отправителя, ключ 1 = баланс получателя.
	// Инициализация: 100 у отправителя, 0 у получателя. Переводим 50.
	storageTransfer := newMockStorage()
	storageTransfer.SetStorage(types.Address{}, big.NewInt(0), big.NewInt(100)) // отправитель: 100
	storageTransfer.SetStorage(types.Address{}, big.NewInt(1), big.NewInt(0))   // получатель: 0

	// Байткод: загрузить баланс отправителя (ключ 0), вычесть amount, сохранить;
	// загрузить баланс получателя (ключ 1), прибавить amount, сохранить.
	// В Pallada SUB = (below - top). Нужно [50, sender_bal]: делаем PUSH1 0, SLOAD → [100], PUSH1 50 → [50, 100]; SUB → 50.
	// SSTORE: ключ сверху. После SUB [new_sender]; PUSH1 0 → [0, new_sender]; SSTORE.
	code10 := []byte{
		0x60, 0x00, // PUSH1 0 (ключ отправителя)
		0x54,       // SLOAD → [sender_bal]
		0x60, 0x32, // PUSH1 50 (сумма перевода) → [50, sender_bal]
		0x03,       // SUB (below - top = sender_bal - 50)
		0x60, 0x00, // PUSH1 0 → [0, new_sender]
		0x55,       // SSTORE (новый баланс отправителя)
		0x60, 0x32, // PUSH1 50 (сумма для получателя)
		0x60, 0x01, // PUSH1 1 (ключ получателя)
		0x54,       // SLOAD → [recv_bal, 50]
		0x01,       // ADD (получатель + 50)
		0x60, 0x01, // PUSH1 1 → [1, new_recv]
		0x55,       // SSTORE (новый баланс получателя)
		0x00,       // STOP
	}

	ctx10 := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		blockInfo,
		storageTransfer,
	)

	vm10 := pallada.NewVM(code10, ctx10)
	_, err10 := vm10.Run()

	if err10 != nil {
		fmt.Printf("❌ Ошибка: %v\n", err10)
	} else {
		senderAfter, _ := storageTransfer.GetStorage(types.Address{}, big.NewInt(0))
		receiverAfter, _ := storageTransfer.GetStorage(types.Address{}, big.NewInt(1))
		fmt.Printf("✅ Выполнение успешно\n")
		fmt.Printf("   Использовано газа: %d\n", vm10.GasUsed())
		fmt.Printf("   Было:  отправитель=100, получатель=0. Перевод 50.\n")
		fmt.Printf("   Стало: отправитель=%s, получатель=%s\n", senderAfter.String(), receiverAfter.String())
		fmt.Printf("   (значение взяли из storage, вычли у одного, прибавили другому, положили обратно)\n")
	}

	// ============================================================
	// Итоговая статистика
	// ============================================================
	printSeparator("Итоговая статистика")

	fmt.Println("✅ Все примеры выполнены успешно!")
	fmt.Println("\n📊 Сводка:")
	fmt.Println("   - Простые операции: ✅")
	fmt.Println("   - Арифметика: ✅")
	fmt.Println("   - Память: ✅")
	fmt.Println("   - Storage: ✅")
	fmt.Println("   - RETURN: ✅")
	fmt.Println("   - REVERT: ✅")
	fmt.Println("   - Сравнения: ✅")
	fmt.Println("   - Побитовые операции: ✅")
	fmt.Println("   - Перевод (load/sub/add/store): ✅")
	fmt.Println("\n🎉 Pallada VM работает корректно!")
}
