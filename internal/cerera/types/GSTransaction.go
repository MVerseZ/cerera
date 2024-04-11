package types

import (
	"math/big"
	"time"
)

type GSTransaction struct {
	Nonce     uint64
	To        *Address
	Gas       uint64
	GasPrice  *big.Int
	Value     *big.Int
	Data      []byte
	GasTipCap *big.Int // a.k.a. maxPriorityFeePerGas
	GasFeeCap *big.Int // a.k.a. maxFeePerGas
	Time      time.Time

	R, S, V *big.Int
	Dna     []byte

	From    Address
	Payload []byte
}

func (tx *GSTransaction) dna() []byte {
	return tx.Dna
}

func (tx *GSTransaction) copy() TxData {
	cpy := &GSTransaction{
		Nonce:    tx.Nonce,
		To:       copyAddressPtr(tx.To),
		Gas:      tx.Gas,
		GasPrice: tx.GasPrice,
		Value:    tx.Value,
		R:        new(big.Int),
		S:        new(big.Int),
		Data:     tx.Data,
		Dna:      tx.Dna,
		From:     tx.From,
		Payload:  tx.Payload,
	}
	return cpy
}

func (tx *GSTransaction) txType() byte {
	return LegacyTxType
}

func (tx *GSTransaction) nonce() uint64 {
	return tx.Nonce
}

func (tx *GSTransaction) setSignatureValues(chainId, r, s, v *big.Int) {
	tx.R, tx.S, tx.V = r, s, v
}

func (tx *GSTransaction) getData() []byte {
	return tx.Data
}

func (tx GSTransaction) rawSignatureValues() (R, S, V *big.Int) {
	return tx.R, tx.S, tx.V
}

func (tx *GSTransaction) gas() uint64 {
	return tx.Gas
}

func (tx *GSTransaction) gasPrice() *big.Int {
	return tx.GasPrice
}

func (tx *GSTransaction) value() *big.Int {
	return tx.Value
}

func (tx *GSTransaction) data() []byte {
	return tx.Data
}

func (tx *GSTransaction) to() *Address {
	return tx.To
}

func (tx *GSTransaction) chainID() *big.Int   { return tx.chainID() }
func (tx *GSTransaction) gasFeeCap() *big.Int { return tx.GasFeeCap }
func (tx *GSTransaction) gasTipCap() *big.Int { return tx.GasTipCap }
func (tx *GSTransaction) time() time.Time     { return tx.Time }
func (tx *GSTransaction) from() Address {
	return tx.From
}

func (tx *GSTransaction) getPayload() []byte {
	return tx.Payload
}
