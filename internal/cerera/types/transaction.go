package types

import (
	"errors"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/cerera/internal/cerera/common"
)

var (
	ErrInvalidCurveSig      = errors.New("invalid transaction v, r, s values")
	ErrUnexpectedProtection = errors.New("transaction type does not supported EIP-155 protected signatures")
	ErrInvalidTxType        = errors.New("transaction type not valid in this context")
	// ErrTxTypeNotSupported   = errors.New("transaction type not supported")
	ErrGasFeeCapTooLow = errors.New("fee cap less than base fee")
	errShortTypedTx    = errors.New("typed transaction too short")
)

// Transaction types.
const (
	UnknownTxType = iota
	FeeTxType
	AppTxType
	ApiTxtype
	LegacyTxType
)

type GTransactionType struct {
	Typ   uint8
	Wrapp byte
}

type TxData interface {
	txType() byte
	copy() TxData

	// chainID() *big.Int
	data() []byte
	gas() uint64
	gasPrice() *big.Int
	value() *big.Int
	nonce() uint64
	to() *Address

	dna() []byte
	time() time.Time

	rawSignatureValues() (r, s, v *big.Int)
	setSignatureValues(chainID, r, s, v *big.Int)

	getPayload() []byte
}

type txJSON struct {
	Data    *common.Bytes `json:"input,omitempty"`
	Message *common.Bytes `json:"message,omitempty"`
	Payload *common.Bytes `json:"payload,omitempty"`
	Type    common.Uint64 `json:"type,omitempty"`
	To      *Address      `json:"to,omitempty"`
	Time    time.Time     `json:"time,omitempty"`
	// Common transaction fields:
	Dna      *common.Bytes  `json:"dna,omitempty"`
	GasPrice *common.Big    `json:"gasPrice,omitempty"`
	Gas      *common.Uint64 `json:"gas,omitempty"`
	Nonce    *common.Uint64 `json:"nonce,omitempty"`
	Value    *common.Big    `json:"value,omitempty"`
	// V     *Big           `json:"v"`
	// R     *Big           `json:"r"`
	// S     *Big           `json:"s"`
	// Only used for encoding:
	Hash common.Hash `json:"hash"`
}

type GTransaction struct {
	inner TxData
	dna   DNA
	time  time.Time

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type DNA interface {
}

type TxByNonce GTransactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].Nonce() < s[j].Nonce() }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (tx *GTransaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))
	total.Add(total, tx.Value())
	return total
}

func NewTx(inner TxData) *GTransaction {
	tx := new(GTransaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

// WithSignature returns a new transaction with the given signature.
func (tx *GTransaction) WithSignature(signer Signer, sig []byte) (*GTransaction, error) {
	// fmt.Printf("try tx with siugnature: %x\r\n", sig)
	// fmt.Printf("with signarute tx contains %v\r\n", tx.from.Load())
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, ErrInvalidCurveSig
	}
	cpy := tx.inner.copy()
	cpy.setSignatureValues(signer.ChainID(), r, s, v)
	var sigTx = &GTransaction{inner: cpy, time: tx.time}
	if fr := tx.from.Load(); fr != nil {
		sigTx.from.Store(fr)
	}
	// fmt.Printf("after all tx from contains %v\r\n", sigTx.from.Load())
	return sigTx, nil
}

func (tx *GTransaction) RawSignatureValues() (R, S, V *big.Int) {
	cpy := tx.inner.copy()
	return cpy.rawSignatureValues()
}

func (tx *GTransaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	var h common.Hash
	if tx.Type() == LegacyTxType {
		h = crvTxHash(tx.inner)
	}
	tx.hash.Store(h)
	return h
}

func (tx GTransaction) CalculateHash() ([]byte, error) {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash).Bytes(), nil
	}

	var h common.Hash
	if tx.Type() == LegacyTxType {
		h = crvTxHash(tx.inner)
	}
	tx.hash.Store(h)
	return h.Bytes(), nil
}

func (tx GTransaction) Equals(other GTransaction) (bool, error) {
	return tx.Hash() == other.Hash(), nil
}

func (tx *GTransaction) Type() uint8 {
	return tx.inner.txType()
}

func (tx *GTransaction) Nonce() uint64 {
	return tx.inner.nonce()
}

func (tx *GTransaction) Gas() uint64 {
	return tx.inner.gas()
}

func (tx *GTransaction) GasPrice() *big.Int {
	return new(big.Int).Set(tx.inner.gasPrice())
}

func (tx *GTransaction) Value() *big.Int {
	return new(big.Int).Set(tx.inner.value())
}

func (tx *GTransaction) Data() []byte { return tx.inner.data() }

func (tx *GTransaction) Dna() []byte { return tx.inner.dna() }

func (tx *GTransaction) Size() uint64 {
	if size := tx.size.Load(); size != nil {
		return size.(uint64)
	}
	// c := writeCounter(0)
	// rlp.Encode(&c, &tx.inner)

	size := uint64(1)
	if tx.Type() != LegacyTxType {
		size += 1 // type byte
	}
	tx.size.Store(size)
	return size
}

func (tx *GTransaction) To() *Address {
	return copyAddressPtr(tx.inner.to())
}

func (tx *GTransaction) setDecoded(inner TxData, size uint64) {
	// partially realized. Need copy other fileds.
	tx.dna = inner.dna()
	tx.time = time.Now()
	tx.inner = inner
	tx.hash.Store(tx.Hash())
	if size > 0 {
		tx.size.Store(size)
	}
}

func (tx *GTransaction) SetTime(t time.Time) {
	tx.time = t
}

func (tx *GTransaction) GetTime() time.Time {
	return tx.time
}

func (tx *GTransaction) From() Address {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		return sigCache.from
	} else {
		return Address{0x0}
	}
}

func (tx *GTransaction) UpdateDna(dna []byte) {
	if len(dna) < 128 {
		tx.dna = dna
	}
}

type PGTransaction struct {
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
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
		Payload:  payload,
	})
}

func NewTransaction(nonce uint64,
	to Address,
	amount *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte) *GTransaction {
	return NewTx(&PGTransaction{
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

type GTransactions []*GTransaction

func (gtxs *GTransactions) Size() int {
	return len(*gtxs)
}

// ///////////////////////////
// ////Fee shit
// //////////////////////////

func (tx *GTransaction) ComparePrice(other *GTransaction) int {
	return tx.inner.gasPrice().Cmp(other.inner.gasPrice())
}
