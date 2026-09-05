package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"golang.org/x/crypto/blake2b"

	"github.com/akrylysov/pogreb"
	"github.com/prometheus/client_golang/prometheus"
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
const EMPTY_PATH = "EMPTY"
const DEFAULT_VAULT_PATH = "./vault"

// Error constants
var (
	ErrMnemonicEmpty             = errors.New("mnemonic phrase cannot be empty")
	ErrFailedCreateMasterKey     = errors.New("failed to create master key")
	ErrFailedSerializePublicKey  = errors.New("failed to serialize public key")
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
	Restore(mnemonic string, pass string) (types.Address, string, error)
	Put(address types.Address, acc *account.StateAccount)
	Get(types.Address) *account.StateAccount
	GetCount() int
	GetAll() any
	// GetKey(signKey string) []byte
	GetOwner() *account.StateAccount
	Size() int64
	ServiceName() string
	Sync(saBytes []byte)
	Status() byte
	VerifyAccount(address types.Address, pass string) (types.Address, error)
	Exec(method string, params []any) any
	Close() error
	// Contract code storage methods
	StoreContractCode(address types.Address, code []byte) error
	GetContractCode(address types.Address) ([]byte, error)
	HasContractCode(address types.Address) bool
	DeleteContractCode(address types.Address) error

	// Contract storage methods (key-value storage)
	SetStorage(address types.Address, key *big.Int, value *big.Int) error
	GetStorage(address types.Address, key *big.Int) (*big.Int, error)

	Methods() map[string]service.RPCHandler
}

type D5Vault struct {
	accounts  *AccountIndex
	initiator *account.StateAccount
	path      string
	rootHash  common.Hash
	inMem     bool
	status    byte // 0xf status means error

	Service_Name string

	faucetLastRequest map[types.Address]time.Time
	faucetMu          sync.RWMutex

	db   *pogreb.DB
	dbMu sync.RWMutex

	dirty   map[types.Address]struct{}
	dirtyMu sync.Mutex

	contractCode    map[types.Address][]byte
	contractStorage map[types.Address]map[string]*big.Int
	contractMu      sync.RWMutex

	baselineSnapshot AccountSnapshot
}

// AccountCreatedHook is invoked with StateAccount.Bytes() after a successful Vault.Create.
// The node wires this to P2P (e.g. GossipSub) so peers can merge the same account state.
var AccountCreatedHook func(accountBytes []byte)

func notifyAccountCreated(sa *account.StateAccount) {
	if AccountCreatedHook == nil || sa == nil {
		return
	}
	b := sa.Bytes()
	if len(b) == 0 {
		return
	}
	AccountCreatedHook(b)
}

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
	gob.Register(account.StateAccount{})
	var rootHashAddress = cfg.NetCfg.ADDR

	v := &D5Vault{
		accounts:          NewAccountIndex(),
		rootHash:          common.EmptyHash(),
		inMem:             cfg.IN_MEM,
		faucetLastRequest: make(map[types.Address]time.Time),
		dirty:             make(map[types.Address]struct{}),
		contractCode:      make(map[types.Address][]byte),
		contractStorage:   make(map[types.Address]map[string]*big.Int),
	}

	vltlogger().Infow("Init vault",
		"address", rootHashAddress.String(),
		"service", VAULT_SERVICE_NAME,
	)

	rootSA := &account.StateAccount{
		StateAccountData: account.StateAccountData{
			Address: rootHashAddress,
			Nonce:   1,
			Root:    v.rootHash,
		},
		Status: 3, // 3: OP_ACC_NODE
		Bloom:  []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Inputs: &account.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	rootSA.SetBalance(coinbase.InitialNodeBalance)

	v.initiator = rootSA

	if v.inMem {
		if err := InitVaultKeys(v); err != nil {
			return nil, err
		}
		vltlogger().Infow("Vault running in memory mode")
		v.accounts.Append(rootSA.Address, rootSA)
		v.status = 0xa
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
		v.updateSupplyMetrics()
		return v, nil
	}

	vaultPath := cfg.ResolveVaultDir()
	if cfg.Vault.PATH == EMPTY_PATH {
		cfg.Vault.PATH = vaultPath
	}

	v.path = vaultPath

	dbDir := v.path
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		vltlogger().Errorw("Failed to create vault directory", "path", dbDir, "err", err)
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}

	if err := ensureVaultKeys(dbDir); err != nil {
		vltlogger().Errorw("Failed to load vault encryption keys", "err", err)
		return nil, fmt.Errorf("failed to load vault encryption keys: %w", err)
	}

	db, err := pogreb.Open(dbDir, nil)
	if err != nil {
		vltlogger().Errorw("Failed to open pogreb database", "path", dbDir, "err", err)
		return nil, fmt.Errorf("failed to open pogreb database: %w", err)
	}
	v.db = db

	key := rootSA.Address.Bytes()
	has, err := db.Has(key)
	if err != nil {
		vltlogger().Errorw("Failed to check if root account exists", "err", err)
		db.Close()
		return nil, fmt.Errorf("failed to check if root account exists: %w", err)
	}
	if !has {
		accountData := rootSA.Bytes()
		if err := putAccountPayload(db, key, accountData); err != nil {
			vltlogger().Errorw("Failed to write root account", "err", err)
			db.Close()
			return nil, fmt.Errorf("failed to write root account: %w", err)
		}
		v.accounts.Append(rootSA.Address, rootSA)
		vltlogger().Info("Created new vault with root account")
	} else {
		if err := v.SyncFromDB(); err != nil {
			vltlogger().Errorw("Failed to sync vault", "err", err)
			db.Close()
			return nil, fmt.Errorf("failed to sync vault: %w", err)
		}
		if v.accounts.GetAccount(rootSA.Address) == nil {
			v.accounts.Append(rootSA.Address, rootSA)
			if err := putAccountPayload(db, key, rootSA.Bytes()); err != nil {
				vltlogger().Errorw("Failed to save root account", "err", err)
				db.Close()
				return nil, fmt.Errorf("failed to save root account: %w", err)
			}
		}
		vltlogger().Info("Synced existing vault")
	}

	v.status = 0xa
	vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	v.updateSupplyMetrics()

	return v, nil
}

