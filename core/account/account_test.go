package account

import (
	"crypto/rand"
	"math/big"
	"reflect"
	"testing"
	"unsafe"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
)

func CreateTestStateAccount() StateAccount {

	privateKey, _ := crypto.GenerateAccount()
	pubkey := &privateKey.PublicKey
	address := crypto.PubkeyToAddress(pubkey)

	newAccount := StateAccount{
		StateAccountData: StateAccountData{
			Address: address,
			Nonce:   1,
		},
		Status: 0, // 0: OP_ACC_NEW
	}
	newAccount.SetBalance(0.0)
	return newAccount
}
func TestStateAccount_Size(t *testing.T) {

	privateKey, _ := crypto.GenerateAccount()
	pubkey := &privateKey.PublicKey
	address := crypto.PubkeyToAddress(pubkey)

	newAccount := StateAccount{
		StateAccountData: StateAccountData{
			Address: address,
			Nonce:   10000000000,
		},
		Status: 0, // 0: OP_ACC_NEW
	}
	newAccount.SetBalance(0.0)

	var sa = CreateTestStateAccount()
	t.Logf("Size of StateAccount: %d", unsafe.Sizeof(sa))
	t.Logf("Address of account %s", sa.Address)
	t.Logf("Size of StateAccount 2: %d", unsafe.Sizeof(newAccount))
	t.Logf("Address of account 2: %s", newAccount.Address)
}

func TestStateAccount_Bytes(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: address.Address{0x1, 0x2, 0x3, 0x4},
			Nonce:   42,
		},
		Status: 1, // 1: OP_ACC_STAKE
	}
	sa.SetBalance(100.0)

	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("Bytes failed: returned empty data")
	}

	sa2 := FromBytes(data)

	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("Bytes failed: Address mismatch")
	}
	if sa.Nonce != sa2.Nonce {
		t.Errorf("Bytes failed: Nonce mismatch")
	}
	if sa.Status != sa2.Status {
		t.Errorf("Bytes failed: Status mismatch")
	}

}

func TestFromBytes(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: address.Address{0x1, 0x2, 0x3},
			Nonce:   123,
		},
		Status: 2, // 2: OP_ACC_F
	}
	sa.SetBalance(50.5)

	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("TestFromBytes failed: returned empty data")
	}

	sa2 := FromBytes(data)

	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("TestFromBytes failed: Address mismatch")
	}
	if sa.Nonce != sa2.Nonce {
		t.Errorf("TestFromBytes failed: Nonce mismatch")
	}
	if sa.Status != sa2.Status {
		t.Errorf("TestFromBytes failed: Status mismatch")
	}
}

func TestStateAccount_ToBytes(t *testing.T) {
	var arr [32]byte
	_, err := rand.Read(arr[:])
	if err != nil {
		t.Fatal(err)
	}

	sa := NewStateAccount(arr, 0.0, common.EmptyRootHash)
	sa.SetBalance(100.5)

	// Convert to bytes
	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("ToBytes failed: returned empty data")
	}

	// Convert back from bytes
	sa2 := FromBytes(data)

	// Compare the fields
	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("ToBytes/FromBytes failed: Address mismatch. Got: %v, Want: %v", sa2.Address, sa.Address)
	}

	if sa.GetBalance() != sa2.GetBalance() {
		t.Errorf("ToBytes/FromBytes failed: Balance mismatch. Got: %f, Want: %f", sa2.GetBalance(), sa.GetBalance())
	}
	if sa.Nonce != sa2.Nonce {
		t.Errorf("ToBytes/FromBytes failed: Nonce mismatch. Got: %d, Want: %d", sa2.Nonce, sa.Nonce)
	}

	if sa.Status != sa2.Status {
		t.Errorf("ToBytes/FromBytes failed: Status mismatch. Got: %d, Want: %d", sa2.Status, sa.Status)
	}
}

func TestStateAccount_GetBalance_SetBalance(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Balance: big.NewInt(0),
		},
	}

	// Тест установки и получения нулевого баланса
	sa.SetBalance(0.0)
	if sa.GetBalance() != 0.0 {
		t.Errorf("SetBalance(0.0) failed: expected 0.0, got %f", sa.GetBalance())
	}

	// Тест установки положительного баланса
	sa.SetBalance(100.5)
	if sa.GetBalance() != 100.5 {
		t.Errorf("SetBalance(100.5) failed: expected 100.5, got %f", sa.GetBalance())
	}

	// Тест установки большого баланса
	sa.SetBalance(999999.999999)
	expected := 999999.999999
	got := sa.GetBalance()
	if got < expected-0.000001 || got > expected+0.000001 {
		t.Errorf("SetBalance(999999.999999) failed: expected ~%f, got %f", expected, got)
	}

	// Тест установки очень маленького баланса
	sa.SetBalance(0.000001)
	if sa.GetBalance() < 0.0000009 || sa.GetBalance() > 0.0000011 {
		t.Errorf("SetBalance(0.000001) failed: expected ~0.000001, got %f", sa.GetBalance())
	}
}

