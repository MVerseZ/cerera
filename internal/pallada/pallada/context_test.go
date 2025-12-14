package pallada

import (
	"math/big"
	"testing"

	"github.com/cerera/internal/cerera/types"
)

func TestVM_Address(t *testing.T) {
	// Создаем тестовый адрес
	testAddress := types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234")

	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		testAddress,
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: ADDRESS (0x90)
	code := []byte{byte(ADDRESS), byte(STOP)}

	vm := NewVMWithContext(code, ctx)
	result, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty return data, got %x", result)
	}

	// Проверяем, что адрес на стеке
	if vm.stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", vm.stack.Len())
	}

	stackValue, err := vm.stack.Peek()
	if err != nil {
		t.Fatalf("Failed to peek stack: %v", err)
	}

	// Конвертируем обратно в адрес и проверяем
	addrFromStack := BigIntToAddress(stackValue)
	if addrFromStack != testAddress {
		t.Errorf("Expected address %x, got %x", testAddress, addrFromStack)
	}
}

func TestVM_Caller(t *testing.T) {
	callerAddress := types.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	contractAddress := types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234")

	ctx := NewContext(
		callerAddress,
		contractAddress,
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: CALLER (0x91)
	code := []byte{byte(CALLER), byte(STOP)}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if vm.stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", vm.stack.Len())
	}

	stackValue, err := vm.stack.Peek()
	if err != nil {
		t.Fatalf("Failed to peek stack: %v", err)
	}

	addrFromStack := BigIntToAddress(stackValue)
	if addrFromStack != callerAddress {
		t.Errorf("Expected caller address %x, got %x", callerAddress, addrFromStack)
	}
}

func TestVM_CallValue(t *testing.T) {
	testValue := big.NewInt(1000000) // 1 миллион wei

	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		testValue,
		nil,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: CALLVALUE (0x92)
	code := []byte{byte(CALLVALUE), byte(STOP)}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if vm.stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", vm.stack.Len())
	}

	stackValue, err := vm.stack.Peek()
	if err != nil {
		t.Fatalf("Failed to peek stack: %v", err)
	}

	if stackValue.Cmp(testValue) != 0 {
		t.Errorf("Expected value %d, got %d", testValue, stackValue)
	}
}

func TestVM_CallDataSize(t *testing.T) {
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		big.NewInt(0),
		testData,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: CALLDATASIZE (0x94)
	code := []byte{byte(CALLDATASIZE), byte(STOP)}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if vm.stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", vm.stack.Len())
	}

	stackValue, err := vm.stack.Peek()
	if err != nil {
		t.Fatalf("Failed to peek stack: %v", err)
	}

	expectedSize := big.NewInt(int64(len(testData)))
	if stackValue.Cmp(expectedSize) != 0 {
		t.Errorf("Expected size %d, got %d", expectedSize, stackValue)
	}
}

func TestVM_CallDataLoad(t *testing.T) {
	// Тестовые данные: первые 4 байта = 0x01020304
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		big.NewInt(0),
		testData,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: PUSH1 0 (offset), CALLDATALOAD (0x93)
	code := []byte{
		byte(PUSH1), 0x00, // offset = 0
		byte(CALLDATALOAD),
		byte(STOP),
	}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	if vm.stack.Len() != 1 {
		t.Fatalf("Expected stack length 1, got %d", vm.stack.Len())
	}

	stackValue, err := vm.stack.Peek()
	if err != nil {
		t.Fatalf("Failed to peek stack: %v", err)
	}

	// Проверяем, что значение не нулевое (данные загружены)
	if stackValue.Sign() == 0 {
		t.Error("Expected non-zero value from CALLDATALOAD")
	}

	// Проверяем, что значение содержит наши данные
	// big.Int.Bytes() возвращает байты без ведущих нулей в big-endian порядке
	// Поскольку мы скопировали данные в конец массива (младшие байты),
	// а big.Int.SetBytes() интерпретирует их как big-endian,
	// первые байты результата должны соответствовать началу testData
	bytes := stackValue.Bytes()

	// Проверяем, что первые байты соответствуют началу testData
	// (в big-endian первые байты - это старшие байты, которые мы скопировали в конец массива)
	if len(bytes) >= 4 {
		// Первые 4 байта должны быть нашими данными
		first4 := bytes[:4]
		for i := 0; i < 4; i++ {
			if first4[i] != testData[i] {
				t.Errorf("Byte at position %d: expected 0x%02x, got 0x%02x", i, testData[i], first4[i])
			}
		}
	} else {
		t.Errorf("Expected at least 4 bytes, got %d", len(bytes))
	}
}

func TestVM_CallDataCopy(t *testing.T) {
	testData := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		big.NewInt(0),
		testData,
		100000,
		big.NewInt(1),
		nil,
	)

	// Байткод: PUSH1 6 (size), PUSH1 0 (offset), PUSH1 0 (destOffset), CALLDATACOPY (0x95)
	code := []byte{
		byte(PUSH1), 0x06, // size = 6
		byte(PUSH1), 0x00, // offset = 0
		byte(PUSH1), 0x00, // destOffset = 0
		byte(CALLDATACOPY),
		byte(STOP),
	}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что данные скопированы в память
	memData, err := vm.memory.Get(0, 6)
	if err != nil {
		t.Fatalf("Failed to get memory: %v", err)
	}

	if len(memData) != 6 {
		t.Fatalf("Expected memory size 6, got %d", len(memData))
	}

	// Проверяем содержимое
	for i := 0; i < len(testData); i++ {
		if memData[i] != testData[i] {
			t.Errorf("Memory[%d]: expected 0x%02x, got 0x%02x", i, testData[i], memData[i])
		}
	}
}

func TestVM_GasTracking(t *testing.T) {
	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		big.NewInt(0),
		nil,
		1000, // Лимит газа
		big.NewInt(1),
		nil,
	)

	// Байткод: несколько операций ADDRESS
	code := []byte{
		byte(ADDRESS),
		byte(ADDRESS),
		byte(ADDRESS),
		byte(STOP),
	}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()
	if err != nil {
		t.Fatalf("VM execution failed: %v", err)
	}

	// Проверяем, что газ был использован
	gasUsed := vm.GasUsed()
	if gasUsed == 0 {
		t.Error("Expected gas to be used, got 0")
	}

	// Каждая операция ADDRESS стоит Base газа (2), всего 3 операции = 6 газа
	// Но также есть STOP, который может стоить газа
	if gasUsed < 6 {
		t.Errorf("Expected at least 6 gas used, got %d", gasUsed)
	}
}

func TestVM_OutOfGas(t *testing.T) {
	ctx := NewContext(
		types.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"),
		types.HexToAddress("0x1234567890123456789012345678901234567890123456789012345678901234"),
		big.NewInt(0),
		nil,
		5, // Очень маленький лимит газа
		big.NewInt(1),
		nil,
	)

	// Байткод: много операций ADDRESS (каждая стоит 2 газа)
	code := []byte{
		byte(ADDRESS),
		byte(ADDRESS),
		byte(ADDRESS), // Это должно вызвать ошибку нехватки газа
		byte(STOP),
	}

	vm := NewVMWithContext(code, ctx)
	_, err := vm.Run()

	// Должна быть ошибка нехватки газа
	if err == nil {
		t.Fatal("Expected out of gas error, got nil")
	}

	if !IsOutOfGas(err) {
		t.Errorf("Expected out of gas error, got: %v", err)
	}
}
