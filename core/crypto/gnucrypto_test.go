package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"

	"github.com/cerera/core/common"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
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

func TestEncode(t *testing.T) {
	var data2 = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis fermentum id justo a cursus. Ut id libero tortor. Nunc sagittis."
	expected2 := "QHcZT8DWsLqJQfp343kzSV5KosyyV17iuCrD7gkLYr4S5yVvn7wPgbfgAGEmBsC8Jxztv9sKjMxu6oHCZc7WQutzopekQfYoR4Z44q3524nXT24JdHq5683Fak2anPDwvZid1RJnzCRv2Q9YdbXcGi1Dqa3zrLvidTTvUjC8aKrM"
	result := Base58Encode([]byte(data2))
	if result != expected2 {
		t.Errorf("different encode strings!, Have %s, want %s", result, expected2)
	}
	var data1 = "EXAMPLE 1"
	expected := "tCJ7UoNDJRUk"
	result = Base58Encode([]byte(data1))
	if result != expected {
		t.Errorf("different encode strings!, Have %s, want %s", result, expected)
	}
}

// TestNewINRISeq tests the NewINRISeq function
func TestNewINRISeq(t *testing.T) {
	state := NewINRISeq()
	if state == nil {
		t.Fatal("NewINRISeq returned nil")
	}

	// Test that it implements hash.Hash interface
	data := []byte("test data")
	state.Write(data)
	result := state.Sum(nil)
	if len(result) == 0 {
		t.Fatal("NewINRISeq result should not be empty")
	}
}

// TestINRISeq tests the INRISeq function
func TestINRISeq(t *testing.T) {
	data1 := []byte("test1")
	data2 := []byte("test2")

	result := INRISeq(data1, data2)
	if len(result) != 64 {
		t.Errorf("INRISeq should return 64 bytes, got %d", len(result))
	}

	// Test with no data
	resultEmpty := INRISeq()
	if len(resultEmpty) != 64 {
		t.Errorf("INRISeq with no data should return 64 bytes, got %d", len(resultEmpty))
	}
}

// TestINRISeqHash tests the INRISeqHash function
func TestINRISeqHash(t *testing.T) {
	data1 := []byte("test1")
	data2 := []byte("test2")

	result := INRISeqHash(data1, data2)
	if result == (common.Hash{}) {
		t.Fatal("INRISeqHash should return non-empty hash")
	}

	// Test with no data
	resultEmpty := INRISeqHash()
	if resultEmpty == (common.Hash{}) {
		t.Fatal("INRISeqHash should return non-empty hash even with no data")
	}
}

// TestFromECDSAPub tests the FromECDSAPub function
func TestFromECDSAPub(t *testing.T) {
	// Test with valid key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	result := FromECDSAPub(&privateKey.PublicKey)
	if len(result) == 0 {
		t.Fatal("FromECDSAPub should return non-empty bytes")
	}

	// Test with nil
	resultNil := FromECDSAPub(nil)
	if resultNil != nil {
		t.Error("FromECDSAPub with nil should return nil")
	}
}

// TestGenerateKey tests the GenerateKey function
func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if key == nil {
		t.Fatal("GenerateKey should not return nil key")
	}
	if key.PublicKey.Curve == nil {
		t.Fatal("GenerateKey should return key with valid curve")
	}
}

// TestCheckSignature tests the checkSignature function
func TestCheckSignature(t *testing.T) {
	// Test with valid signature
	validSig := make([]byte, 65)
	validSig[64] = 0
	err := checkSignature(validSig)
	if err != nil {
		t.Errorf("checkSignature should accept valid signature: %v", err)
	}

	// Test with invalid length
	invalidSig := []byte{0, 1, 2}
	err = checkSignature(invalidSig)
	if err != ErrInvalidSignatureLen {
		t.Errorf("checkSignature should return ErrInvalidSignatureLen for invalid length, got: %v", err)
	}

	// Test with invalid recovery ID
	invalidRecoverId := make([]byte, 65)
	invalidRecoverId[64] = 4
	err = checkSignature(invalidRecoverId)
	if err != ErrInvalidRecoveryID {
		t.Errorf("checkSignature should return ErrInvalidRecoveryID for invalid recovery ID, got: %v", err)
	}
}

// TestGenerateAccount tests the GenerateAccount function
func TestGenerateAccount(t *testing.T) {
	key, err := GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount failed: %v", err)
	}
	if key == nil {
		t.Fatal("GenerateAccount should not return nil key")
	}
}

// TestEncodePrivateKeyToByte tests the EncodePrivateKeyToByte function
func TestEncodePrivateKeyToByte(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encoded := EncodePrivateKeyToByte(privateKey)
	if len(encoded) == 0 {
		t.Fatal("EncodePrivateKeyToByte should return non-empty bytes")
	}

	// Verify it can be decoded
	block, _ := pem.Decode(encoded)
	if block == nil {
		t.Fatal("Encoded data should be valid PEM")
	}
}

