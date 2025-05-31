package types

import (
	"math/big"
	"time"
)

type PGTransaction struct {
	Status   TxStatus
	ChainID  *big.Int
	Nonce    uint64
	GasPrice *big.Int // wei per gas
	Gas      uint64   // gas limit
	To       *Address `rlp:"nil"` // nil means contract creation
	Value    *big.Int
	Data     []byte
	R        *big.Int `json:"r" gencodec:"required"`
	S        *big.Int `json:"s" gencodec:"required"`
	V        *big.Int
	Dna      []byte    `json:"dna" gencodec:"required"`
	Time     time.Time `json:"time" gencodec:"required"`

	Payload []byte
	FullGas *big.Int
}

func NewTransactionEnrich(nonce uint64,
	to Address,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte,
	payload []byte) *GTransaction {
	return NewTx(&PGTransaction{
		Status:   0x1,
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
		Payload:  payload,
	})
}

func NewTransaction(
	nonce uint64,
	to Address,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte) *GTransaction {
	return NewTx(&PGTransaction{
		Status:   0x1,
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
		Time:     time.Now(),
	})
}

func (tx *PGTransaction) dna() []byte {
	return tx.Dna
}

func (tx *PGTransaction) copy() TxData {
	if tx == nil {
		return nil
	}
	cpy := &PGTransaction{
		Status: tx.Status,
		Nonce:  tx.Nonce,
		To:     copyAddressPtr(tx.To),
		Data:   CopyBytes(tx.Data),
		Gas:    tx.Gas,
		// atomic
		Value:    new(big.Int),
		GasPrice: new(big.Int),
		V:        new(big.Int),
		R:        new(big.Int),
		S:        new(big.Int),
		Payload:  CopyBytes(tx.Payload),
		Time:     tx.time(),
	}
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.GasPrice != nil {
		cpy.GasPrice.Set(tx.GasPrice)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	return cpy
}

func (tx *PGTransaction) nonce() uint64 {
	return tx.Nonce
}

func (tx *PGTransaction) gas() uint64 {
	return tx.Gas
}

func (tx *PGTransaction) gasPrice() *big.Int {
	return tx.GasPrice
}

func (tx *PGTransaction) value() *big.Int {
	return tx.Value
}

func (tx *PGTransaction) setSignatureValues(chainId, r, s, v *big.Int) {
	tx.ChainID, tx.R, tx.S, tx.V = chainId, r, s, v
}

func (tx *PGTransaction) rawSignatureValues() (r, s, v *big.Int) {
	return tx.R, tx.S, tx.V
}

func (tx *PGTransaction) data() []byte {
	return tx.Data
}

func (tx *PGTransaction) to() *Address {
	return tx.To
}

func (tx *PGTransaction) txType() byte {
	return LegacyTxType
}

func (tx *PGTransaction) getPayload() []byte {
	return tx.Payload
}

func (tx *PGTransaction) time() time.Time {
	return tx.Time
}

func (tx *PGTransaction) status() TxStatus {
	return tx.Status
}
func copyAddressPtr(a *Address) *Address {
	if a == nil {
		return nil
	}
	cpy := *a
	return &cpy
}

func CopyBytes(b []byte) (copiedBytes []byte) {
	if b == nil {
		return nil
	}
	copiedBytes = make([]byte, len(b))
	copy(copiedBytes, b)
	return copiedBytes
}
