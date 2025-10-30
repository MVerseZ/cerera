package coinbase

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

type coinbaseData struct {
	address         types.Address
	coinbaseAccount types.StateAccount
	balance         *big.Int
	mu              sync.RWMutex // Mutex for thread-safe operations
}

// Create a global instance of coinbaseData
var Coinbase coinbaseData
var Faucet coinbaseData

// Faucet request tracking
type FaucetRequest struct {
	Address   types.Address
	Amount    *big.Int
	Timestamp time.Time
}

type FaucetTracker struct {
	requests map[types.Address]time.Time
	mu       sync.RWMutex
}

var faucetTracker = FaucetTracker{
	requests: make(map[types.Address]time.Time),
}

// Instance of sharing base data

const AddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000f"
const FaucetAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000a"

var TotalValue = types.FloatToBigInt(699999000000.0)
var FaucetInitialBalance = types.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = types.FloatToBigInt(1.0)
var InitialNodeBalance = 0.0000
var FaucetValue = types.FloatToBigInt(100.0000)

// Faucet limits and constraints
var MaxFaucetAmount = types.FloatToBigInt(1000.0) // Maximum amount per request
var MinFaucetAmount = types.FloatToBigInt(1.0)    // Minimum amount per request
var FaucetCooldownHours = 1                       // Hours between requests per address

func CurrentReward() int {
	return 1024
}

// SetCoinbase initializes the global Coinbase data.
func InitOperationData() error {
	var addr = types.HexToAddress(AddressHex)
	var faucetAddr = types.HexToAddress(FaucetAddressHex)

	ca := types.StateAccount{
		Address: addr,
		// Balance:  TotalValue,
		Bloom:    []byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CodeHash: []byte{},
		Nonce:    1,
		Root:     common.HexToHash(AddressHex),
		Status:   "OP_ACC_C",
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	ca.SetBalance(types.BigIntToFloat(TotalValue))
	Coinbase = coinbaseData{
		coinbaseAccount: ca,
		address:         addr,
		balance:         types.FloatToBigInt(ca.GetBalance()),
	}

	fc := types.StateAccount{
		Address: faucetAddr,
		// Balance:  FaucetInitialBalance,
		Bloom:    []byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CodeHash: []byte{},
		Nonce:    1,
		Root:     common.HexToHash(FaucetAddressHex),
		Status:   "OP_ACC_F",
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	fc.SetBalance(types.BigIntToFloat(FaucetInitialBalance))
	Faucet = coinbaseData{
		coinbaseAccount: fc,
		address:         faucetAddr,
		balance:         FaucetInitialBalance,
	}
	return nil
}

// GetCoinbaseAddress returns the global Coinbase address.
func GetCoinbaseAddress() types.Address {
	return Coinbase.address
}

// GetCoinbaseBalance returns the global Coinbase balance.
func GetCoinbaseBalance() *big.Int {
	Coinbase.mu.RLock()
	defer Coinbase.mu.RUnlock()
	return new(big.Int).Set(Coinbase.balance) // Return a copy to prevent external modification
}

func CoinBaseStateAccount() types.StateAccount {
	Coinbase.mu.RLock()
	defer Coinbase.mu.RUnlock()
	return Coinbase.coinbaseAccount
}

func RewardBlock() *big.Int {
	Coinbase.mu.Lock()
	defer Coinbase.mu.Unlock()
	Coinbase.balance = big.NewInt(0).Sub(Coinbase.balance, blockReward)
	Coinbase.coinbaseAccount.SetBalance(types.BigIntToFloat(Coinbase.balance))
	return new(big.Int).Set(blockReward) // Return a copy
}

func DropFaucet(faucetValue *big.Int) *big.Int {
	Coinbase.mu.Lock()
	defer Coinbase.mu.Unlock()
	Coinbase.balance = Coinbase.balance.Sub(Coinbase.balance, faucetValue)
	Coinbase.coinbaseAccount.SetBalance(types.BigIntToFloat(Coinbase.balance))
	return new(big.Int).Set(faucetValue) // Return a copy
}

func CreateCoinBaseTransation(nonce uint64, addr types.Address) types.GTransaction {
	tx := types.NewCoinBaseTransaction(nonce, addr, blockReward, 0, big.NewInt(100), []byte("OP_REW"))
	// Set the from address to coinbase address for proper execution
	tx.SetFrom(GetCoinbaseAddress())
	return *tx
}

func FaucetAccount() types.StateAccount {
	Faucet.mu.RLock()
	defer Faucet.mu.RUnlock()
	return Faucet.coinbaseAccount
}

func GetFaucetAddress() types.Address {
	return Faucet.address
}

// GetFaucetBalance returns the current faucet balance safely
func GetFaucetBalance() *big.Int {
	Faucet.mu.RLock()
	defer Faucet.mu.RUnlock()
	return new(big.Int).Set(Faucet.balance) // Return a copy to prevent external modification
}

func FaucetTransaction(nonce uint64, destAddr types.Address) *types.GTransaction {
	// Do not mutate faucet balance here; actual state changes must happen on tx application
	return types.NewFaucetTransaction(
		nonce,
		destAddr,
		FaucetValue,
	)
}

// CheckFaucetLimits validates faucet request against limits
func CheckFaucetLimits(address types.Address, amount *big.Int) error {
	if amount == nil || amount.Sign() <= 0 {
		return errors.New("invalid faucet amount")
	}

	// Check minimum amount
	if amount.Cmp(MinFaucetAmount) < 0 {
		return errors.New("faucet amount below minimum")
	}

	// Check maximum amount
	if amount.Cmp(MaxFaucetAmount) > 0 {
		return errors.New("faucet amount exceeds maximum")
	}

	// Check cooldown period
	faucetTracker.mu.RLock()
	lastRequest, exists := faucetTracker.requests[address]
	faucetTracker.mu.RUnlock()

	if exists {
		timeSinceLastRequest := time.Since(lastRequest)
		if timeSinceLastRequest < time.Duration(FaucetCooldownHours)*time.Hour {
			remainingTime := time.Duration(FaucetCooldownHours)*time.Hour - timeSinceLastRequest
			return fmt.Errorf("faucet cooldown active, try again in %v", remainingTime.Round(time.Minute))
		}
	}

	return nil
}

// RecordFaucetRequest records a faucet request for tracking
func RecordFaucetRequest(address types.Address, amount *big.Int) {
	faucetTracker.mu.Lock()
	defer faucetTracker.mu.Unlock()
	faucetTracker.requests[address] = time.Now()
}

// GetFaucetCooldownRemaining returns remaining cooldown time for an address
func GetFaucetCooldownRemaining(address types.Address) time.Duration {
	faucetTracker.mu.RLock()
	defer faucetTracker.mu.RUnlock()

	lastRequest, exists := faucetTracker.requests[address]
	if !exists {
		return 0
	}

	timeSinceLastRequest := time.Since(lastRequest)
	cooldownDuration := time.Duration(FaucetCooldownHours) * time.Hour

	if timeSinceLastRequest >= cooldownDuration {
		return 0
	}

	return cooldownDuration - timeSinceLastRequest
}
