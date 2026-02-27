// accountlen — утилита для проверки длины сериализации (Bytes()) разных типов аккаунтов.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const (
	TypeNormal   = 0
	TypeStaking  = 1
	TypeVoting   = 2
	TypeFaucet   = 3
	TypeCoinbase = 4
)

func main() {
	fmt.Println("=== Длина байтов аккаунтов (StateAccount.Bytes()) ===")
	fmt.Println()

	// 1) Минимальный обычный аккаунт (нет inputs)
	minNorm := makeMinimalAccount(TypeNormal, "minimal normal")
	printRow("Минимальный (normal, без inputs)", minNorm)

	// 2) Системные адреса (Base, Faucet, CoreStaking) — тоже через GenerateAccount
	baseAcc := makeMinimalAccount(TypeNormal, "")
	baseAcc.Address = address.HexToAddress(account.BaseAddressHex)
	printRow("Base-адрес (0xf..0f)", baseAcc)

	faucetAcc := makeMinimalAccount(TypeFaucet, "")
	faucetAcc.Address = address.HexToAddress(account.FaucetAddressHex)
	printRow("Faucet (0xf..0a)", faucetAcc)

	coreStakingAcc := makeMinimalAccount(TypeStaking, "")
	coreStakingAcc.Address = address.HexToAddress(account.CoreStakingAddressHex)
	printRow("CoreStaking (0xf..0b)", coreStakingAcc)

	// 4) Разные типы с одинаковым «телом»
	for t := byte(0); t <= 4; t++ {
		acc := makeMinimalAccount(TypeNormal, "")
		acc.Type = t
		name := typeName(t)
		printRow("Тип "+name, acc)
	}

	// 5) Реальный сгенерированный аккаунт (ключ + адрес)
	realAcc := makeRealisticAccount()
	printRow("Реальный (GenerateAccount)", realAcc)

	// 6) Vault-style (bip39 + KeyHash, как в vault.Create)
	vaultAcc := makeVaultStyleAccount()
	if vaultAcc != nil {
		printRow("Vault-style (bip39+KeyHash)", vaultAcc)
	}

	// 7) Разный размер Bloom
	bloom10 := makeMinimalAccount(TypeNormal, "")
	bloom10.Bloom = make([]byte, 10)
	printRow("Bloom 10 байт", bloom10)
	bloom32 := makeMinimalAccount(TypeNormal, "")
	bloom32.Bloom = make([]byte, 32)
	printRow("Bloom 32 байт", bloom32)

	fmt.Println("\n--- Сводка по полям (фиксированные размеры) ---")
	fmt.Println("  Type: 1 байт")
	fmt.Println("  Address: 4 (len) + 32 байт (фиксированная)")
	fmt.Println("  Passphrase: 32 байт")
	fmt.Println("  Bloom: 4 (len) + 10 байт (фиксированная)")
	fmt.Println("  Nonce: 8 байт")
	fmt.Println("  Root: 32 байт")
	fmt.Println("  KeyHash: 32 байт (в StateAccountData)")
	fmt.Println("  Status: 1 байт")
	fmt.Println("  Balance: 4 (len) + N байт")

	fmt.Println()
	printZKExample()
}

// printZKExample — пример ZK для аккаунта: коммит к типу аккаунта и ключу (адресу), раскрытие ключа.
// C = H(nonce || accountType || address). Раскрытие = (nonce, type, address); проверка пересчётом хеша.
func printZKExample() {
	fmt.Println("=== Пример ZK: коммит к типу аккаунта и раскрытие ключа ===")
	fmt.Println()
	fmt.Println("  Идея: коммитим тип аккаунта (normal/staking/...) и ключ (адрес). Позже раскрываем ключ")
	fmt.Println("  и тип — верификатор проверяет, что они совпадают с коммитом.")
	fmt.Println()

	priv, err := crypto.GenerateAccount()
	if err != nil {
		fmt.Println("  GenerateAccount:", err)
		return
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	keyBytes := addr.Bytes()
	accountType := byte(TypeStaking) // 1 = staking

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		fmt.Println("  rand.Read:", err)
		return
	}

	// Commit: C = H(nonce || accountType || key)
	commit := zkAccountCommit(nonce, accountType, keyBytes)
	fmt.Println("  1) Коммит (тип аккаунта + ключ)")
	fmt.Printf("     Тип аккаунта:  %s (%d)\n", typeName(accountType), accountType)
	fmt.Printf("     Ключ (адрес):  %s...  (%d байт)\n", hex.EncodeToString(keyBytes)[:24], len(keyBytes))
	fmt.Printf("     Nonce:         %d байт\n", len(nonce))
	fmt.Printf("     Формула:       C = SHA256(nonce || type || address)\n")
	fmt.Printf("     Коммит C:      %s...  →  %d байт\n", hex.EncodeToString(commit)[:24], len(commit))
	fmt.Println()

	// Opening = раскрытие ключа (nonce + type + key)
	opening := zkAccountOpening(nonce, accountType, keyBytes)
	fmt.Println("  2) Раскрытие ключа (proof)")
	fmt.Printf("     Opening = nonce + type + address:  %d + 1 + %d = %d байт\n", len(nonce), len(keyBytes), len(opening))
	fmt.Println()

	// Verify
	ok := zkAccountVerify(commit, opening)
	fmt.Println("  3) Проверка")
	fmt.Printf("     По opening извлекаем type и адрес, считаем SHA256(nonce||type||address), сравниваем с C  →  OK: %v\n", ok)
	fmt.Println()
	fmt.Println("  Итог: коммит привязывает тип и ключ; раскрытие ключа доказывает соответствие коммиту.")
}

