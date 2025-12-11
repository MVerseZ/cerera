package coinbase

import (
	"math/big"

	"github.com/cerera/internal/cerera/types"
)

// economy structs
var TotalValue = types.FloatToBigInt(700000000000.0)
var FaucetInitialBalance = types.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = types.FloatToBigInt(1.0)
var InitialNodeBalance = 0.0000
var FaucetValue = types.FloatToBigInt(100.0000)

// Faucet limits and constraints
var MaxFaucetAmount = types.FloatToBigInt(1000.0) // Maximum amount per request
var MinFaucetAmount = types.FloatToBigInt(1.0)    // Minimum amount per request
var FaucetCooldownHours = 0.01
