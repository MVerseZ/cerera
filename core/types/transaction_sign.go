package types

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrInvalidSig = errors.New("invalid base64 signature")
	errBadKey     = errors.New("bad key")
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
	// fmt.Printf("Sign tx %s with key: %s\r\n", tx.Hash(), prv.D)
	h := s.Hash(tx)
	sig, err := Sign(h[:], prv)
	if err != nil {
		return nil, errBadKey
	}
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		sigCache.from = crypto.PrivKeyToAddress(*prv)
		sigCache.signer = s
		tx.from.Store(sigCache)
	} else {
		var sgCache = sigCache{
			from: crypto.PrivKeyToAddress(*prv),
		}
		sgCache.signer = s
		tx.from.Store(sgCache)
	}
	var signTx, errSign = tx.WithSignature(s, sig)
	if err != nil {
		fmt.Errorf("%s", errSign)
		fmt.Printf("Error while sign tx %s from: %s\r\n", tx.Hash(), tx.From())
	}
	// fmt.Printf("Success sign tx %s from: %s\r\n", tx.Hash(), tx.From())
	return signTx, nil
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

// simple signer
type SimpleSigner struct {
	chainId, chainIdMul *big.Int
}

func NewSimpleSigner(chainId *big.Int) Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return SimpleSigner{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
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
	R, S, V = decodeSignature(sig)
	return R, S, V, nil
}

func (fs SimpleSigner) SignTransaction(tx *GTransaction, k *ecdsa.PrivateKey) (common.Hash, error) {
	sTx, err2 := SignTx(tx, fs, k)
	if err2 != nil {
		fmt.Printf("Error while sign tx: %s\r\n", tx.Hash())
		return common.EmptyHash(), errors.New("error while sign tx")
	}
	return sTx.Hash(), nil
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

func recoverPlain(sighash common.Hash, R, S, V *big.Int, a bool) (Address, error) {
	// Use the same slice range as PubkeyToAddress and PrivKeyToAddress
	pubBytes := crypto.FromECDSAPub(&ecdsa.PublicKey{
		Curve: crypto.ChainElliptic(),
		X:     R,
		Y:     S,
	})
	return address.BytesToAddress(crypto.INRISeq(pubBytes[1:])[32:]), nil
}

// removed custom pubkey extraction; legacy recovery kept

func decodeSignature(sig []byte) (r, s, v *big.Int) {
	// fmt.Printf("decode signature len: %v\r\n", len(sig))
	if len(sig) < 96 {
		return new(big.Int), new(big.Int), new(big.Int)
	}
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
