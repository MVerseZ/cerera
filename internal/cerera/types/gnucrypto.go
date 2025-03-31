package types

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"hash"
	"math/big"

	"github.com/cerera/internal/cerera/common"
	"golang.org/x/crypto/blake2b"
)

const SignatureLength = 64 + 1

var (
	chainElliptic     = elliptic.P256()
	chainEllipticEcdh = ecdh.P256()
)

// as keccak
type INRI interface {
	hash.Hash
}

func NewINRISeq() INRI {
	state, _ := blake2b.New512(nil)
	return state.(INRI)
}

func INRISeq(data ...[]byte) []byte {
	b := make([]byte, 48)
	d := NewINRISeq()

	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(b)
}

func INRISeqHash(data ...[]byte) (h common.Hash) {
	d := NewINRISeq()

	for _, b := range data {
		d.Write(b)
	}

	return common.BytesToHash(d.Sum(h[:]))
}

func FromECDSAPub(pub *ecdh.PublicKey) []byte {
	return pub.Bytes()
}

func PubkeyToAddress(p ecdh.PublicKey) Address {
	pubBytes := FromECDSAPub(&p)
	return BytesToAddress(INRISeq(pubBytes[1:])[16:])
}

func PrivKeyToAddress(p *ecdh.PrivateKey) Address {
	pubBytes := FromECDSAPub(p.PublicKey())

	return BytesToAddress(INRISeq(pubBytes[1:])[32:])
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

func GenerateKey() (*ecdh.PrivateKey, error) {
	return chainEllipticEcdh.GenerateKey(rand.Reader)
}

// unused
// func checkSignature(sig []byte) error {
// 	if len(sig) != 65 {
// 		return ErrInvalidSignatureLen
// 	}
// 	if sig[64] >= 4 {
// 		return ErrInvalidRecoveryID
// 	}
// 	return nil
// }

func PublicKeyFromString(s string) (*ecdh.PublicKey, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	// curve := chainElliptic
	pubKey, err := ecdh.P256().NewPublicKey(decoded)
	if err != nil {
		return nil, fmt.Errorf("invalid public key encoding: %v", err)
	}

	return pubKey, nil
}

func PublicKeyToString(publicKey *ecdh.PublicKey) (string, error) {
	curve := chainElliptic
	if curve == nil {
		return "", fmt.Errorf("public key curve is nil")
	}
	encoded := publicKey.Bytes()
	return hex.EncodeToString(encoded), nil
}

func GenerateAccount() (*ecdh.PrivateKey, error) {
	return chainEllipticEcdh.GenerateKey(rand.Reader)
}

func EncodeKeys(privateKey *ecdh.PrivateKey) (string, string) {
	ecdsaKey := ECDHToECDSAPrivate(privateKey)
	x509Encoded, _ := x509.MarshalECPrivateKey(ecdsaKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(privateKey.PublicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	return string(pemEncoded), string(pemEncodedPub)
}

func EncodePrivateKeyToToString(pk *ecdh.PrivateKey) string {
	// Convert ECDH to ECDSA private key first
	ecdsaKey := ECDHToECDSAPrivate(pk)

	x509Encoded, _ := x509.MarshalECPrivateKey(ecdsaKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	return string(pemEncoded)
}

func EncodePrivateKeyToByte(pk *ecdh.PrivateKey) []byte {
	// Convert ECDH private key to ECDSA private key first
	ecdsaKey := ECDHToECDSAPrivate(pk)

	// Now marshal the ECDSA key
	x509Encoded, _ := x509.MarshalECPrivateKey(ecdsaKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})
	return pemEncoded
}

func EncodePublicKeyToByte(pub *ecdh.PublicKey) []byte {
	x509Encoded, _ := x509.MarshalPKIXPublicKey(pub)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509Encoded})
	return pemEncoded
}

func DecodeByteToPublicKey(data []byte) (*ecdh.PublicKey, error) {
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
	ecdhPub, err := chainEllipticEcdh.NewPublicKey(elliptic.Marshal(chainElliptic, ecdsaPub.X, ecdsaPub.Y))
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ECDH key: %v", err)
	}
	return ecdhPub, nil
}

func DecodePrivKey(pemEncoded string) *ecdh.PrivateKey {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

	// Convert ECDSA to ECDH private key
	ecdhKey, _ := chainEllipticEcdh.NewPrivateKey(privateKey.D.Bytes())
	return ecdhKey
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

func ECDHToECDSAPrivate(ecdhKey *ecdh.PrivateKey) *ecdsa.PrivateKey {
	// Get the raw private key bytes
	d := ecdhKey.Bytes()

	// Create new ECDSA private key
	privKey := new(ecdsa.PrivateKey)
	privKey.D = new(big.Int).SetBytes(d)
	privKey.Curve = chainElliptic

	// Calculate public key components
	// FY DEPRECATE LEGACY SHIT
	privKey.X, privKey.Y = privKey.Curve.ScalarBaseMult(d)

	return privKey
}

// Add this function to convert ECDH public key to ECDSA public key
func ECDHToECDSAPublic(ecdhPub *ecdh.PublicKey) *ecdsa.PublicKey {
	pubBytes := ecdhPub.Bytes()
	x, y := elliptic.UnmarshalCompressed(chainElliptic, pubBytes)
	return &ecdsa.PublicKey{Curve: chainElliptic, X: x, Y: y}
}

func ECDSAToECDHPrivate(ecdsaKey *ecdsa.PrivateKey) *ecdh.PrivateKey {
	ecdhKey, _ := chainEllipticEcdh.NewPrivateKey(ecdsaKey.D.Bytes())
	return ecdhKey
}
