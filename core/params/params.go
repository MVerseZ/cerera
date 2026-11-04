package params

import (
	"math/big"

	"github.com/cerera/core/common"
)

var TotalValue = common.FloatToBigInt(700000000000.0)
var FaucetInitialBalance = common.FloatToBigInt(1000000.0)
var QuarterValue = big.NewInt(0).Div(TotalValue, big.NewInt(4))
var InitialReward = common.FloatToBigInt(10000.0)
var HalvingInterval = uint64(2100)
var InitialNodeBalance = 0.0000
var FaucetValue = common.FloatToBigInt(100.0000)

var MaxFaucetAmount = common.FloatToBigInt(1000.0)
var MinFaucetAmount = common.FloatToBigInt(1.0)
var FaucetCooldownHours = 0.01

// BlockReward returns the block reward for height (halving every HalvingInterval blocks).
func BlockReward(height uint64) *big.Int {
	halvings := height / HalvingInterval
	reward := new(big.Int).Set(InitialReward)
	for i := uint64(0); i < halvings; i++ {
		reward = new(big.Int).Div(reward, big.NewInt(2))
	}
	if reward.Sign() < 0 {
		return big.NewInt(0)
	}
	return reward
}
