package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
)

func TestKxAddress(t *testing.T) {
	var curve = elliptic.P256()
	var k1, err = ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Error(err)
	}
	var addr = PubkeyToAddress(k1.PublicKey)
	var s, _ = PublicKeyToString(&k1.PublicKey)
	var pk, _ = PublicKeyFromString(s)
	if pk.X == k1.X {
		t.Fatal("diff keys")
	}
	fmt.Printf("=== EXEC	Generating address: %s\r\n", addr)
}

func TestKeyToAddress(t *testing.T) {
	var curve = elliptic.P256()
	var k1, err = ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Error(err)
	}
	var addrPub = PubkeyToAddress(k1.PublicKey)
	var addrPriv = PrivKeyToAddress(*k1)
	if addrPub != addrPriv {
		t.Fatalf("Different addresses: %s %s", addrPub.String(), addrPriv.String())
	}
}

// TestEncodePublicKeyToByte tests the EncodePublicKeyToByte function.
func TestEncodePublicKeyToByte(t *testing.T) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Encode the public key
	pemEncodedPub := EncodePublicKeyToByte(&privateKey.PublicKey)

	// Check if the encoded public key is not empty
	if len(pemEncodedPub) == 0 {
		t.Fatalf("Encoded public key should not be empty")
	}
}

// TestDecodePrivKey tests the DecodePrivKey function.
func TestDecodePrivKey(t *testing.T) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Encode the private key to PEM format
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: x509Encoded})

	// Decode the private key
	decodedPrivateKey := DecodePrivKey(string(pemEncoded))

	// Check if the decoded private key matches the original private key
	if !privateKey.Equal(decodedPrivateKey) {
		t.Fatalf("Decoded private key does not match the original private key")
	}
}

// TestDecodePrivateAndPublicKey tests the DecodePrivateAndPublicKey function.
func TestDecodePrivateAndPublicKey(t *testing.T) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Encode the private key to PEM format
	x509EncodedPriv, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncodedPriv := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: x509EncodedPriv})

	// Encode the public key to PEM format
	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	// Decode the private and public key
	decodedPrivateKey, decodedPublicKey := DecodePrivateAndPublicKey(string(pemEncodedPriv), string(pemEncodedPub))

	// Check if the decoded private key matches the original private key
	if !privateKey.Equal(decodedPrivateKey) {
		t.Fatalf("Decoded private key does not match the original private key")
	}

	// Check if the decoded public key matches the original public key
	if !privateKey.PublicKey.Equal(decodedPublicKey) {
		t.Fatalf("Decoded public key does not match the original public key")
	}
}

// TestEncodeKeys tests the EncodeKeys function.
func TestEncodeKeys(t *testing.T) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Encode the keys
	pemEncodedPriv, pemEncodedPub := EncodeKeys(privateKey)

	// Check if the encoded private key is not empty
	if len(pemEncodedPriv) == 0 {
		t.Fatalf("Encoded private key should not be empty")
	}

	// Check if the encoded public key is not empty
	if len(pemEncodedPub) == 0 {
		t.Fatalf("Encoded public key should not be empty")
	}

	// Decode the private key
	block, _ := pem.Decode([]byte(pemEncodedPriv))
	x509Encoded := block.Bytes
	decodedPrivateKey, err := x509.ParseECPrivateKey(x509Encoded)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	// Decode the public key
	// blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	// x509EncodedPub := blockPub.Bytes
	// genericPublicKey, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	// if err != nil {
	// 	t.Fatalf("Failed to parse public key: %v", err)
	// }
	// decodedPublicKey := genericPublicKey.(*ecdsa.PublicKey)

	// Check if the decoded private key matches the original private key
	if !privateKey.Equal(decodedPrivateKey) {
		t.Fatalf("Decoded private key does not match the original private key")
	}

	// Check if the decoded public key matches the original public key
	// if !privateKey.PublicKey.Equal(decodedPublicKey) {
	// 	t.Fatalf("Decoded public key does not match the original public key")
	// }
}

// TestEncodePrivateKeyToToString tests the EncodePrivateKeyToToString function.
func TestEncodePrivateKeyToToString(t *testing.T) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Encode the private key
	pemEncodedPriv := EncodePrivateKeyToToString(privateKey)

	// Check if the encoded private key is not empty
	if len(pemEncodedPriv) == 0 {
		t.Fatalf("Encoded private key should not be empty")
	}

	// Decode the private key
	block, _ := pem.Decode([]byte(pemEncodedPriv))
	x509Encoded := block.Bytes
	decodedPrivateKey, err := x509.ParseECPrivateKey(x509Encoded)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	// Check if the decoded private key matches the original private key
	if !privateKey.Equal(decodedPrivateKey) {
		t.Fatalf("Decoded private key does not match the original private key")
	}
}
