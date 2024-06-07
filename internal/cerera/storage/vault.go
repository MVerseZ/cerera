package storage

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"math/big"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type Vault interface {
	CoinBase() *ecdsa.PrivateKey
	Create(name string, pass string) (string, string, *types.Address, error)
	Put(address types.Address, acc types.StateAccount)
	Get(types.Address) types.StateAccount
	GetAll() interface{}
	GetKey(signKey string) []byte
	Size() int
	//
}

type D5Vault struct {
	accounts *AccountsTrie
	coinBase types.StateAccount
	rootHash common.Hash
}

var vlt D5Vault

func S() int {
	return vlt.Size()
}
func Sync() []byte {
	res := make([]byte, 0)
	for _, sa := range vlt.accounts.accounts {
		res = append(res, sa.Bytes()...)
	}
	return res
}
func GetVault() *D5Vault {
	return &vlt
}

// NewD5Vault initializes and returns a new D5Vault instance.
func NewD5Vault(cfg *config.Config) Vault {
	var rootHashAddress = cfg.NetCfg.ADDR

	vlt = D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.BytesToHash(rootHashAddress.Bytes()),
	}

	var inps = make([]common.Hash, 0)
	rootSA := types.StateAccount{
		Address:  rootHashAddress,
		Name:     rootHashAddress.String(),
		Nonce:    1,
		Balance:  types.FloatToBigInt(100.0),
		Root:     vlt.rootHash,
		CodeHash: types.EncodePrivateKeyToByte(types.DecodePrivKey(cfg.NetCfg.PRIV)),
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xa, 0x0, 0x0, 0x0, 0xf, 0xd, 0xd, 0xd, 0xd, 0xd},
		Inputs:   inps,
	}

	vlt.accounts.Append(rootHashAddress, rootSA)
	vlt.coinBase = coinbase.CoinBaseStateAccount()
	SavePair(rootSA.Address, rootSA.Bytes())
	return &vlt
}

// Create - create an account to store and return it
func (v *D5Vault) Create(name string, pass string) (string, string, *types.Address, error) {

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, pass)

	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	privateKey, err := types.GenerateAccount()
	if err != nil {
		return "", "", nil, err
	}
	pubkey := &privateKey.PublicKey
	address := types.PubkeyToAddress(*pubkey)
	derBytes := types.EncodePrivateKeyToByte(privateKey)
	// derBytes, _ := x509.MarshalECPrivateKey(privateKey)

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
		Mnemonic:   mnemonic,
		MPub:       publicKey,
		MPriv:      masterKey,
	}
	v.accounts.Append(address, newAccount)

	// pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBytes})
	// x509EncodedPub, _ := x509.MarshalPKIXPublicKey(pubkey)
	// pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	SavePair(address, newAccount.Bytes())

	return publicKey.B58Serialize(), mnemonic, &address, nil
}

func (v *D5Vault) Get(addr types.Address) types.StateAccount {
	return v.accounts.GetAccount(addr)
}
func (v *D5Vault) GetKey(signKey string) []byte {
	pubKey, _ := bip32.B58Deserialize(signKey)

	var fp = v.accounts.GetKBytes(pubKey)

	if fp != nil {
		return fp
	} else {
		return []byte{0x0, 0x0, 0xf, 0xf}
	}
}
func (v *D5Vault) GetAll() interface{} {
	// refactor
	// this function returns all active (register) addressses with balance
	// [addr1:balance1, addr2:balance2, ..., addrN:balanceN]
	res := make(map[types.Address]float64)
	for addr, v := range v.accounts.accounts {
		res[addr] = types.BigIntToFloat(v.Balance)
	}
	return res
}
func (v *D5Vault) Put(address types.Address, acc types.StateAccount) {
	v.accounts.Append(address, acc)
}
func (v *D5Vault) Size() int {
	return v.accounts.Size()
}
func (v *D5Vault) UpdateBalance(from types.Address, to types.Address, cnt *big.Int, txHash common.Hash) {

	// decrement first
	// wtf big int sub only?

	fmt.Println("Update balance")
	var sa = v.Get(from)
	sa.Balance = big.NewInt(0).Sub(sa.Balance, cnt)
	sa = v.accounts.GetAccount(from)

	// increment second
	var saDest = v.Get(to)
	saDest.Balance = saDest.Balance.Add(saDest.Balance, cnt)
	// when increment, add input to account - tx hash
	saDest.Inputs = append(saDest.Inputs, txHash)
	saDest = v.accounts.GetAccount(to)
	// done
}

// faucet method without creating transaction
func (v *D5Vault) FaucetBalance(to types.Address, val *big.Int) {
	var destAddr = v.Get(to)
	destAddr.Balance.Add(destAddr.Balance, val)
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
