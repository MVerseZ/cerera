package storage

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const VAULT_SERVICE_NAME = "D5_VAULT_CERERA_001_1_7"

type Vault interface {
	Create(pass string) (string, string, string, *types.Address, error)
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
	ServiceName() string
	Sync(saBytes []byte)
	Status() byte
	VerifyAccount(address types.Address, pass string) (types.Address, error)
	Exec(method string, params []interface{}) interface{}
}

type D5Vault struct {
	accounts  *AccountsTrie
	initiator *types.StateAccount
	path      string
	rootHash  common.Hash
	inMem     bool
	status    byte

	// channels
	// stChan chan [32]byte
	Service_Name string
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
func NewD5Vault(ctx context.Context, cfg *config.Config) (Vault, error) {
	gob.Register(types.StateAccount{})
	var rootHashAddress = cfg.NetCfg.ADDR

	vlt = D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.BytesToHash(coinbase.GetCoinbaseAddress().Bytes()),
		inMem:    cfg.IN_MEM,
		// stChan:   make(chan [32]byte),
	}

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "GENESISNODE")
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	fmt.Printf("Init vault with %s\r\n\t%s\r\n", rootHashAddress, VAULT_SERVICE_NAME)
	fmt.Println("==========================================")
	fmt.Printf("%s\r\n", cfg.NetCfg.PRIV)
	fmt.Println("==========================================")

	rootSA := &types.StateAccount{
		Address: rootHashAddress,
		Nonce:   1,
		// Balance:  types.FloatToBigInt(coinbase.InitialNodeBalance),
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
	rootSA.SetBalance(coinbase.InitialNodeBalance)

	vlt.initiator = rootSA

	if vlt.inMem {
		var cbAcc = coinbase.CoinBaseStateAccount()
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), &cbAcc)

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), &faucetAddr)

		vlt.accounts.Append(rootSA.Address, rootSA)
	} else {
		// Initialize vault path if not set
		if cfg.Vault.PATH == "EMPTY" {
			cfg.UpdateVaultPath("./vault.dat")
		}

		vlt.path = cfg.Vault.PATH

		// Check if vault file exists, if not create it
		if _, err := os.Stat(cfg.Vault.PATH); errors.Is(err, os.ErrNotExist) {
			if err := InitSecureVault(rootSA, cfg.Vault.PATH); err != nil {
				panic(err)
			}
		} else {
			//sync with existing vault
			if err := SyncVault(cfg.Vault.PATH); err != nil {
				panic(err)
			}
		}

		var cbAcc = coinbase.CoinBaseStateAccount()
		vlt.accounts.Append(coinbase.GetCoinbaseAddress(), &cbAcc)
		SaveToVault(cbAcc.Bytes(), cfg.Vault.PATH)

		var faucetAddr = coinbase.FaucetAccount()
		vlt.accounts.Append(coinbase.GetFaucetAddress(), &faucetAddr)
		SaveToVault(faucetAddr.Bytes(), cfg.Vault.PATH)
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
func (v *D5Vault) Create(pass string) (string, string, string, *types.Address, error) {
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

	newAccount := &types.StateAccount{
		Address: address,
		Nonce:   1,
		//Balance:  types.FloatToBigInt(100.0),
		Root:     v.rootHash,
		CodeHash: derBytes,
		Status:   "OP_ACC_NEW",
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte(pass)),
		MPub:       publicKey.B58Serialize(),
	}
	newAccount.SetBalance(0.0)
	v.accounts.Append(address, newAccount)

	if !vlt.inMem {
		SaveToVault(newAccount.Bytes(), vlt.path)
	}

	return masterKey.B58Serialize(), publicKey.B58Serialize(), mnemonic, &address, nil
}

// Restore - restore account by mnemonic phrase, return B58Serialized credentials
//
//	args: mnemonic:string, pass:string (see Create, not required)
//	return: address:types.Address, master key:string, public key:string, error:error
func (v *D5Vault) Restore(mnemonic string, pass string) (types.Address, string, string, error) {
	// Validate input parameters
	if mnemonic == "" {
		return types.EmptyAddress(), "", "", errors.New("mnemonic phrase cannot be empty")
	}

	// entropy := bip39.EntropyFromMnemonic(mnemonic)
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return types.EmptyAddress(), "", "", fmt.Errorf("failed to create master key: %w", err)
	}
	publicKey := masterKey.PublicKey()

	addr, err := v.accounts.FindAddrByPub(publicKey.B58Serialize())
	if err != nil {
		return types.EmptyAddress(), "", "", fmt.Errorf("account not found: %w", err)
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
	pubKey, err := bip32.B58Deserialize(signKey)
	if err != nil || pubKey == nil {
		return []byte{0x0, 0x0, 0xf, 0xf}
	}

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
	var s, err = VaultSourceSize(v.path)
	if err != nil {
		return -1
	} else {
		return s
	}
}

