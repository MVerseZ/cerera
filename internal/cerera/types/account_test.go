package types

import (
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/stretchr/testify/assert"
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

func TestBloomUp(t *testing.T) {
	account := StateAccount{
		Bloom: []byte{0x0, 0x1, 0x0},
	}
	account.BloomUp()
	assert.Equal(t, byte(0x2), account.Bloom[1], "BloomUp should increment the second byte")
}

func TestBloomDown(t *testing.T) {
	account := StateAccount{
		Bloom: []byte{0x0, 0x3, 0x0},
	}
	account.BloomDown()
	assert.Equal(t, byte(0x2), account.Bloom[1], "BloomDown should decrement the second byte")
}

func TestBytes(t *testing.T) {
	account := CreateTestStateAccount()
	data := account.Bytes()
	assert.NotNil(t, data, "Bytes should return non-nil result")
}

func TestBytesToStateAccount(t *testing.T) {
	account := CreateTestStateAccount()
	data := account.Bytes()
	newAccount := BytesToStateAccount(data)
	assert.Equal(t, account, newAccount, "BytesToStateAccount should return an account identical to the original")
}

func TestInputs(t *testing.T) {
	account := CreateTestStateAccount()
	dna := append(make([]byte, 0, 16), 0xf, 0xa, 0x42)
	to := HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    txValue,
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)
	account.AddInput(itx.Hash(), itx.Value())
	// assert.Equal(t, 1, account.In)
}
