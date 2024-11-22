package coinbase

import (
	"math/big"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

type coinbaseData struct {
	address         types.Address
	coinbaseAccount types.StateAccount
	balance         *big.Int
}

// Create a global instance of coinbaseData
var Coinbase coinbaseData

var AddressHex = "0xf0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f"
var TotalValue = types.FloatToBigInt(21000000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = types.FloatToBigInt(1024.0)

func CurrentReward() int {
	return 1024
}

// SetCoinbase initializes the global Coinbase data.
func SetCoinbase() {
	var addr = types.HexToAddress(AddressHex)
	ca := types.StateAccount{
		Address:  addr,
		Balance:  TotalValue,
		Bloom:    []byte("COINBASE_ACC"),
		CodeHash: []byte{},
		Name:     "COINBASE",
		Nonce:    1,
		Root:     common.HexToHash(AddressHex),
		Status:   "OP_ACC_C",
		Inputs:   []common.Hash{},
	}
	Coinbase = coinbaseData{
		coinbaseAccount: ca,
		address:         addr,
		balance:         ca.Balance,
	}
}

// GetCoinbaseAddress returns the global Coinbase address.
func GetCoinbaseAddress() types.Address {
	return Coinbase.address
}

// GetCoinbaseBalance returns the global Coinbase balance.
func GetCoinbaseBalance() *big.Int {
	return Coinbase.balance
}

func CoinBaseStateAccount() types.StateAccount {
	return Coinbase.coinbaseAccount
}

func RewardBlock() *big.Int {
	Coinbase.balance = big.NewInt(0).Sub(Coinbase.balance, blockReward)
	return blockReward
}

func Faucet(faucetValue int) *big.Int {
	var faucetVal_BigInt = types.FloatToBigInt(float64(faucetValue))
	Coinbase.balance = big.NewInt(0).Sub(Coinbase.balance, faucetVal_BigInt)
	return faucetVal_BigInt
}
