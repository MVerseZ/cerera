package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
	"golang.org/x/crypto/blake2b"

	"github.com/akrylysov/pogreb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"go.uber.org/zap"
)

// vltlogger returns a sugared logger for the vault package.
// It is defined as a function (not a global variable) so that it always
// uses the logger configured by logger.Init(), even if this package is
// imported before logging is set up in main().
func vltlogger() *zap.SugaredLogger {
	return logger.Named("vault")
}

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
	Close() error
	// Contract code storage methods
	StoreContractCode(address types.Address, code []byte) error
	GetContractCode(address types.Address) ([]byte, error)
	HasContractCode(address types.Address) bool
	
	// Contract storage methods (key-value storage)
	SetStorage(address types.Address, key *big.Int, value *big.Int) error
	GetStorage(address types.Address, key *big.Int) (*big.Int, error)
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

	// Faucet tracking
	faucetLastRequest map[types.Address]time.Time
	faucetMu          sync.RWMutex

	// Pogreb database (only for non-in-memory mode)
	db   *pogreb.DB
	dbMu sync.RWMutex
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
		accounts:          GetAccountsTrie(),
		rootHash:          common.EmptyHash(),
		inMem:             cfg.IN_MEM,
		faucetLastRequest: make(map[types.Address]time.Time),
		// stChan:   make(chan [32]byte),
	}

	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	seed := bip39.NewSeed(mnemonic, "NODE_PASS")
	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()
	var mpub [78]byte
	pubKeyStr := publicKey.B58Serialize()
	copy(mpub[:], []byte(pubKeyStr))

	vltlogger().Infow("Init vault",
		"address", rootHashAddress.String(),
		"service", VAULT_SERVICE_NAME,
	)
	vltlogger().Infow("Vault recovery mnemonic generated", "mnemonic", mnemonic)
	vltlogger().Infow("Vault recovery password generated", "password", "NODE_PASS")
	vltlogger().Infow("Vault master key generated", "masterKey", masterKey.B58Serialize())
	vltlogger().Infow("Vault public key generated", "publicKey", publicKey.B58Serialize())
	// vltlogger.Printf("%s\r\n", cfg.NetCfg.PRIV)

	rootSA := &types.StateAccount{
		Address: rootHashAddress,
		Nonce:   1,
		// Balance:  types.FloatToBigInt(coinbase.InitialNodeBalance),
		Root: vlt.rootHash,
		CodeHash: types.EncodePrivateKeyToByte(
			types.DecodePrivKey(cfg.NetCfg.PRIV),
		),
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

	if vlt.inMem {
		vltlogger().Infow("Vault running in memory mode")
		vlt.accounts.Append(rootSA.Address, rootSA)
		vlt.status = 0xa
		vaultAccountsTotal.Set(float64(vlt.accounts.Size()))
		vlt.updateSupplyMetrics()

		return &vlt, nil
	}

	// Initialize vault path if not set (pogreb uses directory, not file)
	if cfg.Vault.PATH == "EMPTY" {
		cfg.UpdateVaultPath("./vault")
	}

	vlt.path = cfg.Vault.PATH

	// Open pogreb database once and store it
	dbDir := vlt.path
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		vltlogger().Errorw("Failed to create vault directory", "path", dbDir, "err", err)
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}

	db, err := pogreb.Open(dbDir, nil)
	if err != nil {
		vltlogger().Errorw("Failed to open pogreb database", "path", dbDir, "err", err)
		return nil, fmt.Errorf("failed to open pogreb database: %w", err)
	}
	vlt.db = db

	// Check if rootSA already exists in database
	key := rootSA.Address.Bytes()
	has, err := db.Has(key)
	if err != nil {
		vltlogger().Errorw("Failed to check if root account exists", "err", err)
		db.Close()
		return nil, fmt.Errorf("failed to check if root account exists: %w", err)
	}
	if !has {
		// Create new vault with rootSA
		accountData := rootSA.Bytes()
		if err := db.Put(key, accountData); err != nil {
			vltlogger().Errorw("Failed to write root account", "err", err)
			db.Close()
			return nil, fmt.Errorf("failed to write root account: %w", err)
		}
		// Add rootSA to accounts
		vlt.accounts.Append(rootSA.Address, rootSA)
		vltlogger().Info("Created new vault with root account")
	} else {
		// Sync with existing vault
		if err := vlt.SyncFromDB(); err != nil {
			vltlogger().Errorw("Failed to sync vault", "err", err)
			db.Close()
			return nil, fmt.Errorf("failed to sync vault: %w", err)
		}
		// Ensure rootSA is in accounts after sync (may not exist in old vaults)
		if vlt.accounts.GetAccount(rootSA.Address) == nil {
			vlt.accounts.Append(rootSA.Address, rootSA)
			if err := db.Put(key, rootSA.Bytes()); err != nil {
				vltlogger().Errorw("Failed to save root account", "err", err)
				db.Close()
				return nil, fmt.Errorf("failed to save root account: %w", err)
			}
		}
		vltlogger().Info("Synced existing vault")
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

	codeHash := types.EncodePrivateKeyToByte(privateKey)
	var mpub [78]byte
	pubKeyStr := publicKey.B58Serialize()
	copy(mpub[:], []byte(pubKeyStr))
	newAccount := &types.StateAccount{
		Address: address,
		Nonce:   1,
		//Balance:  types.FloatToBigInt(100.0),
		Root:     v.rootHash,
		CodeHash: codeHash,
		Status:   0, // 0: OP_ACC_NEW
		Bloom:    []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
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

	if !v.inMem && v.db != nil {
		key := newAccount.Address.Bytes()
		if err := v.db.Put(key, newAccount.Bytes()); err != nil {
			vltlogger().Errorw("Failed to save account to vault",
				"address", address.Hex(),
				"err", err,
			)
			return "", "", "", nil, fmt.Errorf("failed to save account to vault: %w", err)
		}
		vltlogger().Infow("Account saved to vault", "address", address.Hex())
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
		return types.EmptyAddress(), "", "", ErrMnemonicEmpty
	}

	// Sync vault from database if not in memory mode
	if !v.inMem && v.db != nil {
		if err := v.SyncFromDB(); err != nil {
			return types.EmptyAddress(), "", "", fmt.Errorf("failed to sync vault: %w", err)
		}
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
		vltlogger().Warnw("GetKey: failed to deserialize public key",
			"signKey", signKey,
			"err", err,
		)
		return []byte{0x0, 0x0, 0xf, 0xf}
	}

	var fp = v.accounts.GetKBytes(pubKey)

	if fp != nil {
		return fp
	} else {
		vltlogger().Warnw("GetKey: key not found in accounts", "signKey", signKey)
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
	if v.inMem || v.db == nil {
		return int64(v.accounts.Size())
	}
	// Pogreb doesn't have Stats() method, so we count items manually
	count := int64(0)
	it := v.db.Items()
	for {
		_, _, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			return -1
		}
		count++
	}
	return count
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

	if !v.inMem && v.db != nil {
		destKey := saDest.Address.Bytes()
		fromKey := saFrom.Address.Bytes()
		if err := v.db.Put(destKey, saDest.Bytes()); err != nil {
			vltlogger().Errorw("Failed to update destination account in database", "err", err)
		}
		if err := v.db.Put(fromKey, saFrom.Bytes()); err != nil {
			vltlogger().Errorw("Failed to update source account in database", "err", err)
		}
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

	if !v.inMem && v.db != nil {
		destKey := saDest.Address.Bytes()
		if err := v.db.Put(destKey, saDest.Bytes()); err != nil {
			vltlogger().Errorw("Failed to update account in database", "err", err)
		}
	}

	v.updateSupplyMetrics()
	return saDest, nil
}

// DropFaucet mints new coins directly to the destination account with faucet metrics.
func (v *D5Vault) DropFaucet(to types.Address, cnt *big.Int, txHash common.Hash) error {
	// Check amount limits
	if cnt == nil || cnt.Sign() <= 0 {
		return ErrInvalidMintAmount
	}

	// Check minimum amount
	if cnt.Cmp(coinbase.MinFaucetAmount) < 0 {
		return fmt.Errorf("faucet amount %s is below minimum %s", cnt.String(), coinbase.MinFaucetAmount.String())
	}

	// Check maximum amount
	if cnt.Cmp(coinbase.MaxFaucetAmount) > 0 {
		return fmt.Errorf("faucet amount %s exceeds maximum %s", cnt.String(), coinbase.MaxFaucetAmount.String())
	}

	// Initialize faucetLastRequest if nil (safety check)
	v.faucetMu.Lock()
	if v.faucetLastRequest == nil {
		v.faucetLastRequest = make(map[types.Address]time.Time)
	}
	v.faucetMu.Unlock()

	// Check cooldown period
	v.faucetMu.RLock()
	lastRequest, exists := v.faucetLastRequest[to]
	v.faucetMu.RUnlock()

	if exists {
		cooldownDuration := time.Duration(coinbase.FaucetCooldownHours) * time.Hour
		timeSinceLastRequest := time.Since(lastRequest)
		if timeSinceLastRequest < cooldownDuration {
			remainingTime := cooldownDuration - timeSinceLastRequest
			return fmt.Errorf("faucet cooldown period not expired: please wait %v before next request", remainingTime.Round(time.Minute))
		}
	}

	// Mint the coins
	if _, err := v.creditMintedAmount(to, cnt, txHash); err != nil {
		return err
	}

	// Update last request time
	v.faucetMu.Lock()
	v.faucetLastRequest[to] = time.Now()
	v.faucetMu.Unlock()

	// Update metrics
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

// SyncFromDB loads all accounts from the pogreb database into memory
func (v *D5Vault) SyncFromDB() error {
	if v.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if v.accounts == nil {
		v.accounts = GetAccountsTrie()
	}

	v.accounts.Clear()

	// Iterate over all items in the database
	it := v.db.Items()
	for {
		key, accountData, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			vltlogger().Errorw("syncFromDB: failed to get next item", "err", err)
			continue
		}

		// Try to deserialize account, skip on error
		func() {
			defer func() {
				if r := recover(); r != nil {
					vltlogger().Warnw("Skipping corrupted account data",
						"reason", r,
						"key", fmt.Sprintf("%x", key),
						"length", len(accountData),
					)
				}
			}()
			account := types.BytesToStateAccount(accountData)
			if account != nil {
				vltlogger().Infow("Read account from pogreb vault", "address", account.Address.Hex())
				v.accounts.Append(account.Address, account)
			} else {
				previewLen := 20
				if len(accountData) < previewLen {
					previewLen = len(accountData)
				}
				vltlogger().Errorw("Failed to deserialize account",
					"key", fmt.Sprintf("%x", key),
					"length", len(accountData),
					"preview", fmt.Sprintf("%x", accountData[:previewLen]),
				)
			}
		}()
	}

	return nil
}

// Close closes the pogreb database
func (v *D5Vault) Close() error {
	v.dbMu.Lock()
	defer v.dbMu.Unlock()

	if v.db == nil {
		vltlogger().Infow("Close(): database is already closed or not initialized")
		return nil
	}

	// Close the database (pogreb handles syncing internally)
	if err := v.db.Close(); err != nil {
		vltlogger().Errorw("Close(): error closing database", "err", err)
		v.db = nil
		return fmt.Errorf("failed to close pogreb database: %w", err)
	}

	vltlogger().Infow("Close(): pogreb database closed successfully")
	v.db = nil
	return nil
}

// StoreContractCode сохраняет байткод контракта в хранилище
// Использует префикс "code:" для ключа в pogreb
func (v *D5Vault) StoreContractCode(address types.Address, code []byte) error {
	if len(code) == 0 {
		return fmt.Errorf("contract code cannot be empty")
	}

	// Вычисляем хеш кода
	hash, err := blake2b.New256(nil)
	if err != nil {
		return fmt.Errorf("failed to create hash: %w", err)
	}
	hash.Write(code)
	codeHash := hash.Sum(nil)

	// Обновляем CodeHash в StateAccount
	account := v.Get(address)
	if account == nil {
		// Создаем новый аккаунт для контракта
		account = types.NewStateAccount(address, 0, v.rootHash)
		account.CodeHash = codeHash
		// Устанавливаем тип как контракт (Type = 5 для контракта, если нужно)
		// Пока оставляем Type = 0, но CodeHash будет указывать на наличие кода
		v.accounts.Append(address, account)
		vaultAccountsTotal.Set(float64(v.accounts.Size()))
	} else {
		// Обновляем CodeHash существующего аккаунта
		account.CodeHash = codeHash
		v.accounts.Append(address, account)
	}

	// Сохраняем код в pogreb с префиксом "code:"
	if !v.inMem && v.db != nil {
		v.dbMu.Lock()
		defer v.dbMu.Unlock()

		// Ключ: "code:" + address.Bytes()
		key := append([]byte("code:"), address.Bytes()...)
		if err := v.db.Put(key, code); err != nil {
			vltlogger().Errorw("Failed to store contract code", "address", address.Hex(), "err", err)
			return fmt.Errorf("failed to store contract code: %w", err)
		}

		// Также сохраняем обновленный аккаунт
		accountKey := address.Bytes()
		if err := v.db.Put(accountKey, account.Bytes()); err != nil {
			vltlogger().Errorw("Failed to update account with code hash", "address", address.Hex(), "err", err)
			return fmt.Errorf("failed to update account: %w", err)
		}

		vltlogger().Infow("Stored contract code", "address", address.Hex(), "codeSize", len(code), "codeHash", fmt.Sprintf("%x", codeHash))
	}

	return nil
}

// GetContractCode получает байткод контракта из хранилища
func (v *D5Vault) GetContractCode(address types.Address) ([]byte, error) {
	// Проверяем, есть ли CodeHash в аккаунте
	account := v.Get(address)
	if account == nil || len(account.CodeHash) == 0 {
		return nil, fmt.Errorf("contract code not found for address %s", address.Hex())
	}

	// Если in-memory режим, код должен быть в памяти (но у нас нет такого хранилища)
	// Для in-memory нужно будет добавить отдельное хранилище
	if v.inMem {
		// В in-memory режиме код не хранится, возвращаем ошибку
		// TODO: добавить in-memory хранилище кода контрактов
		return nil, fmt.Errorf("contract code storage not available in in-memory mode")
	}

	if v.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	v.dbMu.RLock()
	defer v.dbMu.RUnlock()

	// Ключ: "code:" + address.Bytes()
	key := append([]byte("code:"), address.Bytes()...)

	// Проверяем, существует ли ключ
	has, err := v.db.Has(key)
	if err != nil {
		vltlogger().Errorw("Failed to check contract code existence", "address", address.Hex(), "err", err)
		return nil, fmt.Errorf("failed to check contract code: %w", err)
	}
	if !has {
		return nil, fmt.Errorf("contract code not found for address %s", address.Hex())
	}

	code, err := v.db.Get(key)
	if err != nil {
		vltlogger().Errorw("Failed to get contract code", "address", address.Hex(), "err", err)
		return nil, fmt.Errorf("failed to get contract code: %w", err)
	}

	// Проверяем хеш кода
	hash, err := blake2b.New256(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash: %w", err)
	}
	hash.Write(code)
	computedHash := hash.Sum(nil)

	// Сравниваем с сохраненным хешем
	if len(account.CodeHash) > 0 {
		if !bytes.Equal(computedHash, account.CodeHash) {
			vltlogger().Warnw("Contract code hash mismatch", "address", address.Hex(),
				"expected", fmt.Sprintf("%x", account.CodeHash),
				"computed", fmt.Sprintf("%x", computedHash))
			// Не возвращаем ошибку, но логируем предупреждение
		}
	}

	return code, nil
}

// HasContractCode проверяет, есть ли код контракта для данного адреса
func (v *D5Vault) HasContractCode(address types.Address) bool {
	account := v.Get(address)
	return account != nil && len(account.CodeHash) > 0
}

// SetStorage сохраняет значение в storage контракта
// key и value - это 32-байтные значения (big.Int)
func (v *D5Vault) SetStorage(address types.Address, key *big.Int, value *big.Int) error {
	if key == nil {
		return fmt.Errorf("storage key cannot be nil")
	}
	if value == nil {
		value = big.NewInt(0)
	}

	// Получаем или создаем аккаунт контракта
	account := v.Get(address)
	if account == nil {
		// Создаем новый аккаунт для контракта
		account = types.NewStateAccount(address, 0, v.rootHash)
		v.accounts.Append(address, account)
		vaultAccountsTotal.Set(float64(v.accounts.Size()))
	}

	// Сохраняем в pogreb с ключом "storage:" + address + ":" + key
	if !v.inMem && v.db != nil {
		v.dbMu.Lock()
		defer v.dbMu.Unlock()

		// Ключ: "storage:" + address.Bytes() + ":" + key.Bytes()
		keyBytes := make([]byte, 32)
		key.FillBytes(keyBytes) // Заполняет big-endian, дополняя нулями слева
		
		storageKey := append([]byte("storage:"), address.Bytes()...)
		storageKey = append(storageKey, ':')
		storageKey = append(storageKey, keyBytes...)

		// Значение: value.Bytes() (32 байта)
		valueBytes := make([]byte, 32)
		value.FillBytes(valueBytes)

		if err := v.db.Put(storageKey, valueBytes); err != nil {
			vltlogger().Errorw("Failed to set storage", 
				"address", address.Hex(),
				"key", key.Text(16),
				"err", err,
			)
			return fmt.Errorf("failed to set storage: %w", err)
		}

		vltlogger().Debugw("Storage set",
			"address", address.Hex(),
			"key", key.Text(16),
			"value", value.Text(16),
		)
	}

	return nil
}

// GetStorage получает значение из storage контракта
func (v *D5Vault) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	if key == nil {
		return nil, fmt.Errorf("storage key cannot be nil")
	}

	// Если in-memory режим, возвращаем 0 (storage не поддерживается в памяти)
	if v.inMem {
		return big.NewInt(0), nil
	}

	if v.db == nil {
		return big.NewInt(0), nil
	}

	v.dbMu.RLock()
	defer v.dbMu.RUnlock()

	// Ключ: "storage:" + address.Bytes() + ":" + key.Bytes()
	keyBytes := make([]byte, 32)
	key.FillBytes(keyBytes)
	
	storageKey := append([]byte("storage:"), address.Bytes()...)
	storageKey = append(storageKey, ':')
	storageKey = append(storageKey, keyBytes...)

	// Проверяем, существует ли ключ
	has, err := v.db.Has(storageKey)
	if err != nil {
		vltlogger().Errorw("Failed to check storage existence",
			"address", address.Hex(),
			"key", key.Text(16),
			"err", err,
		)
		return big.NewInt(0), nil // Возвращаем 0 при ошибке
	}
	if !has {
		return big.NewInt(0), nil // Возвращаем 0 если ключ не найден
	}

	valueBytes, err := v.db.Get(storageKey)
	if err != nil {
		vltlogger().Errorw("Failed to get storage",
			"address", address.Hex(),
			"key", key.Text(16),
			"err", err,
		)
		return big.NewInt(0), nil // Возвращаем 0 при ошибке
	}

	// Конвертируем байты в big.Int
	value := new(big.Int).SetBytes(valueBytes)
	return value, nil
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
			return err.Error()
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
	case "getInfo":
		return "Not implemented"
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
		account := v.Get(types.HexToAddress(addr))
		if account == nil {
			return make(map[common.Hash]*big.Int) // Возвращаем пустую map
		}
		// Возвращаем копию инпутов без mutex для безопасной сериализации в JSON
		return account.GetAllInputs()
	}
	return nil
}
