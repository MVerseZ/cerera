package types

import (
	"math/big"
	"time"
)

type CBTransaction struct {
	ChainID  *big.Int
	Nonce    uint64
	GasPrice *big.Int // wei per gas
	Gas      float64  // gas limit
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

func NewCoinBaseTransaction(nonce uint64,
	to Address,
	amount *big.Int,
	gasLimit float64,
	gasPrice *big.Int,
	data []byte) *GTransaction {
	return NewCoinBaseTx(&CBTransaction{
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
		Time:     time.Now(),
	})
}

func NewCoinBaseTx(inner TxData) *GTransaction {
	tx := new(GTransaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

func (tx *CBTransaction) dna() []byte {
	return tx.Dna
}

func (tx *CBTransaction) copy() TxData {
	if tx == nil {
		return nil
	}
	cpy := &CBTransaction{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		Data:  CopyBytes(tx.Data),
		Gas:   tx.Gas,
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

func (tx *CBTransaction) nonce() uint64 {
	return tx.Nonce
}

func (tx *CBTransaction) gas() float64 {
	return tx.Gas
}

func (tx *CBTransaction) gasPrice() *big.Int {
	return tx.GasPrice
}

func (tx *CBTransaction) value() *big.Int {
	return tx.Value
}

func (tx *CBTransaction) setSignatureValues(chainId, r, s, v *big.Int) {
	tx.ChainID, tx.R, tx.S, tx.V = chainId, r, s, v
}

func (tx *CBTransaction) rawSignatureValues() (r, s, v *big.Int) {
	return tx.R, tx.S, tx.V
}

func (tx *CBTransaction) data() []byte {
	return tx.Data
}

func (tx *CBTransaction) to() *Address {
	return tx.To
}

func (tx *CBTransaction) txType() byte {
	return CoinbaseTxType
}

func (tx *CBTransaction) getPayload() []byte {
	return tx.Payload
}

func (tx *CBTransaction) time() time.Time {
	return tx.Time
}

type FaucetTransaction struct {
	ChainID  *big.Int
	Nonce    uint64
	GasPrice *big.Int // wei per gas
	Gas      float64  // gas limit
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

func NewFaucetTransaction(nonce uint64, to Address, amount *big.Int) *GTransaction {
	return NewFaucetTx(&FaucetTransaction{
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Time:     time.Now(),
		Gas:      1000.0,
		GasPrice: FloatToBigInt(1000.0),
	})
}

func NewFaucetTx(inner TxData) *GTransaction {
	tx := new(GTransaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

func (tx *FaucetTransaction) dna() []byte {
	return tx.Dna
}

func (tx *FaucetTransaction) copy() TxData {
	if tx == nil {
		return nil
	}
	cpy := &FaucetTransaction{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		Data:  CopyBytes(tx.Data),
		Gas:   tx.Gas,
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

func (tx *FaucetTransaction) nonce() uint64 {
	return tx.Nonce
}

func (tx *FaucetTransaction) gas() float64 {
	return tx.Gas
}

func (tx *FaucetTransaction) gasPrice() *big.Int {
	return tx.GasPrice
}

func (tx *FaucetTransaction) value() *big.Int {
	return tx.Value
}

func (tx *FaucetTransaction) setSignatureValues(chainId, r, s, v *big.Int) {
	tx.ChainID, tx.R, tx.S, tx.V = chainId, r, s, v
}

func (tx *FaucetTransaction) rawSignatureValues() (r, s, v *big.Int) {
	return tx.R, tx.S, tx.V
}

func (tx *FaucetTransaction) data() []byte {
	return tx.Data
}

func (tx *FaucetTransaction) to() *Address {
	return tx.To
}

func (tx *FaucetTransaction) txType() byte {
	return FaucetTxType
}

func (tx *FaucetTransaction) getPayload() []byte {
	return tx.Payload
}

func (tx *FaucetTransaction) time() time.Time {
	return tx.Time
}