func (v *D5Vault) Prepare() {

}

func (v *D5Vault) Clear() error {
	return v.accounts.Clear()
}

func (v *D5Vault) Get(addr types.Address) *account.StateAccount {
	return v.accounts.GetAccount(addr)
}

// Get - get account by index
//
//	args: index:int64
//	return: account:types.StateAccount
func (v *D5Vault) GetPos(pos int64) *account.StateAccount {
	return v.accounts.GetByIndex(pos)
}

func (v *D5Vault) GetAll() interface{} {
	// Returns current in-memory account balances. Reload from disk via SyncVault / SyncFromDB when needed.
	return v.accounts.GetAll()
}

func (v *D5Vault) Put(address types.Address, acc *account.StateAccount) {
	wasNew := v.accounts.GetAccount(address) == nil
	v.accounts.Append(address, acc)
	if wasNew {
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	}
	v.markDirty(address)
}

// EnsureAccount ensures an address exists in the vault with balance 0 if not present.
// Used so that validator/node addresses appear in account list (e.g. cerera.account.getAll)
// even when they have never received a transaction.
func (v *D5Vault) EnsureAccount(addr types.Address) {
	if addr.IsEmpty() {
		return
	}
	if v.accounts.GetAccount(addr) != nil {
		return
	}
	acc := account.NewStateAccount(addr, 0, v.rootHash)
	acc.Status = 3 // OP_ACC_NODE — visible in getAll, excluded from consensus state root
	v.accounts.Append(addr, acc)
	vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	v.markDirty(addr)
}
func (v *D5Vault) Size() int64 {
	if v.inMem || v.db == nil {
		return int64(v.accounts.Size())
	}
	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	// Count account rows only (exclude code:/storage: keys).
	count := int64(0)
	it := v.db.Items()
	for {
		k, _, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			return -1
		}
		if isContractOrStorageKey(k) {
			continue
		}
		count++
	}
	return count
}

func (v *D5Vault) UpdateBalance(from types.Address, to types.Address, cnt *big.Int, txHash common.Hash) error {
	if cnt == nil || cnt.Sign() <= 0 {
		return ErrInvalidMintAmount
	}

	saFrom := v.Get(from)
	if saFrom == nil {
		return ErrAccountNotFound
	}

	saDest := v.Get(to)
	if saDest == nil {
		saDest = account.NewStateAccount(to, 0, v.rootHash)
		v.accounts.Append(to, saDest)
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	}

	newFromBal := new(big.Int).Sub(saFrom.GetBalanceBI(), cnt)
	saFrom.SetBalanceBI(newFromBal)

	newToBal := new(big.Int).Add(saDest.GetBalanceBI(), cnt)
	saDest.SetBalanceBI(newToBal)
	saDest.AddInput(txHash, cnt)

	v.markDirty(from)
	v.markDirty(to)
	vaultTransfersTotal.Inc()
	return nil
}

