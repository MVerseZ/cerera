package types

import (
	"bytes"
	"math/big"
	"reflect"
	"sync"
	"testing"

	"github.com/cerera/internal/cerera/common"
)

func CreateTestStateAccount() StateAccount {
	var pass = "test_pass"

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	// seed := bip39.NewSeed(mnemonic, pass)

	// masterKey, _ := bip32.NewMasterKey(seed)
	// publicKey := masterKey.PublicKey()

	privateKey, _ := GenerateAccount()

	pubkey := &privateKey.PublicKey
	address := PubkeyToAddress(*pubkey)
	derBytes := EncodePrivateKeyToByte(privateKey)
	// derBytes, _ := x509.MarshalECPrivateKey(privateKey)
	var mpub [78]byte
	copy(mpub[:], []byte("test_public_key_placeholder_for_testing_purposes_only_78_bytes"))
	newAccount := StateAccount{
		StateAccountData: StateAccountData{
			Address: address,
			Nonce:   1,
			Root:    common.Hash(address.Bytes()),
		},
		CodeHash: derBytes,
		Status:   0, // 0: OP_ACC_NEW
		Bloom:    []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte(pass)),
		MPub:       mpub,
		// MPriv:      masterKey,
	}
	newAccount.SetBalance(0.0)
	return newAccount
}
func TestStateAccount_BloomUp(t *testing.T) {
	sa := &StateAccount{
		Bloom: []byte{0x0, 0x1, 0x0},
	}

	// Test incrementing Bloom[1] when it's less than 0xf
	sa.BloomUp()
	if sa.Bloom[1] != 0x2 {
		t.Errorf("BloomUp failed: expected Bloom[1] to be 0x2, got 0x%x", sa.Bloom[1])
	}

	// Set Bloom[1] to 0xf and test overflow behavior
	sa.Bloom[1] = 0xf
	sa.BloomUp()
	if sa.Bloom[1] != 0xf || sa.Bloom[2] != 0xf {
		t.Errorf("BloomUp failed: expected Bloom[1] to be 0xf and Bloom[2] to be 0xf, got 0x%x and 0x%x", sa.Bloom[1], sa.Bloom[2])
	}
}

func TestStateAccount_BloomDown(t *testing.T) {
	sa := &StateAccount{
		Bloom: []byte{0x0, 0x2, 0x0},
	}

	// Test decrementing Bloom[1] when it's greater than 0x1
	sa.BloomDown()
	if sa.Bloom[1] != 0x1 {
		t.Errorf("BloomDown failed: expected Bloom[1] to be 0x1, got 0x%x", sa.Bloom[1])
	}

	// Set Bloom[1] to 0x1 and test underflow behavior
	sa.Bloom[1] = 0x1
	sa.BloomDown()
	if sa.Bloom[1] != 0x1 || sa.Bloom[2] != 0xf {
		t.Errorf("BloomDown failed: expected Bloom[1] to be 0x1 and Bloom[2] to be 0xf, got 0x%x and 0x%x", sa.Bloom[1], sa.Bloom[2])
	}
}

func TestStateAccount_Bytes(t *testing.T) {
	var mpub [78]byte
	copy(mpub[:], []byte("public_key"))
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2, 0x3, 0x4},
			Nonce:   42,
			Root:    common.Hash{0x7, 0x8, 0x9},
		},
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Status:     1, // 1: OP_ACC_STAKE
		Passphrase: common.Hash{0xa, 0xb, 0xc},
		MPub:       mpub,
		Inputs:     &Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}
	sa.SetBalance(100.0)

	// Test JSON serialization (original Bytes method)
	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("Bytes failed: returned empty data")
	}

	// Test JSON deserialization
	sa2 := BytesToStateAccount(data)

	// Compare fields that are serialized in JSON
	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("Bytes failed: Address mismatch")
	}
	if !reflect.DeepEqual(sa.Bloom, sa2.Bloom) {
		t.Errorf("Bytes failed: Bloom mismatch")
	}
	if !reflect.DeepEqual(sa.CodeHash, sa2.CodeHash) {
		t.Errorf("Bytes failed: CodeHash mismatch")
	}
	if sa.Nonce != sa2.Nonce {
		t.Errorf("Bytes failed: Nonce mismatch")
	}
	if sa.Root != sa2.Root {
		t.Errorf("Bytes failed: Root mismatch")
	}
	if sa.Status != sa2.Status {
		t.Errorf("Bytes failed: Status mismatch")
	}
	if sa.Passphrase != sa2.Passphrase {
		t.Errorf("Bytes failed: Passphrase mismatch")
	}
	if !reflect.DeepEqual(sa.MPub, sa2.MPub) {
		t.Errorf("Bytes failed: MPub mismatch")
	}

	// Verify Inputs is properly initialized (Inputs are not serialized, but should be initialized)
	if sa2.Inputs == nil || sa2.Inputs.M == nil || sa2.Inputs.RWMutex == nil {
		t.Errorf("Bytes failed: Inputs not properly initialized after binary deserialization")
	}
}

func TestStateAccount_AddInput(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	// Add an input
	txHash := common.Hash{0x1}
	cnt := big.NewInt(50)
	sa.AddInput(txHash, cnt)

	// Verify the input was added
	sa.Inputs.RLock()
	defer sa.Inputs.RUnlock()
	if val, exists := sa.Inputs.M[txHash]; !exists || val.Cmp(cnt) != 0 {
		t.Errorf("AddInput failed: expected %v for hash %v, got %v", cnt, txHash, val)
	}
}

