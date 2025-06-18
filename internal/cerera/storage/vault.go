package storage

import (
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"

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
	Restore(mnemonic string, pass string) (types.Address, string, string, error)
	Put(address types.Address, acc *types.StateAccount)
	Get(types.Address) *types.StateAccount
	GetCount() int
	GetAll() interface{}
	GetKey(signKey string) []byte
	GetOwner() *types.StateAccount
	Size() int64
	Sync(saBytes []byte)
	Status() byte
	VerifyAccount(address types.Address, pass string) (types.Address, error)
}

type D5Vault struct {
	accounts  *AccountsTrie
	initiator *types.StateAccount
	path      string
	rootHash  common.Hash
	inMem     bool
	status    byte
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
func NewD5Vault(cfg *config.Config) (Vault, error) {
	gob.Register(types.StateAccount{})
	var rootHashAddress = cfg.NetCfg.ADDR

	vlt = D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.BytesToHash(coinbase.GetCoinbaseAddress().Bytes()),
		inMem:    cfg.IN_MEM,
	}

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "GENESISNODE")
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	fmt.Printf("Init vault with %s\r\n", rootHashAddress)

	rootSA := &types.StateAccount{
		Address:  rootHashAddress,
		Name:     rootHashAddress.String(),
		Nonce:    1,
		Balance:  types.FloatToBigInt(coinbase.InitialNodeBalance),
		Root:     vlt.rootHash,
		CodeHash: types.EncodePrivateKeyToByte(types.DecodePrivKey(cfg.NetCfg.PRIV)),
		Status:   "OP_ACC_NODE",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		MPub: publicKey.B58Serialize(),
	}

	vlt.initiator = rootSA

	if vlt.inMem {
		var cbAcc = coinbase.CoinBaseStateAccount()
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), &cbAcc)

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), &faucetAddr)

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
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), &cbAcc)
		SaveToVault(cbAcc.Bytes())

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), &faucetAddr)
		SaveToVault(faucetAddr.Bytes())
	}
	vlt.status = 0xa

	return &vlt, nil
}

func (v *D5Vault) Prepare() {

}

func (v *D5Vault) Clear() error {
	return v.accounts.Clear()
}

// Create - create an account to store and return it (B58Serialized)
//
//	args: name:string, pass:string
//	return: master key:string, public key:string, mnemonic phrase:string, address:types.Address, error:error
func (v *D5Vault) Create(name string, pass string) (string, string, string, *types.Address, error) {
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
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

	var walletName string
	if name != "" {
		walletName = name
	} else {
		walletName = address.String()
	}

	newAccount := &types.StateAccount{
		Address:  address,
		Name:     walletName,
		Nonce:    1,
		Balance:  types.FloatToBigInt(100.0),
		Root:     v.rootHash,
		CodeHash: derBytes,
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte(pass)),
		Mnemonic:   mnemonic,
		MPub:       publicKey.B58Serialize(),
	}
	v.accounts.Append(address, newAccount)

	if !vlt.inMem {
		SaveToVault(newAccount.Bytes())
	}

	return masterKey.B58Serialize(), publicKey.B58Serialize(), mnemonic, &address, nil
}

// Restore - restore account by mnemonic phrase, return B58Serialized credentials
//
//	args: mnemonic:string, pass:string (see Create, not required)
//	return: address:types.Address, master key:string, public key:string, error:error
func (v *D5Vault) Restore(mnemonic string, pass string) (types.Address, string, string, error) {
	// entropy := bip39.EntropyFromMnemonic(mnemonic)
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()
	addr, err := v.accounts.FindAddrByPub(publicKey.B58Serialize())
	if err == nil {
		return types.EmptyAddress(), "", "", err
	}
	return addr, masterKey.B58Serialize(), publicKey.B58Serialize(), nil
}

// Get - get account by address
//
//	args: address:types.Address
//	return: account:types.StateAccount
func (v *D5Vault) Get(addr types.Address) *types.StateAccount {
	return v.accounts.GetAccount(addr)
}

