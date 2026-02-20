package coinbase

import (
	"math/big"

	"github.com/cerera/core/types"
)

// BlockReward returns the currently configured block reward.
func BlockReward() *big.Int {
	return new(big.Int).Set(blockReward)
}

// CreateCoinBaseTransation constructs a coinbase transaction that mints brand
// new coins directly to the provided miner address.
func CreateCoinBaseTransation(nonce uint64, addr types.Address) types.GTransaction {
	tx := types.NewCoinBaseTransaction(
		nonce,
		addr,
		BlockReward(),
		0,
		big.NewInt(0),
		[]byte("OP_REW"),
	)
	return *tx
}