func (v *D5Vault) creditMintedAmount(to types.Address, cnt *big.Int, txHash common.Hash) (*account.StateAccount, error) {
	if cnt == nil || cnt.Sign() <= 0 {
		return nil, ErrInvalidMintAmount
	}
	if err := v.CheckSupplyLimit(cnt); err != nil {
		return nil, err
	}

	var saDest = v.Get(to)
	if saDest == nil {
		saDest = account.NewStateAccount(to, 0, v.rootHash)
		v.accounts.Append(to, saDest)
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	}

	newBal := new(big.Int).Add(saDest.GetBalanceBI(), cnt)
	saDest.SetBalanceBI(newBal)
	saDest.AddInput(txHash, cnt)
	v.markDirty(to)

	v.updateSupplyMetrics()
	return saDest, nil
}

// RewardMiner mints new coins for the miner (coinbase transaction execution).
func (v *D5Vault) RewardMiner(to types.Address, cnt *big.Int, txHash common.Hash) error {
	acc, err := v.creditMintedAmount(to, cnt, txHash)
	if err != nil {
		return err
	}
	// Mining rewards must affect the consensus state root on every node.
	// Initiator accounts start as OP_ACC_NODE (excluded) but coinbase pays head.Node.
	if acc != nil && acc.Status == 3 {
		acc.Status = 0
		v.markDirty(to)
	}
	return nil
}

func (v *D5Vault) CheckRunnable(r *big.Int, s *big.Int, tx *types.GTransaction) bool {
	// ecdsa.Verify(publicKey, tx.Hash().Bytes(), r, s)
	return false
}

func (v *D5Vault) GetCount() int {
	return v.accounts.GetCount()
}

// ExportAccountRangeSortedByAddress returns serialized accounts for a stable
// lexicographic slice of addresses (offset/limit over sorted hex keys).
// TODO this is CRITICAL point of optimization for the future, we need to use a more efficient data structure for this.
func (v *D5Vault) ExportAccountRangeSortedByAddress(offset, limit int) ([][]byte, int) {
	accounts, total := v.accounts.ExportRange(offset, limit)
	out := make([][]byte, 0, len(accounts))
	for _, sa := range accounts {
		if sa != nil {
			out = append(out, sa.Bytes())
		}
	}
	return out, total
}

// SaveBaseline stores the current account map for chain replay after reorg.
func (v *D5Vault) SaveBaseline() {
	if v == nil {
		return
	}
	v.baselineSnapshot = v.SnapshotAccounts()
}

// RestoreBaseline resets accounts to the saved baseline (pre-chain-replay state).
func (v *D5Vault) RestoreBaseline() {
	if v == nil || v.baselineSnapshot == nil {
		return
	}
	v.RestoreAccounts(v.baselineSnapshot)
}

// ComputeStateRoot returns the consensus state root (excludes OP_ACC_NODE accounts).
func (v *D5Vault) ComputeStateRoot() common.Hash {
	leaves := v.consensusAccountLeaves()
	if len(leaves) == 0 {
		return common.EmptyRootHash
	}
	var concat []byte
	for _, leaf := range leaves {
		concat = append(concat, leaf...)
	}
	sum := blake2b.Sum256(concat)
	return common.BytesToHash(sum[:])
}

func (v *D5Vault) consensusAccountLeaves() [][]byte {
	hexes := v.accounts.sortedAddresses()
	v.accounts.mu.RLock()
	defer v.accounts.mu.RUnlock()

	out := make([][]byte, 0, len(hexes))
	for _, h := range hexes {
		addr := types.HexToAddress(h)
		acc := v.accounts.accounts[addr]
		if acc == nil || acc.Status == 3 {
			continue
		}
		out = append(out, consensusAccountLeaf(acc))
	}
	return out
}

func consensusAccountLeaf(acc *account.StateAccount) []byte {
	var buf bytes.Buffer
	buf.Write(acc.Address.Bytes())
	bal := acc.GetBalanceBI().Bytes()
	padded := make([]byte, 32)
	if len(bal) > 32 {
		copy(padded, bal[len(bal)-32:])
	} else {
		copy(padded[32-len(bal):], bal)
	}
	buf.Write(padded)
	_ = binary.Write(&buf, binary.LittleEndian, acc.Nonce)
	sum := blake2b.Sum256(buf.Bytes())
	return sum[:]
}

