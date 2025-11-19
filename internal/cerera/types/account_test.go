package types

import (
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
		Address:  address,
		Nonce:    1,
		Root:     common.Hash(address.Bytes()),
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
		Address:    Address{0x1, 0x2, 0x3, 0x4},
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Nonce:      42,
		Root:       common.Hash{0x7, 0x8, 0x9},
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
		Address:    Address{0x1, 0x2, 0x3},
		Bloom:      []byte{0x4, 0x5, 0x6},
		CodeHash:   []byte{0x7, 0x8, 0x9},
		Nonce:      123,
		Root:       common.Hash{0xa, 0xb, 0xc},
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
		Address:    Address{0x1, 0x2, 0x3, 0x4},
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Nonce:      42,
		Root:       common.Hash{0x7, 0x8, 0x9},
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
		Address:    Address{0x1, 0x2},
		Bloom:      []byte{0x1, 0x2},
		CodeHash:   []byte{0x3, 0x4},
		Nonce:      1,
		Root:       common.Hash{0x5},
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