func (v *D5Vault) UpdateBalance(from types.Address, to types.Address, cnt *big.Int, txHash common.Hash) {

	// Use big.Int arithmetic and ensure destination exists (shadow account)
	if cnt == nil || cnt.Sign() <= 0 {
		return
	}

	var saFrom = v.Get(from)
	if saFrom == nil {
		return
	}

	var saDest = v.Get(to)
	if saDest == nil {
		saDest = types.NewStateAccount(to, 0, v.rootHash)
		v.accounts.Append(to, saDest)
	}

	// from = from - cnt
	newFromBal := new(big.Int).Sub(saFrom.GetBalanceBI(), cnt)
	saFrom.SetBalanceBI(newFromBal)

	// to = to + cnt
	newToBal := new(big.Int).Add(saDest.GetBalanceBI(), cnt)
	saDest.SetBalanceBI(newToBal)

	saDest.AddInput(txHash, cnt)

	if !v.inMem {
		UpdateVault(saDest.Bytes(), v.path)
		UpdateVault(saFrom.Bytes(), v.path)
	}
}

// faucet method without creating transaction
func (v *D5Vault) DropFaucet(to types.Address, cnt *big.Int, txHash common.Hash) error {
	// Validate faucet request against limits
	if err := coinbase.CheckFaucetLimits(to, cnt); err != nil {
		return err
	}

	if cnt == nil || cnt.Sign() <= 0 {
		return nil
	}

	var sa = v.Get(coinbase.GetFaucetAddress())
	if sa == nil {
		return errors.New("faucet account not found")
	}

	// Check if faucet has enough balance
	if sa.GetBalanceBI().Cmp(cnt) < 0 {
		return errors.New("faucet insufficient balance")
	}

	var saDest = v.Get(to)
	if saDest == nil {
		saDest = types.NewStateAccount(to, 0, v.rootHash)
		v.accounts.Append(to, saDest)
	}

	// faucet = faucet - cnt
	sa.SetBalanceBI(new(big.Int).Sub(sa.GetBalanceBI(), cnt))
	// to = to + cnt
	saDest.SetBalanceBI(new(big.Int).Add(saDest.GetBalanceBI(), cnt))

	saDest.AddInput(txHash, cnt)

	// Record the faucet request for tracking
	coinbase.RecordFaucetRequest(to, cnt)

	if !v.inMem {
		UpdateVault(saDest.Bytes(), v.path)
		UpdateVault(sa.Bytes(), v.path)
	}

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
	if acc == nil {
		return types.EmptyAddress(), errors.New("account not found")
	}
	if acc.Passphrase == common.BytesToHash([]byte(pass)) {
		return acc.Address, nil
	}
	return types.EmptyAddress(), errors.New("wrong credentials")
}

func (v *D5Vault) ServiceName() string {
	return VAULT_SERVICE_NAME
}

func (v *D5Vault) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "getAll":
		return v.GetAll()
	case "getCount":
		return v.GetCount()
	case "create":
		passphraseStr, ok1 := params[0].(string)
		if !ok1 {
			return nil
		}
		mk, pk, m, addr, err := v.Create(passphraseStr)
		if err != nil {
			return nil
		}
		type res struct {
			Address  *types.Address `json:"address,omitempty"`
			Priv     string         `json:"priv,omitempty"`
			Pub      string         `json:"pub,omitempty"`
			Mnemonic string         `json:"mnemonic,omitempty"`
		}
		Result := &res{
			Address:  addr,
			Priv:     mk,
			Pub:      pk,
			Mnemonic: m,
		}
		return Result
	case "restore":
		mnemonic, ok1 := params[0].(string)
		pass, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return "Error parsing parameters"
		}
		if strings.Count(mnemonic, " ") != 23 {
			Result := "Wrong words count!"
			return Result
		}
		addr, mk, pk, err := vlt.Restore(mnemonic, pass)
		if err != nil {
			Result := "Error while restore"
			return Result
		}
		type res struct {
			Addr types.Address `json:"address,omitempty"`
			Priv string        `json:"priv,omitempty"`
			Pub  string        `json:"pub,omitempty"`
		}
		Result := &res{
			Priv: mk,
			Pub:  pk,
			Addr: addr,
		}
		return Result
	case "verify":
		addr, ok1 := params[0].(string)
		pass, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return "Error parsing parameters"
		}
		rAddr, err := v.VerifyAccount(types.HexToAddress(addr), pass)
		if err != nil {
			return false //"Error while verify"
		}
		fmt.Println(rAddr.Hex(), addr)
		// INSERT_YOUR_CODE
		// Compare the addresses byte by byte for equality
		if len(addr) != len(rAddr.Hex()) {
			return false
		}
		for i := range len(rAddr) {
			if rAddr[i] != types.HexToAddress(addr)[i] {
				return false
			}
		}
		return true
	case "getBalance":
		addr, ok1 := params[0].(string)
		if !ok1 {
			return "Error parsing parameters"
		}
		return v.Get(types.HexToAddress(addr)).GetBalance()
	case "faucet":
		addrStr, ok1 := params[0].(string)
		amount, ok2 := params[1].(float64)
		if !ok1 || !ok2 {
			return "Error parsing parameters"
		}

		addr := types.HexToAddress(addrStr)
		amountBigInt := types.FloatToBigInt(amount)

		// Create a dummy transaction hash for faucet
		txHash := common.BytesToHash([]byte("faucet_" + addrStr))

		err := v.DropFaucet(addr, amountBigInt, txHash)
		if err != nil {
			return err.Error()
		}

		return "Faucet successful"
	}
	return nil
}