func (v *D5Vault) GetOwner() *account.StateAccount {
	return v.initiator
}

func (v *D5Vault) Sync(saBytes []byte) {
	sa := types.BytesToStateAccount(saBytes)
	if sa == nil {
		return
	}
	v.accounts.Append(sa.Address, sa)
	vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	if err := v.persistAccount(sa.Address, sa); err != nil {
		vltlogger().Warnw("Sync: failed to persist account to vault db",
			"address", sa.Address.Hex(),
			"err", err,
		)
	}
}

func (v *D5Vault) Status() byte {
	return v.status
}

func (v *D5Vault) ServiceName() string {
	return VAULT_SERVICE_NAME
}

// SyncFromDB loads all accounts from the pogreb database into memory // LOAD
func (v *D5Vault) SyncFromDB() error {
	if v.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if v.accounts == nil {
		v.accounts = NewAccountIndex()
	}

	v.accounts.Clear()

	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
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

		if isContractOrStorageKey(key) {
			continue
		}

		// Try to deserialize account, skip on error
		// TODO ERROR IN THIS GOROUTINE WHEN DB USAGE
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
			plain, derr := decodeAccountPayload(accountData)
			if derr != nil {
				vltlogger().Warnw("syncFromDB: decode account payload", "err", derr, "key", fmt.Sprintf("%x", key))
				return
			}
			if !account.ValidSerialized(plain) {
				vltlogger().Errorw("Failed to decrypt or deserialize account (vault encryption key mismatch?)",
					"key", fmt.Sprintf("%x", key),
					"storedLen", len(accountData),
					"plainLen", len(plain),
					"hint", "set VAULT_MNEMONIC from first-run log or ensure .vault_keys matches the DB",
				)
				return
			}
			acc := types.BytesToStateAccount(plain)
			if acc != nil {
				vltlogger().Infow("Read account from pogreb vault", "address", acc.Address.Hex())
				v.accounts.Append(acc.Address, acc)
			} else {
				previewLen := 20
				if len(plain) < previewLen {
					previewLen = len(plain)
				}
				vltlogger().Errorw("Failed to deserialize account",
					"key", fmt.Sprintf("%x", key),
					"storedLen", len(accountData),
					"plainLen", len(plain),
					"plainPreview", fmt.Sprintf("%x", plain[:previewLen]),
				)
			}
		}()
	}

	return nil
}

// Close flushes dirty accounts and closes the pogreb database.
func (v *D5Vault) Close() error {
	if err := v.Flush(); err != nil {
		return err
	}

	v.dbMu.Lock()
	defer v.dbMu.Unlock()

	if v.db == nil {
		vltlogger().Infow("Close(): database is already closed or not initialized")
		return nil
	}

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
	acc := v.Get(address)
	if acc == nil {
		// Создаем новый аккаунт для контракта
		acc = account.NewStateAccount(address, 0, v.rootHash)
		// account.CodeHash = codeHash
		// Устанавливаем тип как контракт (Type = 5 для контракта, если нужно)
		// Пока оставляем Type = 0, но CodeHash будет указывать на наличие кода
		v.accounts.Append(address, acc)
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	} else {
		// Обновляем CodeHash существующего аккаунта
		// account.CodeHash = codeHash
		v.accounts.Append(address, acc)
	}

	v.contractMu.Lock()
	codeCopy := append([]byte(nil), code...)
	v.contractCode[address] = codeCopy
	v.contractMu.Unlock()

	if !v.inMem && v.db != nil {
		v.dbMu.Lock()
		key := append([]byte("code:"), address.Bytes()...)
		if err := v.db.Put(key, code); err != nil {
			v.dbMu.Unlock()
			vltlogger().Errorw("Failed to store contract code", "address", address.Hex(), "err", err)
			return fmt.Errorf("failed to store contract code: %w", err)
		}
		if err := putAccountPayload(v.db, address.Bytes(), acc.Bytes()); err != nil {
			v.dbMu.Unlock()
			vltlogger().Errorw("Failed to update account with code hash", "address", address.Hex(), "err", err)
			return fmt.Errorf("failed to update account: %w", err)
		}
		v.dbMu.Unlock()
		vltlogger().Infow("Stored contract code", "address", address.Hex(), "codeSize", len(code), "codeHash", fmt.Sprintf("%x", codeHash))
	}

	v.markDirty(address)
	return nil
}

