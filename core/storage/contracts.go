package storage

import "math/big"

func contractStorageKey(key *big.Int) string {
	keyBytes := make([]byte, 32)
	key.FillBytes(keyBytes)
	return string(keyBytes)
}
