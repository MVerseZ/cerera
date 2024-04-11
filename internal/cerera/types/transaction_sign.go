package types

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"reflect"

	"github.com/cerera/internal/cerera/common"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrUnknownEntry = errors.New("unknown entry type")
	errNoPubkey     = errors.New("missing public key")
	errBadPubkey    = errors.New("invalid public key")
	errInvalidENR   = errors.New("invalid node record")
	errInvalidChild = errors.New("invalid child hash")
	ErrInvalidSig   = errors.New("invalid base64 signature")
	errSyntax       = errors.New("invalid syntax")
	ErrBadKey       = errors.New("Bad key")
)

type sigCache struct {
	signer Signer
	from   Address
}

// in this file only work with GTransacion now

func Sign(msg []byte, privKey *ecdsa.PrivateKey) ([]byte, error) {
	// fmt.Println("message lenght (tx): ", len(msg))
	h := blake2b.Sum256(msg)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, h[:])

	if err != nil {
		return nil, err
	}
	// Get the byte length of the curve order
	n := (privKey.Curve.Params().N.BitLen() + 7) / 8
	rb := r.Bytes()
	sb := s.Bytes()
	signature := make([]byte, 2*n)
	copy(signature[n-len(rb):], rb)
	copy(signature[2*n-len(sb):], sb)

	xBytes, yBytes := privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()
	backup := make([]byte, 0)
	backup = append(backup, xBytes...)
	backup = append(backup, yBytes...)

	return append(signature, backup...), nil
}

func SignTx(tx *GTransaction, s Signer, prv *ecdsa.PrivateKey) (*GTransaction, error) {
	h := s.Hash(tx)
	sig, err := Sign(h[:], prv)
	if err != nil {
		return nil, ErrBadKey
	}
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		sigCache.from = PrivKeyToAddress(*prv)
		sigCache.signer = s
		tx.from.Store(sigCache)
	} else {
		var sgCache = sigCache{
			from: PrivKeyToAddress(*prv),
		}
		sgCache.signer = s
		tx.from.Store(sgCache)
	}
	return tx.WithSignature(s, sig)
}

func Sender(signer Signer, tx *GTransaction) (Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return Address{0x0}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

type SimpleSigner struct {
	chainId, chainIdMul *big.Int
	pen                 *ecdsa.PrivateKey
}

func (ss SimpleSigner) ChainID() *big.Int {
	return ss.chainId
}

func (s1 SimpleSigner) Equal(s2 Signer) bool {
	ss, ok := s2.(SimpleSigner)
	return ok && ss.chainId.Cmp(s1.chainId) == 0
}

func (fs SimpleSigner) Hash(tx *GTransaction) common.Hash {
	return crvTxHash(tx.inner)
}

func (fs SimpleSigner) Pen() *ecdsa.PrivateKey {
	return fs.pen
}

func crvHash(x interface{}) (h common.Hash) {
	hw, _ := blake2b.New256(nil)
	// rlp.Encode(hw, x)

	t := reflect.TypeOf(x).Elem()
	var s []string
	for i := 0; i < t.NumMethod(); i++ {
		s = append(s, t.Method(i).Name)
	}
	hw.Write(h[:0])
	hw.Sum(h[:0])
	return h
}

func crvTxHash(t TxData) (h common.Hash) {
	hw, _ := blake2b.New256(nil)

	tNonce := make([]byte, 16)
	tGas := make([]byte, 16)
	binary.LittleEndian.PutUint64(tNonce, t.nonce())
	binary.LittleEndian.PutUint64(tGas, t.gas())

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

func (fs SimpleSigner) Sender(tx *GTransaction) (Address, error) {
	if tx.Type() != LegacyTxType {
		return Address{}, ErrTxTypeNotSupported
	}
	r, s, v := tx.RawSignatureValues()
	if len(r.Bits()) > 0 && len(s.Bits()) > 0 {
		return recoverPlain(fs.Hash(tx), r, s, v, false)
	} else {
		return Address{}, ErrInvalidSig
	}
}

func (fs SimpleSigner) SignatureValues(tx *GTransaction, sig []byte) (R, S, V *big.Int, err error) {
	// txdata, ok := tx.inner.(*PGTransaction)
	R, S, V = decodeSignature(sig)
	return R, S, V, nil
}

func recoverPlain(sighash common.Hash, R, S, V *big.Int, a bool) (Address, error) {
	return BytesToAddress(INRISeq(append(R.Bytes())[1:])[12:]), nil
}

func decodeSignature(sig []byte) (r, s, v *big.Int) {
	// fmt.Printf("decode signature len: %v\r\n", len(sig))
	rsSig := sig[64:]
	vSig := sig[1:][:16]
	r = new(big.Int).SetBytes(rsSig[:32])
	s = new(big.Int).SetBytes(rsSig[32:])
	v = new(big.Int).SetBytes(vSig)
	return r, s, v
}

func VerifyECDSAWithZk(pubkey []byte, message []byte, zkProof interface{}) (bool, error) {
	return true, nil
}

func zk_verify(zkProof interface{}) bool {
	// TODO: implement zk-SNARK verification
	return true
}

func NewSimpleSignerWithPen(chainId *big.Int, pekn *ecdsa.PrivateKey) Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return SimpleSigner{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
		pen:        pekn,
	}
}
