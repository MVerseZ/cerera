package coinbase

import (
	"math/big"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

const (
	AddressHex       = "0x0000000000000000000000000000000000000000"
	FaucetAddressHex = "0x0000000000000000000000000000000000000001"
)

var (
	TotalValue           = big.NewInt(1000000000000000000) // 1 billion CER
	FaucetInitialBalance = big.NewInt(100000000000000000)  // 100 million CER
	blockReward          = big.NewInt(10000000000000000)   // 10 million CER
)

type coinbaseData struct {
	coinbaseAccount *types.StateAccount
	address         types.Address
	balance         *big.Int
}

var (
	Coinbase coinbaseData
	Faucet   coinbaseData
)

func InitOperationData() error {
	var addr = types.HexToAddress(AddressHex)
	var faucetAddr = types.HexToAddress(FaucetAddressHex)

	ca := &types.StateAccount{
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

	fc := &types.StateAccount{
		Address:  addr,
		Balance:  FaucetInitialBalance,
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

	Faucet = coinbaseData{
		coinbaseAccount: fc,
		address:         faucetAddr,
		balance:         big.NewInt(0),
	}
	return nil
}

// GetCoinbaseAddress returns the global Coinbase address.
func GetCoinbaseAddress() types.Address {
	return Coinbase.address
}

// GetCoinbaseBalance returns the global Coinbase balance.
func GetCoinbaseBalance() *big.Int {
	return Coinbase.balance
}

func CoinBaseStateAccount() *types.StateAccount {
	return Coinbase.coinbaseAccount
}

func RewardBlock() *big.Int {
	Coinbase.balance = big.NewInt(0).Sub(Coinbase.balance, blockReward)
	return blockReward
}

func DropFaucet(faucetValue *big.Int) *big.Int {
	Coinbase.balance = Coinbase.balance.Sub(Coinbase.balance, faucetValue)
	return faucetValue
}

func CreateCoinBaseTransation(nonce uint64, addr types.Address) types.GTransaction {
	return *types.NewCoinBaseTransaction(nonce, addr, blockReward, 100, big.NewInt(100), []byte("OP_REW"))
}

func FaucetAccount() *types.StateAccount {
	return Faucet.coinbaseAccount
}