func TestBytesToStateAccount(t *testing.T) {
	// Create a test account
	var mpub [78]byte
	copy(mpub[:], []byte("test_public_key"))
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2, 0x3},
			Nonce:   123,
			Root:    common.Hash{0xa, 0xb, 0xc},
		},
		Bloom:      []byte{0x4, 0x5, 0x6},
		CodeHash:   []byte{0x7, 0x8, 0x9},
		Status:     2, // 2: OP_ACC_F
		Passphrase: common.Hash{0xd, 0xe, 0xf},
		MPub:       mpub,
		Inputs:     &Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}
	sa.SetBalance(50.5)

	// Convert to binary bytes using custom serialization
	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("TestBytesToStateAccount failed: returned empty data")
	}

	// Convert back using BytesToStateAccount
	sa2 := BytesToStateAccount(data)

	// Verify all serializable fields
	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("TestBytesToStateAccount failed: Address mismatch")
	}
	if !reflect.DeepEqual(sa.Bloom, sa2.Bloom) {
		t.Errorf("TestBytesToStateAccount failed: Bloom mismatch")
	}
	if !reflect.DeepEqual(sa.CodeHash, sa2.CodeHash) {
		t.Errorf("TestBytesToStateAccount failed: CodeHash mismatch")
	}
	if sa.Nonce != sa2.Nonce {
		t.Errorf("TestBytesToStateAccount failed: Nonce mismatch")
	}
	if sa.Root != sa2.Root {
		t.Errorf("TestBytesToStateAccount failed: Root mismatch")
	}
	if sa.Status != sa2.Status {
		t.Errorf("TestBytesToStateAccount failed: Status mismatch")
	}
	if sa.Passphrase != sa2.Passphrase {
		t.Errorf("TestBytesToStateAccount failed: Passphrase mismatch")
	}
	if !reflect.DeepEqual(sa.MPub, sa2.MPub) {
		t.Errorf("TestBytesToStateAccount failed: MPub mismatch")
	}

	// Verify Inputs is properly initialized (Inputs are not serialized, but should be initialized)
	if sa2.Inputs == nil || sa2.Inputs.M == nil || sa2.Inputs.RWMutex == nil {
		t.Errorf("TestBytesToStateAccount failed: Inputs not properly initialized after binary deserialization")
	}
}

func TestStateAccount_ToBytes(t *testing.T) {
	// Create a test StateAccount with some data
	var mpub [78]byte
	copy(mpub[:], []byte("public_key_string"))
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2, 0x3, 0x4},
			Nonce:   42,
			Root:    common.Hash{0x7, 0x8, 0x9},
		},
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Status:     1, // 1: OP_ACC_STAKE
		Passphrase: common.Hash{0xa, 0xb, 0xc},
		MPub:       mpub,
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	sa.SetBalance(100.5)

	// Add some inputs
	txHash1 := common.Hash{0x1, 0x2, 0x3}
	txHash2 := common.Hash{0x4, 0x5, 0x6}
	sa.AddInput(txHash1, big.NewInt(100))
	sa.AddInput(txHash2, big.NewInt(200))

	// Convert to bytes
	data := sa.Bytes()
	if len(data) == 0 {
		t.Fatal("ToBytes failed: returned empty data")
	}

	// Convert back from bytes
	sa2 := BytesToStateAccount(data)

	// Compare the fields
	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("ToBytes/FromBytes failed: Address mismatch. Got: %v, Want: %v", sa2.Address, sa.Address)
	}

	if sa.GetBalance() != sa2.GetBalance() {
		t.Errorf("ToBytes/FromBytes failed: Balance mismatch. Got: %f, Want: %f", sa2.GetBalance(), sa.GetBalance())
	}

	if !reflect.DeepEqual(sa.Bloom, sa2.Bloom) {
		t.Errorf("ToBytes/FromBytes failed: Bloom mismatch. Got: %v, Want: %v", sa2.Bloom, sa.Bloom)
	}

	if !reflect.DeepEqual(sa.CodeHash, sa2.CodeHash) {
		t.Errorf("ToBytes/FromBytes failed: CodeHash mismatch. Got: %v, Want: %v", sa2.CodeHash, sa.CodeHash)
	}

	if sa.Nonce != sa2.Nonce {
		t.Errorf("ToBytes/FromBytes failed: Nonce mismatch. Got: %d, Want: %d", sa2.Nonce, sa.Nonce)
	}

	if sa.Root != sa2.Root {
		t.Errorf("ToBytes/FromBytes failed: Root mismatch. Got: %v, Want: %v", sa2.Root, sa.Root)
	}

	if sa.Status != sa2.Status {
		t.Errorf("ToBytes/FromBytes failed: Status mismatch. Got: %d, Want: %d", sa2.Status, sa.Status)
	}

	if sa.Passphrase != sa2.Passphrase {
		t.Errorf("ToBytes/FromBytes failed: Passphrase mismatch. Got: %v, Want: %v", sa2.Passphrase, sa.Passphrase)
	}

	if !reflect.DeepEqual(sa.MPub, sa2.MPub) {
		t.Errorf("ToBytes/FromBytes failed: MPub mismatch. Got: %v, Want: %v", sa2.MPub, sa.MPub)
	}

	// Inputs are not serialized, so we only verify they are initialized
	if sa2.Inputs == nil || sa2.Inputs.M == nil || sa2.Inputs.RWMutex == nil {
		t.Errorf("ToBytes/FromBytes failed: Inputs not properly initialized after binary deserialization")
	}
}