// GetContractCode returns contract bytecode from storage.
func (v *D5Vault) GetContractCode(address types.Address) ([]byte, error) {
	v.contractMu.RLock()
	if code, ok := v.contractCode[address]; ok {
		v.contractMu.RUnlock()
		return append([]byte(nil), code...), nil
	}
	v.contractMu.RUnlock()

	if v.inMem {
		return nil, fmt.Errorf("contract code not found for address %s", address.Hex())
	}
	if v.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	key := append([]byte("code:"), address.Bytes()...)
	has, err := v.db.Has(key)
	if err != nil {
		return nil, fmt.Errorf("failed to check contract code: %w", err)
	}
	if !has {
		return nil, fmt.Errorf("contract code not found for address %s", address.Hex())
	}
	code, err := v.db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract code: %w", err)
	}
	return code, nil
}

// HasContractCode reports whether contract bytecode exists for the address.
func (v *D5Vault) HasContractCode(address types.Address) bool {
	v.contractMu.RLock()
	if _, ok := v.contractCode[address]; ok {
		v.contractMu.RUnlock()
		return true
	}
	v.contractMu.RUnlock()

	if v.inMem || v.db == nil {
		return false
	}
	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	key := append([]byte("code:"), address.Bytes()...)
	has, err := v.db.Has(key)
	return err == nil && has
}

// DeleteContractCode removes contract bytecode.
func (v *D5Vault) DeleteContractCode(address types.Address) error {
	v.contractMu.Lock()
	delete(v.contractCode, address)
	delete(v.contractStorage, address)
	v.contractMu.Unlock()

	if v.inMem {
		return nil
	}
	if v.db == nil {
		return fmt.Errorf("database not initialized")
	}

	v.dbMu.Lock()
	defer v.dbMu.Unlock()
	key := append([]byte("code:"), address.Bytes()...)
	if err := v.db.Delete(key); err != nil {
		return fmt.Errorf("failed to delete contract code: %w", err)
	}
	if acc := v.Get(address); acc != nil {
		if err := putAccountPayload(v.db, address.Bytes(), acc.Bytes()); err != nil {
			return fmt.Errorf("failed to update account: %w", err)
		}
	}
	vltlogger().Infow("Deleted contract code", "address", address.Hex())
	return nil
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
	acc := v.Get(address)
	if acc == nil {
		// Создаем новый аккаунт для контракта
		acc = account.NewStateAccount(address, 0, v.rootHash)
		v.accounts.Append(address, acc)
		vaultAccountsTotal.Set(float64(v.accounts.GetCount()))
	}

	keyStr := contractStorageKey(key)
	v.contractMu.Lock()
	if v.contractStorage[address] == nil {
		v.contractStorage[address] = make(map[string]*big.Int)
	}
	v.contractStorage[address][keyStr] = new(big.Int).Set(value)
	v.contractMu.Unlock()

	if !v.inMem && v.db != nil {
		v.dbMu.Lock()
		keyBytes := make([]byte, 32)
		key.FillBytes(keyBytes)
		storageKey := append([]byte("storage:"), address.Bytes()...)
		storageKey = append(storageKey, ':')
		storageKey = append(storageKey, keyBytes...)
		valueBytes := make([]byte, 32)
		value.FillBytes(valueBytes)
		if err := v.db.Put(storageKey, valueBytes); err != nil {
			v.dbMu.Unlock()
			return fmt.Errorf("failed to set storage: %w", err)
		}
		v.dbMu.Unlock()
	}
	return nil
}

