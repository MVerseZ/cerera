package storage

import (
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type Vault interface {
	Create(name string, pass string) (string, string, string, *types.Address, error)
	Clear() error
	Prepare()
	Restore(mnemonic string, pass string) (string, string, error)
	Put(address types.Address, acc types.StateAccount)
	Get(types.Address) types.StateAccount
	GetCount() int
	GetAll() interface{}
	GetKey(signKey string) []byte
	GetOwner() types.StateAccount
	Size() int64
	Sync(saBytes []byte)
	//
}

type D5Vault struct {
	accounts  *AccountsTrie
	initiator types.StateAccount
	path      string
	rootHash  common.Hash
	inMem     bool
}

var vlt D5Vault

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

// NewD5Vault initializes and returns a new vault instance.
func NewD5Vault(cfg *config.Config) Vault {
	gob.Register(types.StateAccount{})
	var rootHashAddress = cfg.NetCfg.ADDR

	vlt = D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.BytesToHash(coinbase.GetCoinbaseAddress().Bytes()),
		inMem:    cfg.IN_MEM,
	}

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, "GENESISNODE")
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	fmt.Printf("Init vault with %s\r\n", rootHashAddress)

	rootSA := types.StateAccount{
		Address:  rootHashAddress,
		Name:     rootHashAddress.String(),
		Nonce:    1,
		Balance:  types.FloatToBigInt(coinbase.InitialNodeBalance),
		Root:     vlt.rootHash,
		CodeHash: types.EncodePrivateKeyToByte(types.DecodePrivKey(cfg.NetCfg.PRIV)),
		Status:   "OP_ACC_NODE",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs:   nil,
		MPub:     publicKey.B58Serialize(),
	}

	vlt.initiator = rootSA

	if vlt.inMem {
		var cbAcc = coinbase.CoinBaseStateAccount()
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), cbAcc)

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), faucetAddr)

		vlt.accounts.Append(rootSA.Address, rootSA)
	} else {
		// TO DO rewrite
		if _, err := os.Stat("./vault.dat"); errors.Is(err, os.ErrNotExist) || cfg.Vault.PATH == "EMPTY" {
			// path/to/whatever does not exist
			if err := InitSecureVault(rootSA); err != nil {
				panic(err)
			}
			cfg.UpdateVaultPath("./vault.dat")
		}

		vlt.path = cfg.Vault.PATH
		//sync with fc
		if err := SyncVault(cfg.Vault.PATH); err != nil {
			panic(err)
		}

		var cbAcc = coinbase.CoinBaseStateAccount()
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), cbAcc)
		SaveToVault(cbAcc.Bytes())

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), faucetAddr)
		SaveToVault(faucetAddr.Bytes())
	}

	return &vlt
}

func (v *D5Vault) Prepare() {

}

func (v *D5Vault) Clear() error {
	return v.accounts.Clear()
}

// Create - create an account to store and return it
func (v *D5Vault) Create(name string, pass string) (string, string, string, *types.Address, error) {

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, pass)

	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	privateKey, err := types.GenerateAccount()
	if err != nil {
		return "", "", "", nil, err
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

	newAccount := types.StateAccount{
		Address:    address,
		Name:       walletName,
		Nonce:      1,
		Balance:    types.FloatToBigInt(0.0),
		Root:       v.rootHash,
		CodeHash:   derBytes,
		Status:     "OP_ACC_NEW",
		Bloom:      []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs:     nil,
		Passphrase: common.BytesToHash([]byte(pass)),
		Mnemonic:   mnemonic,
		MPub:       publicKey.B58Serialize(),
		// MPriv:      masterKey,
	}
	v.accounts.Append(address, newAccount)
	// pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBytes})
	// x509EncodedPub, _ := x509.MarshalPKIXPublicKey(pubkey)
	// pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	if !vlt.inMem {
		SaveToVault(newAccount.Bytes())
	}

	return masterKey.B58Serialize(), publicKey.B58Serialize(), mnemonic, &address, nil
}

func (v *D5Vault) Restore(mnemonic string, pass string) (string, string, error) {
	// entropy := bip39.EntropyFromMnemonic(mnemonic)
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()
	return masterKey.B58Serialize(), publicKey.B58Serialize(), nil
}
func (v *D5Vault) Get(addr types.Address) types.StateAccount {
	return v.accounts.GetAccount(addr)
}
func (v *D5Vault) GetPos(pos int) types.StateAccount {
	return v.accounts.GetByIndex(int64(pos))
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

	if !v.inMem {
		SyncVault(v.path)
	}
	return v.accounts.GetAll()
}
func (v *D5Vault) Put(address types.Address, acc types.StateAccount) {
	v.accounts.Append(address, acc)
}
func (v *D5Vault) Size() int64 {
	var s, err = VaultSourceSize()
	if err != nil {
		return -1
	} else {
		return s
	}
}

func (v *D5Vault) UpdateBalance(from types.Address, to types.Address, cnt *big.Int, txHash common.Hash) {

	// decrement first
	// wtf big int sub only?

	fmt.Println("Update balance")
	var sa = v.Get(from)
	sa.Balance = sa.Balance.Sub(sa.Balance, cnt)
	// sa = v.accounts.GetAccount(from)

	// increment second
	var saDest = v.Get(to)
	saDest.Balance = saDest.Balance.Add(saDest.Balance, cnt)

	// when increment, add input to account - tx hash
	// saDest.Inputs = append(saDest.Inputs, txHash)
	// saDest = v.accounts.GetAccount(to)

	// done
	if !v.inMem {
		UpdateVault(saDest.Bytes())
		UpdateVault(sa.Bytes())
	}
}

// faucet method without creating transaction
func (v *D5Vault) FaucetBalance(to types.Address, cntval int) error {

	var destSA = v.Get(to)
	var faucetTo = coinbase.DropFaucet(cntval)

	destSA.Balance.Add(destSA.Balance, faucetTo)
	if !v.inMem {
		UpdateVault(destSA.Bytes())
	}

	coinbase.TotalValue.Sub(coinbase.TotalValue, types.FloatToBigInt(float64(cntval)))

	return nil
}

func (v *D5Vault) CheckRunnable(r *big.Int, s *big.Int, tx *types.GTransaction) bool {
	// ecdsa.Verify(publicKey, tx.Hash().Bytes(), r, s)
	return false
}

func (v *D5Vault) GetCount() int {
	return v.accounts.Size()
}

func (v *D5Vault) GetOwner() types.StateAccount {
	return v.initiator
}

func (v *D5Vault) Sync(saBytes []byte) {
	var sa = types.BytesToStateAccount(saBytes)
	v.accounts.Append(sa.Address, sa)
}