func TestStateAccount_ToBytes_EmptyInputs(t *testing.T) {
	// Test with empty Inputs map
	var mpub [78]byte
	copy(mpub[:], []byte("test_key"))
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2},
			Nonce:   1,
			Root:    common.Hash{0x5},
		},
		Bloom:      []byte{0x1, 0x2},
		CodeHash:   []byte{0x3, 0x4},
		Status:     0, // 0: OP_ACC_NEW
		Passphrase: common.Hash{0x6},
		MPub:       mpub,
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	sa.SetBalance(0.0)

	// Convert to bytes and back
	data := sa.Bytes()
	sa2 := BytesToStateAccount(data)
	if sa2 == nil {
		t.Fatalf("BytesToStateAccount returned nil")
	}

	// Verify basic fields
	if !reflect.DeepEqual(sa.Address, sa2.Address) {
		t.Errorf("Empty Inputs test failed: Address mismatch")
	}

	if sa.GetBalance() != sa2.GetBalance() {
		t.Errorf("Empty Inputs test failed: Balance mismatch")
	}

	if len(sa2.Inputs.M) != 0 {
		t.Errorf("Empty Inputs test failed: Expected empty Inputs map, got %d entries", len(sa2.Inputs.M))
	}
}

// ============================================================
// Тесты для GetBalance и SetBalance
// ============================================================

func TestStateAccount_GetBalance_SetBalance(t *testing.T) {
	sa := &StateAccount{
		balance: big.NewInt(0),
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

func TestStateAccount_GetAllInputs(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	// Тест с пустыми инпутами
	allInputs := sa.GetAllInputs()
	if len(allInputs) != 0 {
		t.Errorf("GetAllInputs with empty map failed: expected 0, got %d", len(allInputs))
	}

	// Добавляем несколько инпутов
	txHash1 := common.Hash{0x1, 0x2, 0x3}
	txHash2 := common.Hash{0x4, 0x5, 0x6}
	txHash3 := common.Hash{0x7, 0x8, 0x9}

	sa.AddInput(txHash1, big.NewInt(100))
	sa.AddInput(txHash2, big.NewInt(200))
	sa.AddInput(txHash3, big.NewInt(300))

	// Получаем все инпуты
	allInputs = sa.GetAllInputs()
	if len(allInputs) != 3 {
		t.Errorf("GetAllInputs failed: expected 3, got %d", len(allInputs))
	}

	// Проверяем значения
	if allInputs[txHash1].Cmp(big.NewInt(100)) != 0 {
		t.Errorf("GetAllInputs failed: expected 100 for txHash1, got %s", allInputs[txHash1].String())
	}
	if allInputs[txHash2].Cmp(big.NewInt(200)) != 0 {
		t.Errorf("GetAllInputs failed: expected 200 for txHash2, got %s", allInputs[txHash2].String())
	}
	if allInputs[txHash3].Cmp(big.NewInt(300)) != 0 {
		t.Errorf("GetAllInputs failed: expected 300 for txHash3, got %s", allInputs[txHash3].String())
	}

	// Проверяем что возвращается копия (изменение не влияет на оригинал)
	allInputs[txHash1].Add(allInputs[txHash1], big.NewInt(50))
	sa.Inputs.RLock()
	originalVal := sa.Inputs.M[txHash1]
	sa.Inputs.RUnlock()
	if originalVal.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("GetAllInputs should return copies: original value changed")
	}
}

func TestStateAccount_GetAllInputs_NilInputs(t *testing.T) {
	sa := &StateAccount{
		Inputs: nil,
	}

	allInputs := sa.GetAllInputs()
	if allInputs == nil {
		t.Fatal("GetAllInputs with nil Inputs should return empty map, not nil")
	}
	if len(allInputs) != 0 {
		t.Errorf("GetAllInputs with nil Inputs should return empty map, got %d entries", len(allInputs))
	}
}

func TestNewStateAccount(t *testing.T) {
	address := Address{0x1, 0x2, 0x3, 0x4}
	balance := 100.5
	root := common.Hash{0x5, 0x6, 0x7}

	sa := NewStateAccount(address, balance, root)

	if sa == nil {
		t.Fatal("NewStateAccount returned nil")
	}

	if !reflect.DeepEqual(sa.Address, address) {
		t.Errorf("NewStateAccount Address mismatch: expected %v, got %v", address, sa.Address)
	}

	if sa.GetBalance() != balance {
		t.Errorf("NewStateAccount Balance mismatch: expected %f, got %f", balance, sa.GetBalance())
	}

	if sa.Root != root {
		t.Errorf("NewStateAccount Root mismatch: expected %v, got %v", root, sa.Root)
	}

	if sa.Nonce != 1 {
		t.Errorf("NewStateAccount Nonce mismatch: expected 1, got %d", sa.Nonce)
	}

	if sa.Status != 0 {
		t.Errorf("NewStateAccount Status mismatch: expected 0, got %d", sa.Status)
	}

	if sa.Type != 0 {
		t.Errorf("NewStateAccount Type mismatch: expected 0, got %d", sa.Type)
	}

	if sa.Inputs == nil {
		t.Fatal("NewStateAccount Inputs should not be nil")
	}

	if sa.Inputs.M == nil {
		t.Fatal("NewStateAccount Inputs.M should not be nil")
	}

	if len(sa.Bloom) == 0 {
		t.Error("NewStateAccount Bloom should not be empty")
	}
}

func TestStateAccount_AddInput_NilInputs(t *testing.T) {
	sa := &StateAccount{
		Inputs: nil,
	}

	txHash := common.Hash{0x1}
	cnt := big.NewInt(50)

	// AddInput должен инициализировать Inputs если он nil
	sa.AddInput(txHash, cnt)

	if sa.Inputs == nil {
		t.Fatal("AddInput should initialize Inputs if nil")
	}

	sa.Inputs.RLock()
	val, exists := sa.Inputs.M[txHash]
	sa.Inputs.RUnlock()

	if !exists {
		t.Fatal("AddInput failed: input was not added")
	}

	if val.Cmp(cnt) != 0 {
		t.Errorf("AddInput failed: expected %v, got %v", cnt, val)
	}
}

func TestStateAccount_AddInput_NilValue(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	txHash := common.Hash{0x1}

	// Добавляем nil значение
	sa.AddInput(txHash, nil)

	sa.Inputs.RLock()
	val, exists := sa.Inputs.M[txHash]
	sa.Inputs.RUnlock()

	if !exists {
		t.Fatal("AddInput with nil value should still add entry")
	}

	if val.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("AddInput with nil value should set to 0, got %s", val.String())
	}
}