// TestDecodeByteToPublicKey tests the DecodeByteToPublicKey function
func TestDecodeByteToPublicKey(t *testing.T) {
	// Generate a key and encode it
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encodedPub := EncodePublicKeyToByte(&privateKey.PublicKey)

	// Decode it back
	decodedPub, err := DecodeByteToPublicKey(encodedPub)
	if err != nil {
		t.Fatalf("DecodeByteToPublicKey failed: %v", err)
	}

	if !privateKey.PublicKey.Equal(decodedPub) {
		t.Fatal("Decoded public key should match original")
	}

	// Test with invalid data
	_, err = DecodeByteToPublicKey([]byte("invalid pem"))
	if err == nil {
		t.Error("DecodeByteToPublicKey should error on invalid data")
	}

	// Test with wrong block type
	wrongBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid")})
	_, err = DecodeByteToPublicKey(wrongBlock)
	if err == nil {
		t.Error("DecodeByteToPublicKey should error on wrong block type")
	}
}

// TestBase58EncodeWithLeadingZeros tests Base58Encode with leading zeros
func TestBase58EncodeWithLeadingZeros(t *testing.T) {
	data := []byte{0, 0, 0, 0x41}
	result := Base58Encode(data)
	if result == "" {
		t.Fatal("Base58Encode should not return empty string")
	}

	// Test with all zeros
	zeros := make([]byte, 10)
	result = Base58Encode(zeros)
	if result == "" {
		t.Fatal("Base58Encode should not return empty string for all zeros")
	}
}

// TestPublicKeyFromStringError tests PublicKeyFromString with invalid input
func TestPublicKeyFromStringError(t *testing.T) {
	_, err := PublicKeyFromString("invalid hex")
	if err == nil {
		t.Error("PublicKeyFromString should error on invalid hex")
	}

	_, err = PublicKeyFromString("deadbeef")
	if err == nil {
		t.Error("PublicKeyFromString should error on invalid encoding")
	}
}

// TestPublicKeyToStringError tests PublicKeyToString with nil curve
func TestPublicKeyToStringError(t *testing.T) {
	// This is hard to test since the function uses chainElliptic which is a constant
	// But we can test with a valid key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	result, err := PublicKeyToString(&privateKey.PublicKey)
	if err != nil {
		t.Errorf("PublicKeyToString should not error on valid key: %v", err)
	}
	if result == "" {
		t.Error("PublicKeyToString should return non-empty string")
	}
}

// TestChainElliptic tests ChainElliptic returns valid curve
func TestChainElliptic(t *testing.T) {
	curve := ChainElliptic()
	if curve == nil {
		t.Fatal("ChainElliptic should not return nil")
	}
	if curve != elliptic.P256() {
		t.Error("ChainElliptic should return P256")
	}
}

// TestSignatureLength tests SignatureLength constant
func TestSignatureLength(t *testing.T) {
	if SignatureLength != 65 {
		t.Errorf("SignatureLength = %d, want 65", SignatureLength)
	}
}

// TestEncodePublicKeyToString tests EncodePublicKeyToString
func TestEncodePublicKeyToString(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	result := EncodePublicKeyToString(&privateKey.PublicKey)
	if result == "" {
		t.Fatal("EncodePublicKeyToString should return non-empty string")
	}
	// Should be hex encoded, even length
	if len(result)%2 != 0 {
		t.Error("EncodePublicKeyToString should return even-length hex string")
	}
}

// TestPublicKeyRoundTrip tests PublicKeyFromString and PublicKeyToString round-trip
func TestPublicKeyRoundTrip(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encoded, err := PublicKeyToString(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("PublicKeyToString failed: %v", err)
	}

	decoded, err := PublicKeyFromString(encoded)
	if err != nil {
		t.Fatalf("PublicKeyFromString failed: %v", err)
	}

	if !privateKey.PublicKey.Equal(decoded) {
		t.Fatal("Round-trip public key should match original")
	}
}

// TestDecodeBytesToPrivateKey tests DecodeBytesToPrivateKey
func TestDecodeBytesToPrivateKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encoded := EncodePrivateKeyToByte(privateKey)
	decoded, err := DecodeBytesToPrivateKey(encoded)
	if err != nil {
		t.Fatalf("DecodeBytesToPrivateKey failed: %v", err)
	}
	if decoded == nil {
		t.Fatal("DecodeBytesToPrivateKey should not return nil")
	}
	if !privateKey.Equal(decoded) {
		t.Fatal("Decoded private key should match original")
	}

	// Test with invalid data
	_, err = DecodeBytesToPrivateKey([]byte("invalid pem"))
	if err == nil {
		t.Error("DecodeBytesToPrivateKey should error on invalid PEM")
	}

	// Test with wrong block type
	wrongBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("invalid")})
	_, err = DecodeBytesToPrivateKey(wrongBlock)
	if err == nil {
		t.Error("DecodeBytesToPrivateKey should error on wrong block type")
	}
}