// Get - get account by index
//
//	args: index:int64
//	return: account:types.StateAccount
func (v *D5Vault) GetPos(pos int64) *types.StateAccount {
	return v.accounts.GetByIndex(pos)
}

// GetKey - get public key from vault by string representation
//
//	args: public key:string
//	return: bytes representation of public key:[]byte
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

func (v *D5Vault) Put(address types.Address, acc *types.StateAccount) {
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

	// legacy type is like p2p transaction
	// if reciever does not exist - > create shadow
	// if reciever exist - > update

	// decrement first
	// wtf big int sub only?

	fmt.Printf("___VAULT_OPERATIONS___\r\n\tupdate balance of %s\r\n", to)
	// fmt.Printf("\tupdate balance from %s\r\n", from)
	var sa = v.Get(from)
	// fmt.Printf("\tbalance from: %d\r\n", sa.Balance)
	// fmt.Printf("\tamount to transfer: %d\r\n", cnt)
	sa.Balance = sa.Balance.Sub(sa.Balance, cnt)
	// increment second
	var saDest = v.Get(to)
	saDest.Balance = saDest.Balance.Add(saDest.Balance, cnt)
	saDest.AddInput(txHash, cnt)
	// fmt.Printf("\ttotal : \r\n\t\t%s ---> %s\r\n\t\t%d ---> %d\r\n",
	// 	sa.Address.Hex(), saDest.Address.Hex(), sa.Balance, saDest.Balance)
	// fmt.Printf("\tInputs len now: %d\r\n", len(saDest.Inputs.M))

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
func (v *D5Vault) DropFaucet(to types.Address, cnt *big.Int, txHash common.Hash) error {

	fmt.Printf("___VAULT_OPERATIONS___\r\n\tupdate balance of %s\r\n", to)
	// fmt.Printf("\tupdate balance from %s\r\n", coinbase.GetCoinbaseAddress().String())
	var sa = v.Get(coinbase.GetFaucetAddress())
	// fmt.Printf("\tbalance from: %d\r\n", sa.Balance)
	// fmt.Printf("\tamount to transfer: %d\r\n", cnt)
	sa.Balance = sa.Balance.Sub(sa.Balance, cnt)
	// increment second
	var saDest = v.Get(to)
	saDest.Balance = saDest.Balance.Add(saDest.Balance, cnt)
	saDest.AddInput(txHash, cnt)
	// fmt.Printf("\ttotal : \r\n\t\t%s ---> %s\r\n\t\t%d ---> %d\r\n",
	// sa.Address.Hex(), saDest.Address.Hex(), sa.Balance, saDest.Balance)
	// fmt.Printf("\tInputs len now: %d\r\n", len(saDest.Inputs.M))

	// when increment, add input to account - tx hash
	// saDest.Inputs = append(saDest.Inputs, txHash)
	// saDest = v.accounts.GetAccount(to)

	// done
	if !v.inMem {
		UpdateVault(saDest.Bytes())
		UpdateVault(sa.Bytes())
	}
	// coinbase.TotalValue.Sub(coinbase.TotalValue, types.FloatToBigInt(float64(cntval)))

	return nil
}

func (v *D5Vault) CheckRunnable(r *big.Int, s *big.Int, tx *types.GTransaction) bool {
	// ecdsa.Verify(publicKey, tx.Hash().Bytes(), r, s)
	return false
}

func (v *D5Vault) GetCount() int {
	return v.accounts.Size()
}

func (v *D5Vault) GetOwner() *types.StateAccount {
	return v.initiator
}

func (v *D5Vault) Sync(saBytes []byte) {
	var sa = types.BytesToStateAccount(saBytes)
	v.accounts.Append(sa.Address, sa)
}

func (v *D5Vault) Status() byte {
	return v.status
}

func (v *D5Vault) VerifyAccount(addr types.Address, pass string) (types.Address, error) {
	var acc = v.accounts.GetAccount(addr)
	if acc.Passphrase == common.BytesToHash([]byte(pass)) {
		return acc.Address, nil
	}
	return types.EmptyAddress(), errors.New("wrong credentials!")
}