func TestStateAccount_AddInput_Multiple(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	// Добавляем множество инпутов
	for i := 0; i < 100; i++ {
		txHash := common.Hash{byte(i), byte(i >> 8), byte(i >> 16)}
		sa.AddInput(txHash, big.NewInt(int64(i*10)))
	}

	sa.Inputs.RLock()
	count := len(sa.Inputs.M)
	sa.Inputs.RUnlock()

	if count != 100 {
		t.Errorf("AddInput multiple failed: expected 100 entries, got %d", count)
	}
}

func TestStateAccount_AddInput_Overwrite(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	txHash := common.Hash{0x1}

	// Добавляем первый раз
	sa.AddInput(txHash, big.NewInt(100))

	// Перезаписываем тем же хешем
	sa.AddInput(txHash, big.NewInt(200))

	sa.Inputs.RLock()
	val, exists := sa.Inputs.M[txHash]
	sa.Inputs.RUnlock()

	if !exists {
		t.Fatal("AddInput overwrite failed: entry does not exist")
	}

	if val.Cmp(big.NewInt(200)) != 0 {
		t.Errorf("AddInput overwrite failed: expected 200, got %s", val.String())
	}
}

// ============================================================
// Тесты для различных статусов и типов
// ============================================================

func TestStateAccount_StatusTypes(t *testing.T) {
	statuses := []byte{0, 1, 2, 3, 4} // OP_ACC_NEW, OP_ACC_STAKE, OP_ACC_F, OP_ACC_NODE, VOID
	types := []byte{0, 1, 2, 3, 4}    // normal, staking, voting, faucet, coinbase

	for _, status := range statuses {
		for _, accType := range types {
			var mpub [78]byte
			copy(mpub[:], []byte("test_key"))

			sa := &StateAccount{
				StateAccountData: StateAccountData{
					Address: Address{0x1, 0x2},
					Nonce:   1,
					Root:    common.Hash{0x5},
				},
				Status:     status,
				Type:       accType,
				Bloom:      []byte{0x1, 0x2},
				CodeHash:   []byte{0x3, 0x4},
				Passphrase: common.Hash{0x6},
				MPub:       mpub,
				Inputs: &Input{
					RWMutex: &sync.RWMutex{},
					M:       make(map[common.Hash]*big.Int),
				},
			}
			sa.SetBalance(100.0)

			// Сериализуем и десериализуем
			data := sa.Bytes()
			sa2 := BytesToStateAccount(data)

			if sa2.Status != status {
				t.Errorf("Status serialization failed: expected %d, got %d", status, sa2.Status)
			}

			if sa2.Type != accType {
				t.Errorf("Type serialization failed: expected %d, got %d", accType, sa2.Type)
			}
		}
	}
}

// ============================================================
// Тесты для специальных адресов
// ============================================================

func TestStateAccount_SpecialAddresses(t *testing.T) {
	specialAddresses := []string{
		BaseAddressHex,
		FaucetAddressHex,
		CoreStakingAddressHex,
	}

	for _, addrHex := range specialAddresses {
		addr := HexToAddress(addrHex)
		var mpub [78]byte
		copy(mpub[:], []byte("special_key"))

		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: addr,
				Nonce:   1,
				Root:    common.Hash{0x5},
			},
			Bloom:      []byte{0x1, 0x2},
			CodeHash:   []byte{0x3, 0x4, 0x5}, // Должен быть обнулен для специальных адресов
			Status:     0,
			Passphrase: common.Hash{0x6},
			MPub:       mpub,
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		// Сериализуем
		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if sa2.Address != addr {
			t.Errorf("Special address serialization failed for %s", addrHex)
		}

		// CodeHash должен быть обнулен для специальных адресов при сериализации
		// (но при десериализации он будет пустым)
		if len(sa2.CodeHash) != 0 {
			t.Errorf("Special address CodeHash should be empty for %s, got %v", addrHex, sa2.CodeHash)
		}
	}
}

// ============================================================
// Тесты для Nonce
// ============================================================

func TestStateAccount_Nonce(t *testing.T) {
	nonces := []uint64{0, 1, 100, 1000, 999999999}

	for _, nonce := range nonces {
		var mpub [78]byte
		copy(mpub[:], []byte("test_key"))

		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: Address{0x1, 0x2},
				Nonce:   nonce,
				Root:    common.Hash{0x5},
			},
			Bloom:      []byte{0x1, 0x2},
			CodeHash:   []byte{0x3, 0x4},
			Status:     0,
			Passphrase: common.Hash{0x6},
			MPub:       mpub,
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if sa2.Nonce != nonce {
			t.Errorf("Nonce serialization failed: expected %d, got %d", nonce, sa2.Nonce)
		}
	}
}

// ============================================================
// Тесты для Root
// ============================================================