func zkAccountCommit(nonce []byte, accountType byte, keyBytes []byte) []byte {
	h := sha256.New()
	h.Write(nonce)
	h.Write([]byte{accountType})
	h.Write(keyBytes)
	return h.Sum(nil)
}

func zkAccountOpening(nonce []byte, accountType byte, keyBytes []byte) []byte {
	b := make([]byte, 0, len(nonce)+1+len(keyBytes))
	b = append(b, nonce...)
	b = append(b, accountType)
	b = append(b, keyBytes...)
	return b
}

func zkAccountVerify(commit, opening []byte) bool {
	if len(opening) < 32+1+32 {
		return false
	}
	nonce := opening[:32]
	accountType := opening[32]
	keyBytes := opening[33:]
	recomputed := zkAccountCommit(nonce, accountType, keyBytes)
	return bytes.Equal(recomputed, commit)
}

func typeName(t byte) string {
	switch t {
	case TypeNormal:
		return "normal"
	case TypeStaking:
		return "staking"
	case TypeVoting:
		return "voting"
	case TypeFaucet:
		return "faucet"
	case TypeCoinbase:
		return "coinbase"
	default:
		return "unknown"
	}
}

func makeMinimalAccount(accType byte, _ string) *account.StateAccount {
	priv, err := crypto.GenerateAccount()
	if err != nil {
		panic(err)
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	return makeMinimalAccountWithAddress(accType, addr)
}

func makeMinimalAccountWithAddress(accType byte, addr address.Address) *account.StateAccount {
	acc := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: addr,
			Nonce:   1,
			Root:    common.Hash{},
			KeyHash: common.Hash{},
		},
		Bloom:      []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Status:     0,
		Type:       accType,
		Passphrase: common.Hash{},
		Inputs:     &account.Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}
	acc.SetBalance(0)
	return acc
}

func makeRealisticAccount() *account.StateAccount {
	priv, err := crypto.GenerateAccount()
	if err != nil {
		panic(err)
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	acc := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: addr,
			Nonce:   1,
			Root:    common.Hash(addr.Bytes()),
			KeyHash: common.Hash{},
		},
		Bloom:      []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		Status:     0,
		Type:       TypeNormal,
		Passphrase: common.BytesToHash([]byte("test_pass")),
		Inputs:     &account.Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}
	acc.SetBalance(0)
	return acc
}

func makeVaultStyleAccount() *account.StateAccount {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return nil
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil
	}
	pass := "test_pass"
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil
	}
	masterKeyBytes, err := masterKey.Serialize()
	if err != nil {
		return nil
	}
	masterKeyHash := common.BytesToHash(masterKeyBytes)

	priv, err := crypto.GenerateAccount()
	if err != nil {
		return nil
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)

	acc := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: addr,
			Nonce:   1,
			Root:    common.Hash(addr.Bytes()),
			KeyHash: masterKeyHash,
		},
		Bloom:      []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Status:     0,
		Type:       TypeNormal,
		Passphrase: common.BytesToHash([]byte(pass)),
		Inputs:     &account.Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}
	acc.SetBalance(0)
	return acc
}

func shortHex(s string, head, tail int) string {
	if len(s) <= head+tail+2 {
		return s
	}
	return s[:head+2] + ".." + s[len(s)-tail:]
}

func printRow(desc string, acc *account.StateAccount) {
	data := acc.Bytes()
	addr := shortHex(acc.Address.Hex(), 6, 6) // 0x1234..5678
	fmt.Printf("  %-28s %4d B\n", desc, len(data))
	fmt.Printf("    %s %s n:%d b:%.2f\n", addr, typeName(acc.Type), acc.Nonce, acc.GetBalance())
	hexData := hex.EncodeToString(data)
	fmt.Printf("    %s..\n", hexData[:min(32, len(hexData))])
}