func TestStateAccount_GetBalanceBI_SetBalanceBI(t *testing.T) {
	sa := &StateAccount{}

	// Тест с nil
	sa.SetBalanceBI(nil)
	if sa.GetBalanceBI().Cmp(big.NewInt(0)) != 0 {
		t.Errorf("SetBalanceBI(nil) failed: expected 0, got %s", sa.GetBalanceBI().String())
	}

	// Тест с нулевым значением
	sa.SetBalanceBI(big.NewInt(0))
	if sa.GetBalanceBI().Cmp(big.NewInt(0)) != 0 {
		t.Errorf("SetBalanceBI(0) failed: expected 0, got %s", sa.GetBalanceBI().String())
	}

	// Тест с положительным значением
	val := big.NewInt(1000000)
	sa.SetBalanceBI(val)
	if sa.GetBalanceBI().Cmp(val) != 0 {
		t.Errorf("SetBalanceBI(1000000) failed: expected %s, got %s", val.String(), sa.GetBalanceBI().String())
	}

	// Тест что возвращается копия (изменение исходного не влияет)
	val.Add(val, big.NewInt(100))
	if sa.GetBalanceBI().Cmp(big.NewInt(1000000)) != 0 {
		t.Errorf("GetBalanceBI should return a copy: expected 1000000, got %s", sa.GetBalanceBI().String())
	}

	// Тест с очень большим числом
	bigVal := new(big.Int)
	bigVal.SetString("999999999999999999999999999999", 10)
	sa.SetBalanceBI(bigVal)
	if sa.GetBalanceBI().Cmp(bigVal) != 0 {
		t.Errorf("SetBalanceBI with large number failed")
	}
}

func TestStateAccount_BalanceEdgeCases(t *testing.T) {
	sa := &StateAccount{}

	// Тест с очень большим балансом через big.Int
	veryLargeVal := new(big.Int)
	veryLargeVal.SetString("999999999999999999999999999999999999999999", 10)
	sa.SetBalanceBI(veryLargeVal)
	if sa.GetBalanceBI().Cmp(veryLargeVal) != 0 {
		t.Errorf("Very large balance failed")
	}

	// Тест с нулевым балансом
	sa.SetBalance(0.0)
	if sa.GetBalance() != 0.0 {
		t.Errorf("Zero balance failed: expected 0.0, got %f", sa.GetBalance())
	}

	// Тест с очень маленьким балансом
	sa.SetBalance(0.0000000001)
	if sa.GetBalance() <= 0 {
		t.Errorf("Very small balance failed: got %f", sa.GetBalance())
	}
}

// TestStateAccount_Size_WithLargeBalance проверяет размер аккаунта с большим балансом
func TestStateAccount_Size_WithLargeBalance(t *testing.T) {
	sa := CreateTestStateAccount()

	// Маленький баланс
	sa.SetBalance(0.001)
	dataSmall := sa.Bytes()
	sizeSmall := len(dataSmall)

	// Большой баланс
	sa.SetBalance(999999999.999999)
	dataLarge := sa.Bytes()
	sizeLarge := len(dataLarge)

	// Размеры могут быть одинаковыми или большой баланс может быть больше
	// (зависит от того, сколько байт занимает big.Int)
	if sizeLarge < sizeSmall {
		t.Errorf("Large balance size should not be smaller: got %d, expected >= %d", sizeLarge, sizeSmall)
	}

	t.Logf("Account size with small balance (0.001): %d bytes", sizeSmall)
	t.Logf("Account size with large balance (999999999.999999): %d bytes", sizeLarge)
}

// TestStateAccount_Size_WithSpecialAddresses проверяет размер для специальных адресов
func TestStateAccount_Size_WithSpecialAddresses(t *testing.T) {
	sa1 := CreateTestStateAccount()
	data1 := sa1.Bytes()
	size1 := len(data1)

	sa2 := CreateTestStateAccount()
	sa2.Address = address.HexToAddress(BaseAddressHex)
	data2 := sa2.Bytes()
	size2 := len(data2)

	sa3 := CreateTestStateAccount()
	sa3.Address = address.HexToAddress(FaucetAddressHex)
	data3 := sa3.Bytes()
	size3 := len(data3)

	sa4 := CreateTestStateAccount()
	sa4.Address = address.HexToAddress(CoreStakingAddressHex)
	data4 := sa4.Bytes()
	size4 := len(data4)

	t.Logf("Normal address size: %d bytes", size1)
	t.Logf("BaseAddressHex size: %d bytes", size2)
	t.Logf("FaucetAddressHex size: %d bytes", size3)
	t.Logf("CoreStakingAddressHex size: %d bytes", size4)
}