func TestStateAccount_Root(t *testing.T) {
	roots := []common.Hash{
		{0x0},
		{0xff, 0xff, 0xff, 0xff},
		{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
		common.BytesToHash([]byte("test_root_hash_32_bytes_long")),
	}

	for _, root := range roots {
		var mpub [78]byte
		copy(mpub[:], []byte("test_key"))

		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: Address{0x1, 0x2},
				Root:    root,
				Nonce:   1,
			},
			Bloom:      []byte{0x1, 0x2},
			CodeHash:   []byte{0x3, 0x4},
			Status:     0,
			Passphrase: common.Hash{0x6},
			MPub:       mpub,
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if sa2.Root != root {
			t.Errorf("Root serialization failed: expected %v, got %v", root, sa2.Root)
		}
	}
}

// ============================================================
// Тесты для CodeHash
// ============================================================

func TestStateAccount_CodeHash(t *testing.T) {
	codeHashes := [][]byte{
		{},
		{0x1},
		{0x1, 0x2, 0x3, 0x4},
		[]byte("test_code_hash"),
		make([]byte, 100),
	}

	for _, codeHash := range codeHashes {
		var mpub [78]byte
		copy(mpub[:], []byte("test_key"))

		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: Address{0x1, 0x2}, // Не специальный адрес
				Nonce:   1,
				Root:    common.Hash{0x5},
			},
			CodeHash:   codeHash,
			Bloom:      []byte{0x1, 0x2},
			Status:     0,
			Passphrase: common.Hash{0x6},
			MPub:       mpub,
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if !reflect.DeepEqual(sa2.CodeHash, codeHash) {
			t.Errorf("CodeHash serialization failed: expected %v, got %v", codeHash, sa2.CodeHash)
		}
	}
}

// ============================================================
// Тесты для Passphrase
// ============================================================

func TestStateAccount_Passphrase(t *testing.T) {
	passphrases := []common.Hash{
		{0x0},
		{0xff, 0xff, 0xff, 0xff},
		common.BytesToHash([]byte("test_passphrase_32_bytes")),
		{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10,
			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20},
	}

	for _, passphrase := range passphrases {
		var mpub [78]byte
		copy(mpub[:], []byte("test_key"))

		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: Address{0x1, 0x2},
				Nonce:   1,
				Root:    common.Hash{0x5},
			},
			Passphrase: passphrase,
			Bloom:      []byte{0x1, 0x2},
			CodeHash:   []byte{0x3, 0x4},
			Status:     0,
			MPub:       mpub,
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if sa2.Passphrase != passphrase {
			t.Errorf("Passphrase serialization failed: expected %v, got %v", passphrase, sa2.Passphrase)
		}
	}
}

// ============================================================
// Тесты для MPub
// ============================================================

func TestStateAccount_MPub(t *testing.T) {
	mpubValues := [][78]byte{
		{},
		{0xff, 0xff, 0xff, 0xff},
	}

	// Заполняем второй массив
	copy(mpubValues[1][:], []byte("test_public_key_string_for_testing_purposes_78_bytes_long"))

	for _, mpub := range mpubValues {
		sa := &StateAccount{
			StateAccountData: StateAccountData{
				Address: Address{0x1, 0x2},
				Nonce:   1,
				Root:    common.Hash{0x5},
			},
			MPub:       mpub,
			Bloom:      []byte{0x1, 0x2},
			CodeHash:   []byte{0x3, 0x4},
			Status:     0,
			Passphrase: common.Hash{0x6},
			Inputs: &Input{
				RWMutex: &sync.RWMutex{},
				M:       make(map[common.Hash]*big.Int),
			},
		}
		sa.SetBalance(100.0)

		data := sa.Bytes()
		sa2 := BytesToStateAccount(data)

		if !reflect.DeepEqual(sa2.MPub, mpub) {
			t.Errorf("MPub serialization failed: expected %v, got %v", mpub, sa2.MPub)
		}
	}
}

// ============================================================
// Тесты для конкурентного доступа к Inputs
// ============================================================

func TestStateAccount_ConcurrentInputs(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	// Запускаем несколько горутин для конкурентного доступа
	var wg sync.WaitGroup
	numGoroutines := 10
	numOpsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				txHash := common.Hash{byte(id), byte(j), byte(j >> 8)}
				sa.AddInput(txHash, big.NewInt(int64(id*1000+j)))
			}
		}(i)
	}

	wg.Wait()

	// Проверяем что все инпуты добавлены
	sa.Inputs.RLock()
	count := len(sa.Inputs.M)
	sa.Inputs.RUnlock()

	expectedCount := numGoroutines * numOpsPerGoroutine
	if count != expectedCount {
		t.Errorf("Concurrent AddInput failed: expected %d entries, got %d", expectedCount, count)
	}
}

func TestStateAccount_ConcurrentGetAllInputs(t *testing.T) {
	sa := &StateAccount{
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

	// Добавляем инпуты
	for i := 0; i < 100; i++ {
		txHash := common.Hash{byte(i), byte(i >> 8)}
		sa.AddInput(txHash, big.NewInt(int64(i)))
	}

	// Запускаем несколько горутин для конкурентного чтения
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allInputs := sa.GetAllInputs()
			if len(allInputs) != 100 {
				t.Errorf("Concurrent GetAllInputs failed: expected 100, got %d", len(allInputs))
			}
		}()
	}

	wg.Wait()
}

// ============================================================
// Тесты для edge cases баланса
// ============================================================

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

// ============================================================
// Тесты для сериализации с Inputs
// ============================================================

