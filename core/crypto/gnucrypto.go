package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"math/big"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrInvalidMsgLen       = errors.New("invalid message length, need 32 bytes")
	ErrInvalidSignatureLen = errors.New("invalid signature length")
	ErrInvalidRecoveryID   = errors.New("invalid signature recovery id")
	ErrInvalidKey          = errors.New("invalid private key")
	ErrInvalidPubkey       = errors.New("invalid public key")
	ErrSignFailed          = errors.New("signing failed")
	ErrRecoverFailed       = errors.New("recovery failed")
)

const SignatureLength = 64 + 1

var (
	chainElliptic = elliptic.P256()
)

// ChainElliptic returns the curve used for ECDSA (exported for types package)
func ChainElliptic() elliptic.Curve { return chainElliptic }

// as keccak
type INRI interface {
	hash.Hash
}

func NewINRISeq() INRI {
	state, _ := blake2b.New512(nil)
	return state.(INRI)
}

func INRISeq(data ...[]byte) []byte {
	d := NewINRISeq()

	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func INRISeqHash(data ...[]byte) (h common.Hash) {
	d := NewINRISeq()

	for _, b := range data {
		d.Write(b)
	}

	return common.BytesToHash(d.Sum(h[:]))
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	// return append(pub.X.Bytes(), pub.Y.Bytes()...)
	return elliptic.Marshal(chainElliptic, pub.X, pub.Y)
}

func PubkeyToAddress(p ecdsa.PublicKey) address.Address {
	pubBytes := FromECDSAPub(&p)
	// Skip the format byte (0x04) and take the last 32 bytes of the hash
	return address.BytesToAddress(INRISeq(pubBytes[1:])[32:])
}

func PrivKeyToAddress(p ecdsa.PrivateKey) address.Address {
	pubBytes := FromECDSAPub(&p.PublicKey)
	// Skip the format byte (0x04) and take the last 32 bytes of the hash
	return address.BytesToAddress(INRISeq(pubBytes[1:])[32:])
}

// Base58Encode encodes a byte slice to a base58 string
func Base58Encode(input []byte) string {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Encoding as big-endian integers
	x := new(big.Int).SetBytes(input)

	// Encoding base58
	output := ""
	radix := big.NewInt(int64(len(alphabet)))
	zero := big.NewInt(0)
	for x.Cmp(zero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, radix, mod)
		output = string(alphabet[mod.Int64()]) + output
	}

	// Encoding leading zeros
	for _, b := range input {
		if b != 0 {
			break
		}
		output = string(alphabet[0]) + output
	}

	return output
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(chainElliptic, rand.Reader)
}

// unused
func checkSignature(sig []byte) error {
	if len(sig) != 65 {
		return ErrInvalidSignatureLen
	}
	if sig[64] >= 4 {
		return ErrInvalidRecoveryID
	}
	return nil
}

func PublicKeyFromString(s string) (*ecdsa.PublicKey, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	curve := chainElliptic // use the same curve as in publicKeyToString
	x, y := elliptic.Unmarshal(curve, decoded)
	if x == nil || y == nil {
		return nil, fmt.Errorf("invalid encoding")
	}
	publicKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}
	return publicKey, nil
}

func PublicKeyToString(publicKey *ecdsa.PublicKey) (string, error) {
	curve := chainElliptic
	if curve == nil {
		return "", fmt.Errorf("public key curve is nil")
	}
	x := publicKey.X
	y := publicKey.Y
	encoded := elliptic.Marshal(curve, x, y)
	return hex.EncodeToString(encoded), nil
}

func GenerateAccount() (*ecdsa.PrivateKey, error) {
	pk, err := ecdsa.GenerateKey(chainElliptic, rand.Reader)
	if err != nil {
		panic(err)
	}
	return pk, nil
}

func EncodeKeys(privateKey *ecdsa.PrivateKey) (string, string) {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(privateKey.PublicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	return string(pemEncoded), string(pemEncodedPub)
}

func EncodePrivateKeyToToString(pk *ecdsa.PrivateKey) string {
	x509Encoded, _ := x509.MarshalECPrivateKey(pk)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	return string(pemEncoded)
}

func EncodePrivateKeyToByte(pk *ecdsa.PrivateKey) []byte {
	x509Encoded, _ := x509.MarshalECPrivateKey(pk)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})
	return pemEncoded
}

func EncodePublicKeyToByte(pub *ecdsa.PublicKey) []byte {
	x509Encoded, _ := x509.MarshalPKIXPublicKey(pub)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509Encoded})
	return pemEncoded
}

func DecodeByteToPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}
	x509Encoded := block.Bytes
	pub, err := x509.ParsePKIXPublicKey(x509Encoded)
	if err != nil {
		return nil, err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}
	return ecdsaPub, nil
}

func DecodePrivKey(pemEncoded string) *ecdsa.PrivateKey {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

	return privateKey
}

func DecodePrivateAndPublicKey(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, _ := x509.ParsePKIXPublicKey(x509EncodedPub)
	publicKey := genericPublicKey.(*ecdsa.PublicKey)

	return privateKey, publicKey
}
