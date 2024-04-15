package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

type Vault interface {
	CoinBase() *ecdsa.PrivateKey
	Create(name string, pass string) (string, string, *types.Address, error)
	Put(address types.Address, acc types.StateAccount)
	Get(types.Address) types.StateAccount
	GetAll() interface{}
	Size() int
	//
}

type D5Vault struct {
	accounts map[types.Address]types.StateAccount
	coinBase types.StateAccount
	rootHash common.Hash
}

var vlt D5Vault

func S() int {
	return vlt.Size()
}
func Sync() []byte {
	res := make([]byte, 0)
	for _, sa := range vlt.accounts {
		res = append(res, sa.Bytes()...)
	}
	return res
}
func GetVault() *D5Vault {
	return &vlt
}

// NewD5Vault initializes and returns a new D5Vault instance.
func NewD5Vault(netCfg *config.NetworkConfig) Vault {
	var rootHashAddress = netCfg.ADDR

	vlt = D5Vault{
		accounts: make(map[types.Address]types.StateAccount),
		rootHash: common.BytesToHash(rootHashAddress.Bytes()),
	}

	var inps = make([]common.Hash, 0)
	rootSA := types.StateAccount{
		Address:  netCfg.ADDR,
		Name:     netCfg.ADDR.String(),
		Nonce:    1,
		Balance:  types.FloatToBigInt(100.0),
		Root:     vlt.rootHash,
		CodeHash: types.EncodePrivateKeyToByte(types.DecodePrivKey(netCfg.PRIV)),
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		Inputs:   inps,
	}

	vlt.accounts[netCfg.ADDR] = rootSA
	vlt.coinBase = coinbase.CoinBaseStateAccount()
	return &vlt
}

// LoadFromFile loads encrypted data from a JSON file into the vault.
func (v *D5Vault) LoadFromFile(filename string, key []byte) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	decryptedData, err := decrypt(data, key)
	if err != nil {
		return err
	}

	err = json.Unmarshal(decryptedData, &v.accounts)
	if err != nil {
		return err
	}

	return nil
}

// SaveToFile encrypts and saves data from the vault to a JSON file.
func (v *D5Vault) SaveToFile(filename string, key []byte) error {
	data, err := json.Marshal(v.accounts)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(data, key)
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, encryptedData, 0644)
	if err != nil {
		return err
	}

	return nil
}

// Create - create an account to store and return it
func (v *D5Vault) Create(name string, pass string) (string, string, *types.Address, error) {
	privateKey, err := types.GenerateAccount()
	if err != nil {
		return "", "", nil, err
	}
	pubkey := &privateKey.PublicKey
	address := types.PubkeyToAddress(*pubkey)
	derBytes, _ := x509.MarshalECPrivateKey(privateKey)

	var walletName string
	if name != "" {
		walletName = name
	} else {
		walletName = address.String()
	}

	var inps = make([]common.Hash, 0)

	newAccount := types.StateAccount{
		Address:    address,
		Name:       walletName,
		Nonce:      1,
		Balance:    types.FloatToBigInt(0.0),
		Root:       v.rootHash,
		CodeHash:   derBytes,
		Status:     "OP_ACC_NEW",
		Bloom:      []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		Inputs:     inps,
		Passphrase: common.BytesToHash([]byte(pass)),
	}
	v.accounts[address] = newAccount

	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBytes})
	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(pubkey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})
	return string(pemEncoded), string(pemEncodedPub), &address, nil
}

func (v *D5Vault) Get(addr types.Address) types.StateAccount {
	return v.accounts[addr]
}
func (v *D5Vault) GetAll() interface{} {
	res := make(map[types.Address]float64)
	for addr, v := range v.accounts {
		res[addr] = types.BigIntToFloat(v.Balance)
	}
	return res
}
func (v *D5Vault) Put(address types.Address, acc types.StateAccount) {
	v.accounts[address] = acc
}
func (v *D5Vault) Size() int {
	return len(v.accounts)
}
func (v *D5Vault) UpdateBalance(from types.Address, to types.Address, cnt *big.Int, txHash common.Hash) {

	// decrement first
	// wtf big int sub only?

	fmt.Println("Update balance")
	var sa = v.Get(from)
	sa.Balance = big.NewInt(0).Sub(sa.Balance, cnt)
	v.accounts[from] = sa

	// increment second
	var saDest = v.Get(to)
	saDest.Balance = saDest.Balance.Add(saDest.Balance, cnt)
	// when increment, add input to account - tx hash
	saDest.Inputs = append(saDest.Inputs, txHash)
	v.accounts[to] = saDest
	// done
}
func (v *D5Vault) CheckRunnable(r *big.Int, s *big.Int, tx *types.GTransaction) bool {

	// ecdsa.Verify(publicKey, tx.Hash().Bytes(), r, s)

	return false
}

func (v *D5Vault) CoinBase() *ecdsa.PrivateKey {
	privateKey, err := x509.ParseECPrivateKey(v.coinBase.CodeHash)
	if err != nil {
		return nil
	}
	return privateKey
}

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}