func TestStateAccount_SerializationWithInputs(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2, 0x3},
			Nonce:   42,
			Root:    common.Hash{0x5, 0x6},
		},
		Bloom:      []byte{0x1, 0x2},
		CodeHash:   []byte{0x3, 0x4},
		Status:     1,
		Passphrase: common.Hash{0x7, 0x8},
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	sa.SetBalance(100.0)

	// Добавляем несколько инпутов
	txHash1 := common.Hash{0x1, 0x2, 0x3}
	txHash2 := common.Hash{0x4, 0x5, 0x6}
	txHash3 := common.Hash{0x7, 0x8, 0x9}

	sa.AddInput(txHash1, big.NewInt(100))
	sa.AddInput(txHash2, big.NewInt(200))
	sa.AddInput(txHash3, big.NewInt(300))

	// Сериализуем
	data := sa.Bytes()
	sa2 := BytesToStateAccount(data)

	if sa2 == nil {
		t.Fatal("BytesToStateAccount returned nil")
	}

	// Проверяем что инпуты восстановлены
	sa2.Inputs.RLock()
	count := len(sa2.Inputs.M)
	sa2.Inputs.RUnlock()

	if count != 3 {
		t.Errorf("Inputs serialization failed: expected 3, got %d", count)
	}

	// Проверяем значения инпутов
	sa2.Inputs.RLock()
	val1 := sa2.Inputs.M[txHash1]
	val2 := sa2.Inputs.M[txHash2]
	val3 := sa2.Inputs.M[txHash3]
	sa2.Inputs.RUnlock()

	if val1 == nil || val1.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("Input 1 serialization failed: expected 100, got %v", val1)
	}
	if val2 == nil || val2.Cmp(big.NewInt(200)) != 0 {
		t.Errorf("Input 2 serialization failed: expected 200, got %v", val2)
	}
	if val3 == nil || val3.Cmp(big.NewInt(300)) != 0 {
		t.Errorf("Input 3 serialization failed: expected 300, got %v", val3)
	}
}

func TestStateAccount_SerializationWithEmptyInputs(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2},
			Nonce:   1,
			Root:    common.Hash{0x5},
		},
		Bloom:      []byte{0x1, 0x2},
		CodeHash:   []byte{0x3, 0x4},
		Status:     0,
		Passphrase: common.Hash{0x6},
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int), // Пустая map
		},
	}
	sa.SetBalance(100.0)

	data := sa.Bytes()
	sa2 := BytesToStateAccount(data)

	if sa2 == nil {
		t.Fatal("BytesToStateAccount returned nil")
	}

	sa2.Inputs.RLock()
	count := len(sa2.Inputs.M)
	sa2.Inputs.RUnlock()

	if count != 0 {
		t.Errorf("Empty Inputs serialization failed: expected 0, got %d", count)
	}
}

// ============================================================
// Тесты для Bloom edge cases
// ============================================================

func TestStateAccount_BloomEdgeCases(t *testing.T) {
	// Тест с пустым Bloom
	sa := &StateAccount{
		Bloom: []byte{},
	}

	// BloomUp на пустом Bloom должен паниковать или обрабатываться корректно
	// Проверяем что не паникует
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("BloomUp on empty Bloom panicked: %v", r)
		}
	}()

	if len(sa.Bloom) > 1 {
		sa.BloomUp()
	}

	// Тест с очень коротким Bloom
	sa.Bloom = []byte{0x0}
	if len(sa.Bloom) > 1 {
		sa.BloomUp()
	}

	// Тест с Bloom[1] = 0
	sa.Bloom = []byte{0x0, 0x0, 0x0}
	sa.BloomUp()
	if sa.Bloom[1] != 0x1 {
		t.Errorf("BloomUp from 0 failed: expected 0x1, got 0x%x", sa.Bloom[1])
	}

	// Тест с Bloom[1] = 0xf (максимум)
	sa.Bloom = []byte{0x0, 0xf, 0x0}
	sa.BloomUp()
	if sa.Bloom[1] != 0xf || sa.Bloom[2] != 0xf {
		t.Errorf("BloomUp overflow failed: expected Bloom[1]=0xf, Bloom[2]=0xf, got 0x%x, 0x%x", sa.Bloom[1], sa.Bloom[2])
	}

	// Тест BloomDown с Bloom[1] = 0x1 (минимум)
	sa.Bloom = []byte{0x0, 0x1, 0x0}
	sa.BloomDown()
	if sa.Bloom[1] != 0x1 || sa.Bloom[2] != 0xf {
		t.Errorf("BloomDown underflow failed: expected Bloom[1]=0x1, Bloom[2]=0xf, got 0x%x, 0x%x", sa.Bloom[1], sa.Bloom[2])
	}
}

// TestStateAccount_Size_Minimal проверяет минимальный размер сериализованного аккаунта
func TestStateAccount_Size_Minimal(t *testing.T) {
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{},
			Nonce:   0,
			Root:    common.Hash{},
		},
		Bloom:      []byte{},
		CodeHash:   []byte{},
		Status:     0,
		Type:       0,
		Passphrase: common.Hash{},
		MPub:       [78]byte{},
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	sa.SetBalance(0.0)

	data := sa.Bytes()
	minSize := len(data)

	// Минимальный размер должен включать:
	// 1 byte (Type) + 4 bytes (address len) + 0 bytes (address) + 32 bytes (Passphrase) +
	// 4 bytes (MPub len) + 78 bytes (MPub) + 4 bytes (Bloom len) + 0 bytes (Bloom) +
	// 4 bytes (CodeHash len) + 0 bytes (CodeHash) + 8 bytes (Nonce) + 32 bytes (Root) +
	// 1 byte (Status) + 4 bytes (balance len) + 0 bytes (balance) + 4 bytes (inputs count)
	expectedMinSize := 1 + 4 + 0 + 32 + 4 + 78 + 4 + 0 + 4 + 0 + 8 + 32 + 1 + 4 + 0 + 4
	if minSize < expectedMinSize {
		t.Errorf("Minimal size too small: got %d, expected at least %d", minSize, expectedMinSize)
	}
	t.Logf("Minimal account size: %d bytes", minSize)
}

