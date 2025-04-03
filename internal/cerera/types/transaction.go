package types

import (
	"encoding/binary"
	"errors"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/cerera/internal/cerera/common"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrInvalidCurveSig      = errors.New("invalid transaction v, r, s values")
	ErrUnexpectedProtection = errors.New("transaction type does not supported EIP-155 protected signatures")
	ErrInvalidTxType        = errors.New("transaction type not valid in this context")
	// ErrTxTypeNotSupported   = errors.New("transaction type not supported")
	ErrGasFeeCapTooLow = errors.New("fee cap less than base fee")
)

// Transaction types.
const (
	UnknownTxType = iota
	FeeTxType
	AppTxType
	ApiTxtype
	LegacyTxType
	CoinbaseTxType
	FaucetTxType
)

type TxStatus byte

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
	V        *Big           `json:"v"`
	R        *Big           `json:"r"`
	S        *Big           `json:"s"`
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

// Content represents the data that is stored and verified by the tree. A type that
// implements this interface can be used as an item in the tree.
type Content interface {
	CalculateHash() ([]byte, error)
	Equals(other Content) (bool, error)
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

func CreateTransaction(nonce uint64, addressTo Address, count float64, gas uint64, message string) (*GTransaction, error) {
	var tx = NewTransaction(
		nonce,
		addressTo,
		FloatToBigInt(count),
		gas,
		big.NewInt(0),
		[]byte(message),
	)
	return tx, nil
}

func CreateUnbroadcastTransaction(nonce uint64, addressTo Address, count float64, gas uint64, message string) (*GTransaction, error) {
	// check max size of tx here
	if len(message) < 1024 {
		var tx = NewTransaction(
			nonce,
			addressTo,
			FloatToBigInt(count),
			gas,
			big.NewInt(0),
			[]byte(message),
		)
		return tx, nil
	} else {
		return nil, ErrInvalidMsgLen
	}
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
	if tx.Type() == CoinbaseTxType {
		h = crvTxHash(tx.inner)
	}
	if tx.Type() == FaucetTxType {
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
	if tx.Type() == CoinbaseTxType {
		h = crvTxHash(tx.inner)
	}
	if tx.Type() == FaucetTxType {
		h = crvTxHash(tx.inner)
	}
	tx.hash.Store(h)
	return h.Bytes(), nil
}

func (tx GTransaction) CompareHash(other GTransaction) (bool, error) {
	return tx.Hash().Compare(other.Hash()) == 0, nil
}

func (tx GTransaction) Equals(other Content) (bool, error) {
	otherTxHash, err := other.(GTransaction).CalculateHash()
	if err != nil {
		return false, errors.New("value is not of type Transaction")
	}
	return tx.Hash() == common.BytesToHash(otherTxHash), nil
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

	txEncData, _ := tx.MarshalJSON()
	size := uint64(len(txEncData))

	// if tx.Type() != LegacyTxType {
	// size += 1 // type byte
	// }
	tx.size.Store(size)
	return size
}

func (tx *GTransaction) To() *Address {
	return copyAddressPtr(tx.inner.to())
}

func (tx *GTransaction) setDecoded(inner TxData, size uint64) {
	// partially realized. Need copy other fileds.
	tx.dna = inner.dna()
	tx.time = inner.time()
	tx.inner = inner
	tx.hash.Store(tx.Hash())
	if size > 0 {
		tx.size.Store(size)
	} else {
		tx.size.Store(tx.Size())
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

func (tx *GTransaction) IsSigned() bool {
	var r, s, v = tx.RawSignatureValues()
	// fmt.Printf("sig values: %d %d %d\r\n", r, s, v)
	r, s, v = tx.inner.rawSignatureValues()
	// fmt.Printf("inner sig values: %d %d %d\r\n", r, s, v)
	return big.NewInt(0).Cmp(r) == -1 && big.NewInt(0).Cmp(s) == -1 && big.NewInt(0).Cmp(v) == -1
}

type GTransactions []*GTransaction

func (gtxs *GTransactions) Size() int {
	return len(*gtxs)
}

func crvTxHash(t TxData) (h common.Hash) {
	hw, _ := blake2b.New256(nil)
	// hw, _ := blake2b.New256(nil)

	tNonce := make([]byte, 8)
	tGas := make([]byte, 16)
	binary.BigEndian.PutUint64(tNonce, t.nonce())
	binary.BigEndian.PutUint64(tGas, t.gas())

	hw.Write(h[:0])
	hw.Write(t.data())
	hw.Write(t.dna())
	hw.Write(t.value().Bytes())
	hw.Write(tNonce)
	hw.Write(t.to()[:])
	hw.Write(t.gasPrice().Bytes())
	hw.Write(tGas)

	dateBytes, _ := t.time().MarshalBinary()
	hw.Write(dateBytes)
	h.SetBytes(hw.Sum(nil))
	return h
}

// ///////////////////////////
// ////Fee shit
// //////////////////////////

func (tx *GTransaction) ComparePrice(other *GTransaction) int {
	return tx.inner.gasPrice().Cmp(other.inner.gasPrice())
}
