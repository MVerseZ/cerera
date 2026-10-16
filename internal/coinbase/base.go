package coinbase

import (
	"math/big"

	"github.com/cerera/core/types"
)

// BlockReward returns the currently configured block reward for the given height.
// Reward starts at InitialReward and halves every HalvingInterval blocks.
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

// CreateCoinBaseTransation constructs a coinbase transaction that mints brand
// new coins directly to the provided miner address.
func CreateCoinBaseTransation(nonce uint64, addr types.Address, height uint64) types.GTransaction {
	tx := types.NewCoinBaseTransaction(
		nonce,
		addr,
		BlockReward(height),
		0,
		big.NewInt(0),
		[]byte("OP_REW"),
	)
	return *tx
}