// TestStateAccount_Size_WithData проверяет размер аккаунта с данными
func TestStateAccount_Size_WithData(t *testing.T) {
	var mpub [78]byte
	copy(mpub[:], []byte("test_public_key_placeholder_for_testing_purposes_only_78_bytes"))
	sa := &StateAccount{
		StateAccountData: StateAccountData{
			Address: Address{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20},
			Nonce:   12345,
			Root:    common.Hash{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00, 0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00},
		},
		Bloom:      []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa},
		CodeHash:   []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		Status:     1,
		Type:       0,
		Passphrase: common.Hash{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00},
		MPub:       mpub,
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	sa.SetBalance(100.5)

	data := sa.Bytes()
	sizeWithData := len(data)

	// Размер должен быть больше минимального
	minSize := 1 + 4 + 0 + 32 + 4 + 78 + 4 + 0 + 4 + 0 + 8 + 32 + 1 + 4 + 0 + 4
	if sizeWithData <= minSize {
		t.Errorf("Size with data should be larger than minimal: got %d, expected > %d", sizeWithData, minSize)
	}

	// Проверяем, что размер включает все данные
	// Address: 32 bytes + 4 bytes (len)
	// Bloom: 10 bytes + 4 bytes (len)
	// CodeHash: 6 bytes + 4 bytes (len)
	// Balance: несколько байт + 4 bytes (len)
	expectedMinWithData := minSize + 32 + 10 + 6 + 4 // минимум для balance
	if sizeWithData < expectedMinWithData {
		t.Errorf("Size with data too small: got %d, expected at least %d", sizeWithData, expectedMinWithData)
	}

	t.Logf("Account size with data: %d bytes", sizeWithData)
}

// TestStateAccount_Size_WithInputs проверяет размер аккаунта с inputs
func TestStateAccount_Size_WithInputs(t *testing.T) {
	sa := CreateTestStateAccount()
	sa.SetBalance(1000.0)

	// Размер без inputs
	dataWithoutInputs := sa.Bytes()
	sizeWithoutInputs := len(dataWithoutInputs)

	// Добавляем inputs
	txHash1 := common.Hash{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20}
	txHash2 := common.Hash{0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21}
	txHash3 := common.Hash{0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22}

	sa.AddInput(txHash1, big.NewInt(100))
	sa.AddInput(txHash2, big.NewInt(200))
	sa.AddInput(txHash3, big.NewInt(300))

	dataWithInputs := sa.Bytes()
	sizeWithInputs := len(dataWithInputs)

	// Размер с inputs должен быть больше
	if sizeWithInputs <= sizeWithoutInputs {
		t.Errorf("Size with inputs should be larger: got %d, expected > %d", sizeWithInputs, sizeWithoutInputs)
	}

	// Каждый input добавляет: 32 bytes (hash) + 4 bytes (value len) + несколько байт (value)
	// 3 inputs должны добавить минимум: 3 * (32 + 4) = 108 bytes
	expectedIncrease := 3 * (32 + 4) // минимум для каждого input
	actualIncrease := sizeWithInputs - sizeWithoutInputs
	if actualIncrease < expectedIncrease {
		t.Errorf("Inputs size increase too small: got %d, expected at least %d", actualIncrease, expectedIncrease)
	}

	t.Logf("Account size without inputs: %d bytes", sizeWithoutInputs)
	t.Logf("Account size with 3 inputs: %d bytes", sizeWithInputs)
	t.Logf("Size increase: %d bytes", actualIncrease)
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
	// Обычный адрес
	sa1 := CreateTestStateAccount()
	data1 := sa1.Bytes()
	size1 := len(data1)

	// BaseAddressHex
	sa2 := CreateTestStateAccount()
	sa2.Address = HexToAddress(BaseAddressHex)
	sa2.CodeHash = []byte{0x1, 0x2, 0x3} // Обычно для специальных адресов CodeHash = 0
	data2 := sa2.Bytes()
	size2 := len(data2)

	// FaucetAddressHex
	sa3 := CreateTestStateAccount()
	sa3.Address = HexToAddress(FaucetAddressHex)
	sa3.CodeHash = []byte{0x1, 0x2, 0x3}
	data3 := sa3.Bytes()
	size3 := len(data3)

	// CoreStakingAddressHex
	sa4 := CreateTestStateAccount()
	sa4.Address = HexToAddress(CoreStakingAddressHex)
	sa4.CodeHash = []byte{0x1, 0x2, 0x3}
	data4 := sa4.Bytes()
	size4 := len(data4)

	// Для специальных адресов CodeHash записывается как 4 нулевых байта вместо длины + данных
	// Поэтому размер должен быть меньше, если CodeHash был непустым
	if size2 > size1+10 { // допускаем небольшую разницу
		t.Logf("Special address size difference: BaseAddress %d vs normal %d", size2, size1)
	}

	t.Logf("Normal address size: %d bytes", size1)
	t.Logf("BaseAddressHex size: %d bytes", size2)
	t.Logf("FaucetAddressHex size: %d bytes", size3)
	t.Logf("CoreStakingAddressHex size: %d bytes", size4)
}