// GetStorage returns a contract storage slot.
func (v *D5Vault) GetStorage(address types.Address, key *big.Int) (*big.Int, error) {
	if key == nil {
		return nil, fmt.Errorf("storage key cannot be nil")
	}

	keyStr := contractStorageKey(key)
	v.contractMu.RLock()
	if slots, ok := v.contractStorage[address]; ok {
		if val, ok := slots[keyStr]; ok {
			v.contractMu.RUnlock()
			return new(big.Int).Set(val), nil
		}
	}
	v.contractMu.RUnlock()

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

func (v *D5Vault) Exec(method string, params []any) any {
	switch method {
	case "getAll":
		return v.GetAll()
	case "getCount":
		return v.GetCount()
	case "create":
		if len(params) < 1 {
			return fmt.Errorf("%w: passphrase required", ErrErrorParsingParameters)
		}
		passphraseStr, ok1 := params[0].(string)
		if !ok1 {
			return fmt.Errorf("%w: passphrase must be a string", ErrErrorParsingParameters)
		}
		mk, pk, m, addr, err := v.Create(passphraseStr)
		if err != nil {
			return err
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
		addr, pk, err := v.Restore(mnemonic, pass)
		if err != nil {
			return err.Error()
		}
		type res struct {
			Addr types.Address `json:"address,omitempty"`
			Priv string        `json:"priv,omitempty"`
			Pub  string        `json:"pub,omitempty"`
		}
		Result := &res{
			Priv: pk,
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
			dust, err := types.DecimalStringToDust(s)
			if err != nil {
				return err.Error()
			}
			amountBigInt = dust
		} else if f, ok := params[1].(float64); ok {
			amountBigInt = common.FloatToBigInt(f)
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

func (v *D5Vault) Methods() map[string]service.RPCHandler {
	return map[string]service.RPCHandler{
		"getAll": func(ctx context.Context, params []any) (any, error) {
			return v.GetAll(), nil
		},
		"getCount": func(ctx context.Context, params []any) (any, error) {
			return v.GetCount(), nil
		},
		"create": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("passphrase required")
			}
			passphrase, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("passphrase must be string")
			}
			mk, pk, m, addr, err := v.Create(passphrase)
			if err != nil {
				return nil, err
			}
			type res struct {
				Address  *types.Address `json:"address,omitempty"`
				Priv     string         `json:"priv,omitempty"`
				Pub      string         `json:"pub,omitempty"`
				Mnemonic string         `json:"mnemonic,omitempty"`
			}
			return &res{
				Address:  addr,
				Priv:     mk,
				Pub:      pk,
				Mnemonic: m,
			}, nil
		},
		"restore": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("mnemonic and passphrase required")
			}
			mnemonic, ok0 := params[0].(string)
			pass, ok1 := params[1].(string)
			if !ok0 || !ok1 {
				return nil, fmt.Errorf("mnemonic and passphrase must be strings")
			}
			addr, pk, err := v.Restore(mnemonic, pass)
			if err != nil {
				return nil, err
			}
			type res struct {
				Addr types.Address `json:"address,omitempty"`
				Priv string        `json:"priv,omitempty"`
				Pub  string        `json:"pub,omitempty"`
			}
			return &res{
				Priv: pk,
				Addr: addr,
			}, nil
		},
		"verify": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("address and passphrase required")
			}
			addrHex, ok0 := params[0].(string)
			pass, ok1 := params[1].(string)
			if !ok0 || !ok1 {
				return nil, fmt.Errorf("address and passphrase must be strings")
			}
			rAddr, err := v.VerifyAccount(types.HexToAddress(addrHex), pass)
			if err != nil {
				return false, nil
			}
			return rAddr.Hex() == addrHex, nil
		},
		"getBalance": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("address required")
			}
			addrHex, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("address must be string")
			}
			acc := v.Get(types.HexToAddress(addrHex))
			if acc == nil {
				return big.NewInt(0), nil
			}
			return acc.GetBalance(), nil
		},
		"faucet": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("address and amount required")
			}
			addrStr, ok0 := params[0].(string)
			if !ok0 {
				return nil, fmt.Errorf("address must be string")
			}
			addr := types.HexToAddress(addrStr)
			var amountBigInt *big.Int
			if s, ok := params[1].(string); ok {
				dust, err := types.DecimalStringToDust(s)
				if err != nil {
					return nil, err
				}
				amountBigInt = dust
			} else if f, ok := params[1].(float64); ok {
				amountBigInt = common.FloatToBigInt(f)
			} else {
				return nil, fmt.Errorf("amount must be string or number")
			}
			txHash := common.BytesToHash([]byte("faucet_" + addrStr))
			err := v.DropFaucet(addr, amountBigInt, txHash)
			if err != nil {
				return nil, err
			}
			return "Faucet successful", nil
		},
		"inputs": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("address required")
			}
			addrHex, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("address must be string")
			}
			account := v.Get(types.HexToAddress(addrHex))
			if account == nil {
				return map[common.Hash]*big.Int{}, nil
			}
			return account.GetAllInputs(), nil
		},
		"getTotalSupply": func(ctx context.Context, params []any) (any, error) {
			return v.GetTotalSupply(), nil
		},
	}
}
