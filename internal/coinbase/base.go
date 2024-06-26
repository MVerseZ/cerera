package coinbase

import (
	"math/big"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

type coinbaseData struct {
	address         types.Address
	coinbaseAccount types.StateAccount
	balance         big.Int
	publicKey       string
	privateKey      string
}

// Create a global instance of coinbaseData
var Coinbase coinbaseData

var AddressHex = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"
var TotalValue = types.FloatToBigInt(37 * 10 << 37)

// SetCoinbase initializes or updates the global Coinbase data.
func SetCoinbase(publicKey, privateKey string, balance big.Int) {
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
		balance:         balance,
		publicKey:       publicKey,
		privateKey:      privateKey,
	}
}

// GetCoinbaseAddress returns the global Coinbase address.
func GetCoinbaseAddress() types.Address {
	return Coinbase.address
}

// GetCoinbaseBalance returns the global Coinbase balance.
func GetCoinbaseBalance() *big.Int {
	return &Coinbase.balance
}

// GetCoinbasePublicKey returns the global Coinbase public key.
func GetCoinbasePublicKey() string {
	return Coinbase.publicKey
}

// GetCoinbasePrivateKey returns the global Coinbase private key.
func GetCoinbasePrivateKey() string {
	return Coinbase.privateKey
}

func CoinBaseStateAccount() types.StateAccount {
	return Coinbase.coinbaseAccount
}
