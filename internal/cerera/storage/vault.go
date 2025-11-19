package storage

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// vltlogger is a dedicated console logger for chain
var vltlogger = log.New(os.Stdout, "[vault] ", log.LstdFlags|log.Lmicroseconds)

const VAULT_SERVICE_NAME = "D5_VAULT_CERERA_001_1_7"

// Error constants
var (
	ErrMnemonicEmpty             = errors.New("mnemonic phrase cannot be empty")
	ErrFailedCreateMasterKey     = errors.New("failed to create master key")
	ErrAccountNotFound           = errors.New("account not found")
	ErrFaucetAccountNotFound     = errors.New("faucet account not found")
	ErrFaucetInsufficientBalance = errors.New("faucet insufficient balance")
	ErrWrongCredentials          = errors.New("wrong credentials")
	ErrErrorParsingParameters    = errors.New("error parsing parameters")
	ErrWrongWordsCount           = errors.New("wrong words count")
	ErrErrorWhileRestore         = errors.New("error while restore")
	ErrErrorParsingAmount        = errors.New("error parsing amount")
	ErrFailedGenerateUniqueAddr  = errors.New("failed to generate unique address after retries (collision with system address)")
	ErrAddressAlreadyExists      = errors.New("address already exists in vault, bad collision")
	ErrInvalidMintAmount         = errors.New("invalid mint amount")
)

// vault store accounts data
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

var (
	vaultAccountsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vault_accounts_total",
		Help: "Total number of accounts in vault",
	})
	vaultCirculatingSupply = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vault_circulating_supply",
		Help: "Circulating token supply",
	})
	vaultTotalSupply = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vault_total_supply",
		Help: "Total minted token supply",
	})
	vaultFaucetAmountTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vault_faucet_amount_total",
		Help: "Total amount dispensed by faucet",
	})
	vaultTransfersTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "vault_transfers_total",
		Help: "Total number of balance transfer operations",
	})
)

func init() {
	prometheus.MustRegister(
		vaultAccountsTotal,
		vaultCirculatingSupply,
		vaultTotalSupply,
		vaultFaucetAmountTotal,
		vaultTransfersTotal,
	)
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

// GetTotalSupply returns the sum of all account balances currently stored in the vault.
func (v *D5Vault) GetTotalSupply() *big.Int {
	total := big.NewInt(0)
	v.accounts.mu.RLock()
	defer v.accounts.mu.RUnlock()
	for _, acc := range v.accounts.accounts {
		total.Add(total, acc.GetBalanceBI())
	}
	return total
}

// CheckSupplyLimit ensures that minting the requested amount won't exceed the configured cap.
func (v *D5Vault) CheckSupplyLimit(amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return ErrInvalidMintAmount
	}
	currentSupply := v.GetTotalSupply()
	newSupply := new(big.Int).Add(new(big.Int).Set(currentSupply), amount)
	if newSupply.Cmp(coinbase.TotalValue) > 0 {
		return fmt.Errorf("supply limit exceeded: requested %s, current %s, cap %s",
			amount.String(), currentSupply.String(), coinbase.TotalValue.String())
	}
	return nil
}

func (v *D5Vault) updateSupplyMetrics() {
	total := v.GetTotalSupply()
	totalFloat := types.BigIntToFloat(total)
	vaultTotalSupply.Set(totalFloat)
	vaultCirculatingSupply.Set(totalFloat)
}

