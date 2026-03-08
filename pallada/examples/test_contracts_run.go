//go:build contracts
// +build contracts

// Примеры смарт-контрактов для Pallada VM (байткод + вызовы).
// Pallada — стековая VM, похожая на EVM, но со своим набором опкодов.
// Запуск: go run -tags=contracts .
package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	secp256k1ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

type mockStorageContracts struct {
	storage map[string]*big.Int
}

func newMockStorageContracts() *mockStorageContracts {
	return &mockStorageContracts{storage: make(map[string]*big.Int)}
}

func (m *mockStorageContracts) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	if val, ok := m.storage[key.String()]; ok {
		return val, nil
	}
	return big.NewInt(0), nil
}

func (m *mockStorageContracts) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	m.storage[key.String()] = new(big.Int).Set(value)
	return nil
}

func sepContracts(title string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("=", 60))
}

// RunContractExamples запускает 5 примеров смарт-контрактов (байткод Pallada VM).
// Вызывается из main в test_run.go или отдельно.
func RunContractExamples() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Pallada VM — Примеры смарт-контрактов (байткод)     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Pallada VM — Примеры смарт-контрактов (байткод)     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	blockInfo := &pallada.BlockInfo{Number: 1, Timestamp: 1234567890, Hash: make([]byte, 32)}

	// -------------------------------------------------------------------------
	// Контракт 1: Counter — счётчик (storage[0]). get: возврат значения; increment: +1.
	// Диспетчеризация: если calldata пустой или только 1 байт 0 → get; иначе increment.
	// -------------------------------------------------------------------------
	sepContracts("Контракт 1: Counter (get / increment)")

	// Байткод:
	//   CALLDATASIZE, PUSH1 2, LT, PUSH1 getPC, JUMPI  -> если calldata < 2 байт, get
	//   increment: SLOAD(0), PUSH1 1, ADD, SSTORE(0), STOP
	//   get:       SLOAD(0), PUSH1 0, MSTORE, PUSH1 32, PUSH1 0, RETURN
	counterBytecode := []byte{
		0x36,       // CALLDATASIZE
		0x60, 0x02, // PUSH1 2
		0x10,       // LT
		0x60, 0x11, // PUSH1 17 (get JUMPDEST)
		0x57,       // JUMPI
		// increment (PC 7-16)
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD
		0x60, 0x01, // PUSH1 1
		0x01,       // ADD
		0x60, 0x00, // PUSH1 0
		0x55,       // SSTORE
		0x00,       // STOP
		// get (PC 17)
		0x5B,       // JUMPDEST
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD
		0x60, 0x00, // PUSH1 0 (offset)
		0x52,       // MSTORE
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xF3,       // RETURN
	}

	storageCounter := newMockStorageContracts()
	storageCounter.SetStorage(types.Address{}, big.NewInt(0), big.NewInt(10)) // начальное значение 10

	ctxGet := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), []byte{0}, 10000, big.NewInt(1), blockInfo, storageCounter,
	)
	vmGet := pallada.NewVM(counterBytecode, ctxGet)
	retGet, errGet := vmGet.Run()
	if errGet != nil {
		fmt.Printf("  get: ошибка %v\n", errGet)
	} else {
		val := new(big.Int).SetBytes(retGet)
		fmt.Printf("  get(): возврат = %s (газ: %d)\n", val.String(), vmGet.GasUsed())
	}

	ctxInc := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), []byte{0x01, 0x00}, 10000, big.NewInt(1), blockInfo, storageCounter,
	)
	vmInc := pallada.NewVM(counterBytecode, ctxInc)
	_, errInc := vmInc.Run()
	if errInc != nil {
		fmt.Printf("  increment(): ошибка %v\n", errInc)
	} else {
		after, _ := storageCounter.GetStorage(types.Address{}, big.NewInt(0))
		fmt.Printf("  increment(): storage[0] было 10, стало %s (газ: %d)\n", after.String(), vmInc.GasUsed())
	}

	// -------------------------------------------------------------------------
	// Контракт 2: SimpleToken — перевод между двумя «аккаунтами» (storage 0 и 1).
	// Calldata: 32 байта — сумма перевода. storage[0] -= amount, storage[1] += amount.
	// -------------------------------------------------------------------------
	sepContracts("Контракт 2: SimpleToken (transfer)")

	// PUSH1 0, SLOAD -> bal0
	// PUSH1 0, CALLDATALOAD -> amount
	// SUB (bal0 - amount) -> new0
	// PUSH1 0, SSTORE
	// PUSH1 1, SLOAD -> bal1
	// PUSH1 0, CALLDATALOAD -> amount
	// ADD -> new1
	// PUSH1 1, SSTORE
	// STOP
	tokenBytecode := []byte{
		0x60, 0x00, // PUSH1 0 (key sender)
		0x54,       // SLOAD
		0x60, 0x00, // PUSH1 0 (offset calldata)
		0x35,       // CALLDATALOAD
		0x03,       // SUB  (sender - amount)
		0x60, 0x00, // PUSH1 0
		0x55,       // SSTORE
		0x60, 0x01, // PUSH1 1 (key recipient)
		0x54,       // SLOAD
		0x60, 0x00, // PUSH1 0
		0x35,       // CALLDATALOAD
		0x01,       // ADD
		0x60, 0x01, // PUSH1 1
		0x55,       // SSTORE
		0x00,       // STOP
	}

	storageToken := newMockStorageContracts()
	storageToken.SetStorage(types.Address{}, big.NewInt(0), big.NewInt(1000)) // отправитель
	storageToken.SetStorage(types.Address{}, big.NewInt(1), big.NewInt(0))    // получатель

	amount := make([]byte, 32)
	big.NewInt(250).FillBytes(amount)

	ctxToken := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), amount, 100000, big.NewInt(1), blockInfo, storageToken,
	)
	vmToken := pallada.NewVM(tokenBytecode, ctxToken)
	_, errToken := vmToken.Run()
	if errToken != nil {
		fmt.Printf("  transfer(250): ошибка %v\n", errToken)
	} else {
		sender, _ := storageToken.GetStorage(types.Address{}, big.NewInt(0))
		recipient, _ := storageToken.GetStorage(types.Address{}, big.NewInt(1))
		fmt.Printf("  transfer(250): отправитель %s, получатель %s (газ: %d)\n", sender.String(), recipient.String(), vmToken.GasUsed())
	}

	// -------------------------------------------------------------------------
	// Контракт 3: Vault — «сейф». deposit(amount) по calldata; getBalance() при пустом calldata.
	// storage[0] = общий баланс.
	// -------------------------------------------------------------------------
	sepContracts("Контракт 3: Vault (deposit / getBalance)")

	// Диспетчер: CALLDATASIZE == 0 -> getBalance; иначе deposit(amount из calldata)
	// getBalance: SLOAD(0), MSTORE(0), RETURN(0, 32)
	// deposit:    CALLDATALOAD(0) = amount, SLOAD(0), ADD, SSTORE(0), STOP
	vaultBytecode := []byte{
		0x36,       // CALLDATASIZE
		0x15,       // ISZERO
		0x60, 0x10, // PUSH1 16 (getBalance JUMPDEST)
		0x57,       // JUMPI
		// deposit (PC 4-19)
		0x60, 0x00, // PUSH1 0
		0x35,       // CALLDATALOAD
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD
		0x01,       // ADD
		0x60, 0x00, // PUSH1 0
		0x55,       // SSTORE
		0x00,       // STOP
		// getBalance (PC 20: 0x5B = JUMPDEST at index 20)
		0x5B,       // JUMPDEST
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD
		0x60, 0x00, // PUSH1 0
		0x52,       // MSTORE
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xF3,       // RETURN
	}

	storageVault := newMockStorageContracts()

	depAmount := make([]byte, 32)
	big.NewInt(500).FillBytes(depAmount)
	ctxDep := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), depAmount, 10000, big.NewInt(1), blockInfo, storageVault,
	)
	vmDep := pallada.NewVM(vaultBytecode, ctxDep)
	_, _ = vmDep.Run()

	ctxGetBal := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), nil, 10000, big.NewInt(1), blockInfo, storageVault,
	)
	vmGetBal := pallada.NewVM(vaultBytecode, ctxGetBal)
	retBal, errBal := vmGetBal.Run()
	if errBal != nil {
		fmt.Printf("  getBalance: ошибка %v\n", errBal)
	} else {
		bal := new(big.Int).SetBytes(retBal)
		fmt.Printf("  deposit(500) затем getBalance() = %s (газ getBalance: %d)\n", bal.String(), vmGetBal.GasUsed())
	}

	// -------------------------------------------------------------------------
	// Контракт 4: EventEmitter — сохраняет число в storage и эмитит событие LOG1.
	// Calldata: 32 байта = значение. storage[0] = value, LOG1(topic=0xEE..EE, data=value).
	// -------------------------------------------------------------------------
	sepContracts("Контракт 4: EventEmitter (store + LOG1)")

	// CALLDATALOAD(0) -> value; PUSH1 0, SSTORE(value); value в память для LOG1
	// PUSH1 0, MSTORE(value уже на стеке — нужен DUP перед SSTORE)
	// value, PUSH1 0, SSTORE  -> нужен порядок: key, value для SSTORE. value на вершине, key под ним.
	// Итак: CALLDATALOAD(0) -> v; DUP1, PUSH1 0, SWAP1, SSTORE (key=0, value=v)
	// потом v в память: PUSH1 0, SWAP1, MSTORE (offset, value). Стек был [v]; после SSTORE пусто. Нужно снова v.
	// Проще: CALLDATALOAD(0), PUSH1 0, MSTORE (положим в память); PUSH1 0, SLOAD... нет, мы ещё не сохранили.
	// CALLDATALOAD(0) -> v; PUSH1 0, DUP2, SSTORE (v, 0, v -> pop v, pop 0, store v at 0). Остаётся v на стеке.
	// DUP1, PUSH1 0, MSTORE (v, 0 -> store v at 0). Стек пуст.
	// LOG1: offset=0, length=32, topic. Стек для LOG1: сначала топики (1 штука), потом offset, length. В коде: offset, length, topic — нет, в operations.go: offset pop first, length, then topics. So stack: length, offset, topic1. So: PUSH32 topic, PUSH1 32, PUSH1 0, LOG1.
	topicEE := make([]byte, 32)
	topicEE[31] = 0xEE
	eventEmitterBytecode := []byte{
		0x60, 0x00, // PUSH1 0
		0x35,       // CALLDATALOAD  -> value
		0x80,       // DUP1
		0x60, 0x00, // PUSH1 0
		0x90,       // SWAP1  -> 0, value
		0x55,       // SSTORE  (key=0, value)
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD   -> value (для LOG)
		0x60, 0x00, // PUSH1 0 (offset)
		0x52,       // MSTORE  (offset, value)
		0x7F,       // PUSH32 topic
	}
	eventEmitterBytecode = append(eventEmitterBytecode, topicEE...)
	eventEmitterBytecode = append(eventEmitterBytecode,
		0x60, 0x20, // PUSH1 32
		0x60, 0x00, // PUSH1 0
		0xA1,       // LOG1
		0x00,       // STOP
	)

	storageEvent := newMockStorageContracts()
	eventInput := make([]byte, 32)
	big.NewInt(12345).FillBytes(eventInput)
	ctxEvent := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), eventInput, 50000, big.NewInt(1), blockInfo, storageEvent,
	)
	vmEvent := pallada.NewVM(eventEmitterBytecode, ctxEvent)
	_, errEvent := vmEvent.Run()
	if errEvent != nil {
		fmt.Printf("  emit(12345): ошибка %v\n", errEvent)
	} else {
		logs := vmEvent.GetLogs()
		stored, _ := storageEvent.GetStorage(types.Address{}, big.NewInt(0))
		fmt.Printf("  emit(12345): storage[0] = %s, событий = %d (газ: %d)\n", stored.String(), len(logs), vmEvent.GasUsed())
		if len(logs) > 0 {
			fmt.Printf("  LOG1 topic[0] = %x, data = %x\n", logs[0].Topics[0].Bytes(), logs[0].Data)
		}
	}

	// -------------------------------------------------------------------------
	// Контракт 5: Owned — проверка владельца по ECDSA. storage[0] = owner (20 байт в 32).
	// Calldata: hash(32), v(32), r(32), s(32). ECRECOVER -> адрес; если == owner, storage[1]=1.
	// -------------------------------------------------------------------------
	sepContracts("Контракт 5: Owned (проверка подписи ECRECOVER)")

	privOwner, _ := secp256k1.GeneratePrivateKey()
	pubOwner := privOwner.PubKey()
	pubBytes := pubOwner.SerializeUncompressed()
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write(pubBytes[1:])
	ownerAddr := keccak.Sum(nil)[12:] // 20 байт
	ownerBig := make([]byte, 32)
	copy(ownerBig[12:], ownerAddr)

	msgHash := make([]byte, 32)
	copy(msgHash, []byte("Pallada owned contract"))
	sig := secp256k1ecdsa.SignCompact(privOwner, msgHash, false)
	vByte := sig[0]
	rBytes := sig[1:33]
	sBytes := sig[33:65]

	// Байткод: PUSH32 s, PUSH32 r, PUSH1 v, PUSH32 hash -> ECRECOVER
	// затем PUSH1 0, SLOAD -> owner; EQ; ISZERO, PUSH1 fail, JUMPI; PUSH1 1, PUSH1 1, SSTORE, STOP
	// fail: REVERT(0,0)
	ownedBytecode := []byte{0x7F}
	ownedBytecode = append(ownedBytecode, sBytes...)
	ownedBytecode = append(ownedBytecode, 0x7F)
	ownedBytecode = append(ownedBytecode, rBytes...)
	ownedBytecode = append(ownedBytecode, 0x60, vByte)
	ownedBytecode = append(ownedBytecode, 0x7F)
	ownedBytecode = append(ownedBytecode, msgHash...)
	ownedBytecode = append(ownedBytecode,
		0x23,       // ECRECOVER -> recovered on stack
		0x60, 0x00, // PUSH1 0
		0x54,       // SLOAD  -> owner (32 bytes)
		0x14,       // EQ
		0x15,       // ISZERO  -> 1 if not eq
		0x60, 0x2C, // PUSH1 44 (fail)
		0x57,       // JUMPI
		// success: set storage[1] = 1
		0x60, 0x01, // PUSH1 1
		0x60, 0x01, // PUSH1 1
		0x55,       // SSTORE
		0x00,       // STOP
		// fail (PC 44)
		0x5B,       // JUMPDEST
		0x60, 0x00, // PUSH1 0
		0x60, 0x00, // PUSH1 0
		0xFD,       // REVERT
	)

	storageOwned := newMockStorageContracts()
	storageOwned.SetStorage(types.Address{}, big.NewInt(0), new(big.Int).SetBytes(ownerBig))

	calldataOwned := append(append(append(msgHash, make([]byte, 32)...), rBytes...), sBytes...)
	calldataOwned = append(calldataOwned[:32], append([]byte{vByte}, calldataOwned[32:]...)...) // v в вторых 32 байтах
	// Формат: hash(32), v(32) — только младший байт значим, r(32), s(32)
	calldataOwned = make([]byte, 0, 128)
	calldataOwned = append(calldataOwned, msgHash...)
	vPad := make([]byte, 32)
	vPad[31] = vByte
	calldataOwned = append(calldataOwned, vPad...)
	calldataOwned = append(calldataOwned, rBytes...)
	calldataOwned = append(calldataOwned, sBytes...)

	// Контракт не читает calldata для ECRECOVER — мы захардкодили hash,v,r,s в байткод для простоты.
	// В реальности контракт бы делал CALLDATALOAD(0), CALLDATALOAD(32) и т.д. Здесь уже подставили конкретные значения.
	ctxOwned := pallada.NewContextWithStorage(
		types.Address{}, types.Address{}, big.NewInt(0), nil, 100000, big.NewInt(1), blockInfo, storageOwned,
	)
	vmOwned := pallada.NewVM(ownedBytecode, ctxOwned)
	_, errOwned := vmOwned.Run()
	if errOwned != nil {
		fmt.Printf("  verify(owner): ошибка %v\n", errOwned)
	} else {
		unlocked, _ := storageOwned.GetStorage(types.Address{}, big.NewInt(1))
		fmt.Printf("  verify(owner): storage[1] (unlocked) = %s (газ: %d)\n", unlocked.String(), vmOwned.GasUsed())
		fmt.Printf("  Владелец верифицирован по ECRECOVER, контракт разблокирован.\n")
	}

	// -------------------------------------------------------------------------
	sepContracts("Итог")
	fmt.Println("  1. Counter   — get/increment по calldata")
	fmt.Println("  2. SimpleToken — transfer между storage[0] и storage[1]")
	fmt.Println("  3. Vault     — deposit/getBalance")
	fmt.Println("  4. EventEmitter — store + LOG1")
	fmt.Println("  5. Owned     — ECRECOVER и сравнение с storage[0]")
	fmt.Println("\n  Все контракты — нативный байткод Pallada VM.")
}

func main() {
	RunContractExamples()
}
