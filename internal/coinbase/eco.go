package coinbase

import (
	"math/big"

	"github.com/cerera/core/common"
)

// economy structs
var TotalValue = common.FloatToBigInt(700000000000.0)
var FaucetInitialBalance = common.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var blockReward = common.FloatToBigInt(1.0)
var InitialNodeBalance = 0.0000
var FaucetValue = common.FloatToBigInt(100.0000)

// Faucet limits and constraints
var MaxFaucetAmount = common.FloatToBigInt(1000.0) // Maximum amount per request
var MinFaucetAmount = common.FloatToBigInt(1.0)    // Minimum amount per request
var FaucetCooldownHours = 0.01