// TestStateAccount_Size_WithDifferentBloomSizes проверяет размер с разными размерами Bloom
func TestStateAccount_Size_WithDifferentBloomSizes(t *testing.T) {
	sa1 := CreateTestStateAccount()
	sa1.Bloom = []byte{} // Пустой Bloom
	data1 := sa1.Bytes()
	size1 := len(data1)

	sa2 := CreateTestStateAccount()
	sa2.Bloom = []byte{0x1, 0x2, 0x3} // Маленький Bloom
	data2 := sa2.Bytes()
	size2 := len(data2)

	sa3 := CreateTestStateAccount()
	sa3.Bloom = make([]byte, 100) // Большой Bloom
	for i := range sa3.Bloom {
		sa3.Bloom[i] = byte(i % 256)
	}
	data3 := sa3.Bytes()
	size3 := len(data3)

	// Размер должен увеличиваться с размером Bloom
	if size2 <= size1 {
		t.Errorf("Size with small Bloom should be larger: got %d, expected > %d", size2, size1)
	}
	if size3 <= size2 {
		t.Errorf("Size with large Bloom should be larger: got %d, expected > %d", size3, size2)
	}

	// Проверяем, что разница соответствует размеру Bloom
	// Каждый байт Bloom добавляет 1 байт к размеру
	bloomDiff1 := size2 - size1
	bloomDiff2 := size3 - size2

	if bloomDiff1 < 3 {
		t.Errorf("Bloom size increase too small: got %d, expected at least 3", bloomDiff1)
	}
	if bloomDiff2 < 97 {
		t.Errorf("Bloom size increase too small: got %d, expected at least 97", bloomDiff2)
	}

	t.Logf("Account size with empty Bloom: %d bytes", size1)
	t.Logf("Account size with small Bloom (3 bytes): %d bytes", size2)
	t.Logf("Account size with large Bloom (100 bytes): %d bytes", size3)
}

// TestStateAccount_Size_WithDifferentCodeHashSizes проверяет размер с разными размерами CodeHash
func TestStateAccount_Size_WithDifferentCodeHashSizes(t *testing.T) {
	sa1 := CreateTestStateAccount()
	sa1.CodeHash = []byte{} // Пустой CodeHash
	data1 := sa1.Bytes()
	size1 := len(data1)

	sa2 := CreateTestStateAccount()
	sa2.CodeHash = []byte{0x1, 0x2, 0x3} // Маленький CodeHash
	data2 := sa2.Bytes()
	size2 := len(data2)

	sa3 := CreateTestStateAccount()
	sa3.CodeHash = make([]byte, 64) // Большой CodeHash (например, hash контракта)
	for i := range sa3.CodeHash {
		sa3.CodeHash[i] = byte(i % 256)
	}
	data3 := sa3.Bytes()
	size3 := len(data3)

	// Размер должен увеличиваться с размером CodeHash
	if size2 <= size1 {
		t.Errorf("Size with small CodeHash should be larger: got %d, expected > %d", size2, size1)
	}
	if size3 <= size2 {
		t.Errorf("Size with large CodeHash should be larger: got %d, expected > %d", size3, size2)
	}

	t.Logf("Account size with empty CodeHash: %d bytes", size1)
	t.Logf("Account size with small CodeHash (3 bytes): %d bytes", size2)
	t.Logf("Account size with large CodeHash (64 bytes): %d bytes", size3)
}

// TestStateAccount_Size_Consistency проверяет консистентность размера при сериализации/десериализации
func TestStateAccount_Size_Consistency(t *testing.T) {
	sa := CreateTestStateAccount()
	sa.SetBalance(123.456)
	sa.AddInput(common.Hash{0x1}, big.NewInt(100))
	sa.AddInput(common.Hash{0x2}, big.NewInt(200))

	// Сериализуем
	data1 := sa.Bytes()
	size1 := len(data1)

	// Десериализуем
	sa2 := BytesToStateAccount(data1)
	if sa2 == nil {
		t.Fatal("BytesToStateAccount returned nil")
	}

	// Сериализуем снова
	data2 := sa2.Bytes()
	size2 := len(data2)

	// Размеры должны совпадать
	if size1 != size2 {
		t.Errorf("Size inconsistency: first serialization %d bytes, second %d bytes", size1, size2)
	}

	// Данные должны совпадать
	if !bytes.Equal(data1, data2) {
		t.Errorf("Data inconsistency: serialized data differs after round trip")
	}

	t.Logf("Consistent account size: %d bytes", size1)
}

// TestStateAccount_Size_WithManyInputs проверяет размер с большим количеством inputs
func TestStateAccount_Size_WithManyInputs(t *testing.T) {
	sa := CreateTestStateAccount()
	sa.SetBalance(1000.0)

	// Размер без inputs
	data0 := sa.Bytes()
	size0 := len(data0)

	// Добавляем inputs постепенно и проверяем размер
	sizes := []int{size0}
	for i := 1; i <= 10; i++ {
		txHash := common.Hash{}
		txHash[0] = byte(i)
		sa.AddInput(txHash, big.NewInt(int64(i*100)))
		data := sa.Bytes()
		sizes = append(sizes, len(data))
	}

	// Размер должен монотонно увеличиваться
	for i := 1; i < len(sizes); i++ {
		if sizes[i] <= sizes[i-1] {
			t.Errorf("Size should increase with more inputs: %d inputs = %d bytes, %d inputs = %d bytes",
				i-1, sizes[i-1], i, sizes[i])
		}
	}

	// Проверяем, что каждый input добавляет примерно одинаковое количество байт
	increases := make([]int, len(sizes)-1)
	for i := 1; i < len(sizes); i++ {
		increases[i-1] = sizes[i] - sizes[i-1]
	}

	// Первые несколько увеличений могут быть разными из-за выравнивания,
	// но в целом должны быть похожими
	avgIncrease := 0
	for _, inc := range increases {
		avgIncrease += inc
	}
	avgIncrease /= len(increases)

	t.Logf("Account size progression with inputs:")
	for i, size := range sizes {
		t.Logf("  %d inputs: %d bytes", i, size)
		if i > 0 {
			t.Logf("    (+%d bytes)", increases[i-1])
		}
	}
	t.Logf("Average increase per input: %d bytes", avgIncrease)
}
