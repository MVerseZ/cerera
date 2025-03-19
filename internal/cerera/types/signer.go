package types

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/cerera/internal/cerera/common"
)

type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *GTransaction) (Address, error)

	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignTransaction(tx *GTransaction, k *ecdsa.PrivateKey) (common.Hash, error)
	SignatureValues(tx *GTransaction, sig []byte) (r, s, v *big.Int, err error)
	ChainID() *big.Int

	Hash(tx *GTransaction) common.Hash

	// // Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool

	// Pen() *ecdsa.PrivateKey
}
type GDP77Signer struct {
}
