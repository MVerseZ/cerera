package types

import (
	"encoding/json"
	"math/big"
	"reflect"
	"sync"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/tyler-smith/go-bip39"
)

func CreateTestStateAccount() StateAccount {
	var name = "a"
	var pass = "aaa"
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	// seed := bip39.NewSeed(mnemonic, pass)

	// masterKey, _ := bip32.NewMasterKey(seed)
	// publicKey := masterKey.PublicKey()

	privateKey, _ := GenerateAccount()

	pubkey := &privateKey.PublicKey
	address := PubkeyToAddress(*pubkey)
	derBytes := EncodePrivateKeyToByte(privateKey)
	// derBytes, _ := x509.MarshalECPrivateKey(privateKey)

	var walletName string
	if name != "" {
		walletName = name
	} else {
		walletName = address.String()
	}

	newAccount := StateAccount{
		Address:  address,
		Name:     walletName,
		Nonce:    1,
		Balance:  FloatToBigInt(0.0),
		Root:     common.Hash(address.Bytes()),
		CodeHash: derBytes,
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		// Inputs:     Input{M: make(map[common.Hash]*big.Int)},
		Passphrase: common.BytesToHash([]byte(pass)),
		Mnemonic:   mnemonic,
		// MPub:       publicKey,
		// MPriv:      masterKey,
	}
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
	sa := &StateAccount{
		Address:    Address{},
		Balance:    big.NewInt(100),
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Name:       "test",
		Nonce:      42,
		Root:       common.Hash{0x7},
		Status:     "active",
		Passphrase: common.Hash{0x8},
		MPub:       "public_key",
		Mnemonic:   "mnemonic phrase",
		Inputs:     &Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}

	// Marshal to bytes
	data := sa.Bytes()

	// Unmarshal back to a new StateAccount
	var sa2 StateAccount
	err := json.Unmarshal(data, &sa2)
	if err != nil {
		t.Fatalf("Bytes failed: could not unmarshal: %v", err)
	}

	// Compare the original and unmarshaled structs (excluding Inputs for now)
	sa2.Inputs = sa.Inputs // Inputs comparison needs separate handling due to mutex
	if !reflect.DeepEqual(sa, &sa2) {
		t.Errorf("Bytes failed: unmarshaled struct does not match original.\nGot: %+v\nWant: %+v", sa2, sa)
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
	sa := &StateAccount{
		Address:    Address{},
		Balance:    big.NewInt(100),
		Bloom:      []byte{0x1, 0x2, 0x3},
		CodeHash:   []byte{0x4, 0x5, 0x6},
		Name:       "test",
		Nonce:      42,
		Root:       common.Hash{0x7},
		Status:     "active",
		Passphrase: common.Hash{0x8},
		MPub:       "public_key",
		Mnemonic:   "mnemonic phrase",
		Inputs:     &Input{RWMutex: &sync.RWMutex{}, M: make(map[common.Hash]*big.Int)},
	}

	// Marshal to bytes
	data, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("TestBytesToStateAccount failed: could not marshal: %v", err)
	}

	// Convert bytes back to StateAccount
	sa2 := BytesToStateAccount(data)

	// Verify the fields (excluding Inputs for now)
	if !reflect.DeepEqual(sa.Address, sa2.Address) ||
		sa.Balance.Cmp(sa2.Balance) != 0 ||
		!reflect.DeepEqual(sa.Bloom, sa2.Bloom) ||
		!reflect.DeepEqual(sa.CodeHash, sa2.CodeHash) ||
		sa.Name != sa2.Name ||
		sa.Nonce != sa2.Nonce ||
		sa.Root != sa2.Root ||
		sa.Status != sa2.Status ||
		sa.Passphrase != sa2.Passphrase ||
		sa.MPub != sa2.MPub ||
		sa.Mnemonic != sa2.Mnemonic {
		t.Errorf("BytesToStateAccount failed: unmarshaled struct does not match original.\nGot: %+v\nWant: %+v", sa2, sa)
	}

	// Verify Inputs is initialized properly
	// if sa2.Inputs == nil || sa2.Inputs.M == nil || sa2.Inputs.RWMutex == nil {
	// 	t.Errorf("BytesToStateAccount failed: Inputs field not properly initialized: %+v", sa2.Inputs)
	// }
}