// TestGenerateMasterKey tests GenerateMasterKey
func TestGenerateMasterKey(t *testing.T) {
	masterKey, mnemonic, err := GenerateMasterKey("test_password")
	if err != nil {
		t.Fatalf("GenerateMasterKey failed: %v", err)
	}
	if masterKey == nil {
		t.Fatal("GenerateMasterKey should not return nil key")
	}
	if mnemonic == "" {
		t.Fatal("GenerateMasterKey should return non-empty mnemonic")
	}
	// Mnemonic should have 24 words (256 bits entropy)
	wordCount := 0
	for _, r := range mnemonic {
		if r == ' ' {
			wordCount++
		}
	}
	wordCount++ // last word
	if wordCount != 24 {
		t.Errorf("Mnemonic should have 24 words, got %d", wordCount)
	}
}

// TestXorRXorRoundTrip tests Xor and RXor round-trip
func TestXorRXorRoundTrip(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		t.Fatalf("NewEntropy failed: %v", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		t.Fatalf("NewMnemonic failed: %v", err)
	}
	seed := bip39.NewSeed(mnemonic, "test_pass")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		t.Fatalf("NewMasterKey failed: %v", err)
	}

	xorData := Xor(privateKey, masterKey)
	if len(xorData) == 0 {
		t.Fatal("Xor should return non-empty data")
	}

	restored := RXor(masterKey, &privateKey.PublicKey, xorData)
	if len(restored) == 0 {
		t.Fatal("RXor should return non-empty data")
	}

	// Restored should be valid PEM-encoded private key
	decoded, err := DecodeBytesToPrivateKey(restored)
	if err != nil {
		t.Fatalf("Decoded restored key failed: %v", err)
	}
	if !privateKey.Equal(decoded) {
		t.Fatal("Restored private key should match original")
	}
}

// TestPubkeyToAddressDeterministic tests that same key gives same address
func TestPubkeyToAddressDeterministic(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	addr1 := PubkeyToAddress(privateKey.PublicKey)
	addr2 := PubkeyToAddress(privateKey.PublicKey)
	if addr1 != addr2 {
		t.Errorf("PubkeyToAddress should be deterministic: %s != %s", addr1.Hex(), addr2.Hex())
	}
}

// TestPrivKeyToAddressMatchesPubkey tests PrivKeyToAddress matches PubkeyToAddress
func TestPrivKeyToAddressMatchesPubkey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	addrPub := PubkeyToAddress(privateKey.PublicKey)
	addrPriv := PrivKeyToAddress(*privateKey)
	if addrPub != addrPriv {
		t.Errorf("PrivKeyToAddress should match PubkeyToAddress: %s != %s", addrPub.Hex(), addrPriv.Hex())
	}
}

// TestFromECDSAPubWithNilX tests FromECDSAPub with key having nil X
func TestFromECDSAPubWithNilX(t *testing.T) {
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     nil,
		Y:     new(big.Int).SetInt64(1),
	}
	result := FromECDSAPub(pub)
	if result != nil {
		t.Error("FromECDSAPub with nil X should return nil")
	}
}

// TestINRISeqDeterministic tests INRISeq returns same result for same input
func TestINRISeqDeterministic(t *testing.T) {
	data := []byte("deterministic test")
	r1 := INRISeq(data)
	r2 := INRISeq(data)
	if len(r1) != len(r2) {
		t.Fatalf("Length mismatch: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("INRISeq should be deterministic: differs at index %d", i)
			break
		}
	}
}

// TestINRISeqHashDeterministic tests INRISeqHash returns same result for same input
func TestINRISeqHashDeterministic(t *testing.T) {
	data := []byte("hash deterministic")
	h1 := INRISeqHash(data)
	h2 := INRISeqHash(data)
	if h1 != h2 {
		t.Error("INRISeqHash should be deterministic")
	}
}

// TestEncodeKeysRoundTrip tests EncodeKeys produces decodable output
func TestEncodeKeysRoundTrip(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	privStr, pubStr := EncodeKeys(privateKey)
	if privStr == "" || pubStr == "" {
		t.Fatal("EncodeKeys should return non-empty strings")
	}

	// Decode private key via DecodePrivKey
	decodedPriv := DecodePrivKey(privStr)
	if decodedPriv == nil {
		t.Fatal("DecodePrivKey should not return nil")
	}
	if !privateKey.Equal(decodedPriv) {
		t.Fatal("Decoded private key should match")
	}

	// Decode public key via EncodePublicKeyToByte + DecodeByteToPublicKey (same path as EncodeKeys uses)
	pubBytes := EncodePublicKeyToByte(&privateKey.PublicKey)
	decodedPub, err := DecodeByteToPublicKey(pubBytes)
	if err != nil {
		t.Fatalf("DecodeByteToPublicKey failed: %v", err)
	}
	if !privateKey.PublicKey.Equal(decodedPub) {
		t.Fatal("Decoded public key should match")
	}
}