// NewD5Vault initializes and returns a new vault instance.
func NewD5Vault(ctx context.Context, cfg *config.Config) (Vault, error) {
	gob.Register(types.StateAccount{})
	var rootHashAddress = cfg.NetCfg.ADDR

	vlt = D5Vault{
		accounts: GetAccountsTrie(),
		rootHash: common.EmptyHash(),
		inMem:    cfg.IN_MEM,
		// stChan:   make(chan [32]byte),
	}

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "GENESISNODE")
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()
	var mpub [78]byte
	copy(mpub[:], publicKey.B58Serialize())

	vltlogger.Printf("Init vault with %s_serv_%s\r\n", rootHashAddress, VAULT_SERVICE_NAME)
	// vltlogger.Printf("%s\r\n", cfg.NetCfg.PRIV)

	rootSA := &types.StateAccount{
		Address: rootHashAddress,
		Nonce:   1,
		// Balance:  types.FloatToBigInt(coinbase.InitialNodeBalance),
		Root: vlt.rootHash,
		// CodeHash: types.EncodePrivateKeyToByte(types.DecodePrivKey(cfg.NetCfg.PRIV)),
		Status: 3, // 3: OP_ACC_NODE
		Bloom:  []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		MPub: mpub,
	}
	rootSA.SetBalance(coinbase.InitialNodeBalance)

	vlt.initiator = rootSA

	// if vlt.inMem {
	// 	var cbAcc = coinbase.CoinBaseStateAccount()
	// 	vlt.accounts.Append(coinbase.GetCoinbaseAddress(), &cbAcc)

	// 	var faucetAddr = coinbase.FaucetAccount()
	// 	vlt.accounts.Append(coinbase.GetFaucetAddress(), &faucetAddr)

	// 	vlt.accounts.Append(rootSA.Address, rootSA)
	// } else {
	// Initialize vault path if not set
	if cfg.Vault.PATH == "EMPTY" {
		cfg.UpdateVaultPath("./vault.dat")
	}

	vlt.path = cfg.Vault.PATH

	// Check if vault file exists, if not create it
	_, err := os.Stat(cfg.Vault.PATH)
	if errors.Is(err, os.ErrNotExist) {
		// Create new vault with rootSA
		if err := InitSecureVault(rootSA, cfg.Vault.PATH); err != nil {
			panic(err)
		}
		// Add rootSA to accounts
		vlt.accounts.Append(rootSA.Address, rootSA)
	} else if err != nil {
		// Handle other errors (permissions, etc.)
		panic(fmt.Errorf("failed to check vault file: %w", err))
	} else {
		// Sync with existing vault
		if err := SyncVault(cfg.Vault.PATH); err != nil {
			panic(err)
		}
		// Ensure rootSA is in accounts after sync (may not exist in old vaults)
		if vlt.accounts.GetAccount(rootSA.Address) == nil {
			vlt.accounts.Append(rootSA.Address, rootSA)
			SaveToVault(rootSA.Bytes(), cfg.Vault.PATH)
		}
	}

	vlt.status = 0xa

	// init metrics
	vaultAccountsTotal.Set(float64(vlt.accounts.Size()))
	vlt.updateSupplyMetrics()

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

	privateKey, err = types.GenerateAccount()
	if err != nil {
		return "", "", "", nil, err
	}
	pubkey = &privateKey.PublicKey
	address = types.PubkeyToAddress(*pubkey)

	// Проверяем, не существует ли уже аккаунт с таким адресом
	if existing := v.accounts.GetAccount(address); existing != nil {
		return "", "", "", nil, fmt.Errorf("%w: %s", ErrAddressAlreadyExists, address.Hex())
	}

	// derBytes := types.EncodePrivateKeyToByte(privateKey)
	var mpub [78]byte
	copy(mpub[:], publicKey.B58Serialize())
	newAccount := &types.StateAccount{
		Address: address,
		Nonce:   1,
		//Balance:  types.FloatToBigInt(100.0),
		Root: v.rootHash,
		// CodeHash: derBytes,
		Status: 0, // 0: OP_ACC_NEW
		Bloom:  []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		Passphrase: common.BytesToHash([]byte(pass)),
		MPub:       mpub,
	}
	newAccount.SetBalance(0.0)
	v.accounts.Append(address, newAccount)
	vaultAccountsTotal.Set(float64(v.accounts.Size()))

	SaveToVault(newAccount.Bytes(), vlt.path)

	return masterKey.B58Serialize(), publicKey.B58Serialize(), mnemonic, &address, nil
}

// Restore - restore account by mnemonic phrase, return B58Serialized credentials
//
//	args: mnemonic:string, pass:string (see Create, not required)
//	return: address:types.Address, master key:string, public key:string, error:error
func (v *D5Vault) Restore(mnemonic string, pass string) (types.Address, string, string, error) {
	// Validate input parameters
	if mnemonic == "" {
		return types.EmptyAddress(), "", "", ErrMnemonicEmpty
	}

	// entropy := bip39.EntropyFromMnemonic(mnemonic)
	seed := bip39.NewSeed(mnemonic, pass)
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return types.EmptyAddress(), "", "", fmt.Errorf("%w: %v", ErrFailedCreateMasterKey, err)
	}
	publicKey := masterKey.PublicKey()

	addr, err := v.accounts.FindAddrByPub(publicKey.B58Serialize())
	if err != nil {
		return types.EmptyAddress(), "", "", fmt.Errorf("%w: %v", ErrAccountNotFound, err)
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
	if v.accounts.GetAccount(address) == nil {
		v.accounts.Append(address, acc)
		vaultAccountsTotal.Set(float64(v.accounts.Size()))
		return
	}
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

	// metrics: transfer count and supply (internal transfers don't change supply, but we still recompute for safety)
	vaultTransfersTotal.Inc()
}

