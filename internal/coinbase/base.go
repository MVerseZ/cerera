package coinbase

import (
	"math/big"

	"github.com/cerera/core/params"
	"github.com/cerera/core/types"
)

// BlockReward returns the currently configured block reward for the given height.
func BlockReward(height uint64) *big.Int {
	return params.BlockReward(height)
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
