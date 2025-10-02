package coinbase

import (
	"math/big"
	"sync"

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

var AddressHex = "0xf0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f"
var FaucetAddressHex = "0xf0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"
var TotalValue = types.FloatToBigInt(699999000000.0)
var FaucetInitialBalance = types.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = types.FloatToBigInt(1024000.0)
var InitialNodeBalance = 0.0000

func CurrentReward() int {
	return 1024
}

// SetCoinbase initializes the global Coinbase data.
func InitOperationData() error {
	var addr = types.HexToAddress(AddressHex)
	var faucetAddr = types.HexToAddress(FaucetAddressHex)

	ca := types.StateAccount{
		Address:  addr,
		Balance:  TotalValue,
		Bloom:    []byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CodeHash: []byte{},
		Name:     "COINBASE",
		Nonce:    1,
		Root:     common.HexToHash(AddressHex),
		Status:   "OP_ACC_C",
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
	Coinbase = coinbaseData{
		coinbaseAccount: ca,
		address:         addr,
		balance:         ca.Balance,
	}

	fc := types.StateAccount{
		Address:  faucetAddr,
		Balance:  FaucetInitialBalance,
		Bloom:    []byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CodeHash: []byte{},
		Name:     "FAUCET",
		Nonce:    1,
		Root:     common.HexToHash(FaucetAddressHex),
		Status:   "OP_ACC_F",
		Inputs: &types.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}

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
	Coinbase.coinbaseAccount.Balance = Coinbase.balance
	return new(big.Int).Set(blockReward) // Return a copy
}

func DropFaucet(faucetValue *big.Int) *big.Int {
	Coinbase.mu.Lock()
	defer Coinbase.mu.Unlock()
	Coinbase.balance = Coinbase.balance.Sub(Coinbase.balance, faucetValue)
	Coinbase.coinbaseAccount.Balance = Coinbase.balance
	return new(big.Int).Set(faucetValue) // Return a copy
}

func CreateCoinBaseTransation(nonce uint64, addr types.Address) types.GTransaction {
	return *types.NewCoinBaseTransaction(nonce, addr, blockReward, 100, big.NewInt(100), []byte("OP_REW"))
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
func FaucetTransaction(nonce uint64, destAddr types.Address, cnt float64) *types.GTransaction {
	// Do not mutate faucet balance here; actual state changes must happen on tx application
	return types.NewFaucetTransaction(
		nonce,
		destAddr,
		types.FloatToBigInt(cnt),
	)
}