func (v *D5Vault) creditMintedAmount(to types.Address, cnt *big.Int, txHash common.Hash) (*types.StateAccount, error) {
	if cnt == nil || cnt.Sign() <= 0 {
		return nil, ErrInvalidMintAmount
	}
	if err := v.CheckSupplyLimit(cnt); err != nil {
		return nil, err
	}

	var saDest = v.Get(to)
	if saDest == nil {
		saDest = types.NewStateAccount(to, 0, v.rootHash)
		v.accounts.Append(to, saDest)
		vaultAccountsTotal.Set(float64(v.accounts.Size()))
	}

	newBal := new(big.Int).Add(saDest.GetBalanceBI(), cnt)
	saDest.SetBalanceBI(newBal)
	saDest.AddInput(txHash, cnt)

	if !v.inMem {
		UpdateVault(saDest.Bytes(), v.path)
	}

	v.updateSupplyMetrics()
	return saDest, nil
}

// DropFaucet mints new coins directly to the destination account with faucet metrics.
func (v *D5Vault) DropFaucet(to types.Address, cnt *big.Int, txHash common.Hash) error {
	if _, err := v.creditMintedAmount(to, cnt, txHash); err != nil {
		return err
	}
	vaultFaucetAmountTotal.Add(types.BigIntToFloat(cnt))
	return nil
}

// RewardMiner mints new coins for the miner (coinbase transaction execution).
func (v *D5Vault) RewardMiner(to types.Address, cnt *big.Int, txHash common.Hash) error {
	_, err := v.creditMintedAmount(to, cnt, txHash)
	return err
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
		return types.EmptyAddress(), ErrAccountNotFound
	}
	if acc.Passphrase == common.BytesToHash([]byte(pass)) {
		return acc.Address, nil
	}
	return types.EmptyAddress(), ErrWrongCredentials
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
			return ErrErrorParsingParameters.Error()
		}
		if strings.Count(mnemonic, " ") != 23 {
			return ErrWrongWordsCount.Error()
		}
		addr, mk, pk, err := vlt.Restore(mnemonic, pass)
		if err != nil {
			return ErrErrorWhileRestore.Error()
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
			return ErrErrorParsingParameters.Error()
		}
		rAddr, err := v.VerifyAccount(types.HexToAddress(addr), pass)
		if err != nil {
			return false //"Error while verify"
		}
		// fmt.Println(rAddr.Hex(), addr)
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
			return ErrErrorParsingParameters.Error()
		}
		return v.Get(types.HexToAddress(addr)).GetBalance()
	case "faucet":
		addrStr, ok1 := params[0].(string)
		if !ok1 {
			return ErrErrorParsingParameters.Error()
		}

		addr := types.HexToAddress(addrStr)
		var amountBigInt *big.Int
		// Prefer exact decimal as string in params[1]
		if s, ok := params[1].(string); ok {
			wei, err := types.DecimalStringToWei(s)
			if err != nil {
				return err.Error()
			}
			amountBigInt = wei
		} else if f, ok := params[1].(float64); ok {
			amountBigInt = types.FloatToBigInt(f)
		} else {
			return ErrErrorParsingAmount.Error()
		}

		// Create a dummy transaction hash for faucet
		txHash := common.BytesToHash([]byte("faucet_" + addrStr))

		err := v.DropFaucet(addr, amountBigInt, txHash)
		if err != nil {
			return err.Error()
		}
		return "Faucet successful"
	case "inputs":
		addr, ok1 := params[0].(string)
		if !ok1 {
			return ErrErrorParsingParameters.Error()
		}
		return v.Get(types.HexToAddress(addr)).Inputs
	}
	return nil
}
