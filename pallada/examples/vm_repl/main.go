//go:build !contracts
// +build !contracts

// vm_repl — интерактивный REPL для виртуальной машины Pallada.
// Запускает VM и принимает команды: базовые арифметические операции и выход.
package main

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
)

// Опкоды Pallada (EVM-подобные)
const (
	opSTOP    = 0x00
	opADD     = 0x01
	opMUL     = 0x02
	opSUB     = 0x03
	opDIV     = 0x04
	opMOD     = 0x06
	opMSTORE  = 0x52
	opPUSH1   = 0x60
	opSWAP1   = 0x90
	opRETURN  = 0xF3
)

type mockStorage struct {
	storage map[string]*big.Int
}

func newMockStorage() *mockStorage {
	return &mockStorage{storage: make(map[string]*big.Int)}
}

func (m *mockStorage) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	if val, ok := m.storage[key.String()]; ok {
		return val, nil
	}
	return big.NewInt(0), nil
}

func (m *mockStorage) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	m.storage[key.String()] = new(big.Int).Set(value)
	return nil
}

// push1Bytecode добавляет PUSH1 <v> в код. v должно быть 0–255.
func push1Bytecode(code []byte, v byte) []byte {
	return append(code, opPUSH1, v)
}

// buildBinaryOp строит байткод: PUSH a, PUSH b, op, запись результата в память, RETURN.
// Операнды a, b и результат интерпретируются как uint8 (0–255).
func buildBinaryOp(a, b byte, op byte) []byte {
	code := []byte{}
	code = push1Bytecode(code, a)
	code = push1Bytecode(code, b)
	code = append(code, op)
	// Результат на вершине стека. MSTORE в Pallada: первый Pop() = offset, второй = value. Нужен стек [offset, value].
	// Стек после op: [result]. Пушим 0: [0, result] — уже подходит для MSTORE.
	code = push1Bytecode(code, 0)
	code = append(code, opMSTORE)
	// RETURN(offset, size): первый Pop() = offset, второй Pop() = length. Стек: offset на вершине, под ним length.
	code = push1Bytecode(code, 32) // length (будет вторым при Pop)
	code = push1Bytecode(code, 0)  // offset (на вершине)
	code = append(code, opRETURN)
	return code
}

func runBinaryOp(a, b byte, op byte, ctx *pallada.Context) (*big.Int, uint64, error) {
	code := buildBinaryOp(a, b, op)
	vm := pallada.NewVM(code, ctx)
	result, err := vm.Run()
	if err != nil {
		return nil, 0, err
	}
	if len(result) == 0 {
		return big.NewInt(0), vm.GasUsed(), nil
	}
	// 32 байта big-endian -> big.Int
	z := new(big.Int).SetBytes(result)
	return z, vm.GasUsed(), nil
}

func parseUint8(s string) (byte, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n > 255 {
		return 0, fmt.Errorf("число должно быть от 0 до 255, получено %d", n)
	}
	return byte(n), nil
}

func main() {
	blockInfo := &pallada.BlockInfo{
		Number:    1,
		Timestamp: 1234567890,
		Hash:      make([]byte, 32),
	}
	ctx := pallada.NewContextWithStorage(
		types.Address{},
		types.Address{},
		big.NewInt(0),
		nil,
		100000,
		big.NewInt(1),
		blockInfo,
		newMockStorage(),
	)

	fmt.Println("Pallada VM — интерактивный REPL")
	fmt.Println("Команды: add a b | sub a b | mul a b | div a b | mod a b")
	fmt.Println("Числа a, b — от 0 до 255. Выход: exit или quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("vm> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToLower(parts[0])
		if cmd == "exit" || cmd == "quit" {
			fmt.Println("Выход.")
			break
		}
		if len(parts) != 3 {
			fmt.Println("Ожидается: операция a b (например: add 10 5)")
			continue
		}
		a, err1 := parseUint8(parts[1])
		b, err2 := parseUint8(parts[2])
		if err1 != nil {
			fmt.Println("Первый аргумент:", err1)
			continue
		}
		if err2 != nil {
			fmt.Println("Второй аргумент:", err2)
			continue
		}
		var op byte
		switch cmd {
		case "add":
			op = opADD
		case "sub":
			op = opSUB
		case "mul":
			op = opMUL
		case "div":
			op = opDIV
			if b == 0 {
				fmt.Println("Ошибка: деление на ноль")
				continue
			}
		case "mod":
			op = opMOD
			if b == 0 {
				fmt.Println("Ошибка: деление на ноль (mod)")
				continue
			}
		default:
			fmt.Printf("Неизвестная команда: %q. Доступны: add, sub, mul, div, mod\n", cmd)
			continue
		}
		res, gas, err := runBinaryOp(a, b, op, ctx)
		if err != nil {
			fmt.Println("Выполнение VM:", err)
			continue
		}
		fmt.Printf("  результат: %s   (газ: %d)\n", res.String(), gas)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Чтение ввода:", err)
		os.Exit(1)
	}
}
