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
var Faucet coinbaseData

var AddressHex = "0xf0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f"
var FaucetAddressHex = "0xf0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"
var TotalValue = types.FloatToBigInt(699999000000.0)
var FaucetInitialBalance = types.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = types.FloatToBigInt(1024.0)
var InitialNodeBalance = 0.0000

func CurrentReward() int {
	return 1024
}

// SetCoinbase initializes the global Coinbase data.
func InitOperationData() {
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
		Inputs:   []common.Hash{},
	}
	Coinbase = coinbaseData{
		coinbaseAccount: ca,
		address:         addr,
		balance:         ca.Balance,
	}

	fc := types.StateAccount{
		Address:  addr,
		Balance:  FaucetInitialBalance,
		Bloom:    []byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CodeHash: []byte{},
		Name:     "COINBASE",
		Nonce:    1,
		Root:     common.HexToHash(AddressHex),
		Status:   "OP_ACC_C",
		Inputs:   []common.Hash{},
	}
	Faucet = coinbaseData{
		coinbaseAccount: fc,
		address:         faucetAddr,
		balance:         big.NewInt(0),
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

func DropFaucet(faucetValue int) *big.Int {
	var faucetVal_BigInt = types.FloatToBigInt(float64(faucetValue))
	Coinbase.balance = Coinbase.balance.Sub(Coinbase.balance, faucetVal_BigInt)
	return faucetVal_BigInt
}

func CreateCoinBaseTransation(nonce uint64, addr types.Address) types.GTransaction {
	return *types.NewCoinBaseTransaction(nonce, addr, blockReward, 100, big.NewInt(100), []byte("OP_REW"))
}

func FaucetAccount() types.StateAccount {
	return Faucet.coinbaseAccount
}

func GetFaucetAddress() types.Address {
	return Faucet.address
}

func FaucetTransaction(nonce uint64, destAddr types.Address, cnt float64) *types.GTransaction {
	var tx = types.NewTransaction(
		nonce,
		destAddr,
		types.FloatToBigInt(cnt),
		10000,
		types.FloatToBigInt(0.343),
		[]byte("FAUCET_REQ_TX"),
	)
	return tx
}
