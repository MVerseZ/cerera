package types

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"testing"
)

func testBase(t *testing.T) {
	var a, _ = Decode("test")
	fmt.Println(a)
}

// TestHexutilDecode tests the Decode function
func TestHexutilDecode(t *testing.T) {
	// Test valid hex string
	validHex := "0x48656c6c6f"
	result, err := Decode(validHex)
	if err != nil {
		t.Fatalf("Decode failed on valid hex: %v", err)
	}
	expected := []byte("Hello")
	if string(result) != string(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}

	// Test empty string
	_, err = Decode("")
	if err != ErrEmptyString {
		t.Errorf("Expected ErrEmptyString, got %v", err)
	}

	// Test without 0x prefix
	_, err = Decode("48656c6c6f")
	if err != ErrMissingPrefix {
		t.Errorf("Expected ErrMissingPrefix, got %v", err)
	}

	// Test odd length
	_, err = Decode("0x48656c6c6")
	if err != ErrOddLength {
		t.Errorf("Expected ErrOddLength, got %v", err)
	}

	// Test invalid hex
	_, err = Decode("0xGH")
	if err != ErrSyntax {
		t.Errorf("Expected ErrSyntax, got %v", err)
	}
}

// TestMustDecode tests the MustDecode function
func TestMustDecode(t *testing.T) {
	validHex := "0x48656c6c6f"
	result := MustDecode(validHex)
	if string(result) != "Hello" {
		t.Errorf("Expected 'Hello', got %v", result)
	}

	// Test that it panics on invalid input
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustDecode should panic on invalid input")
		}
	}()
	MustDecode("invalid")
}

// TestHexutilEncode tests the Encode function
func TestHexutilEncode(t *testing.T) {
	data := []byte("Hello")
	result := Encode(data)
	if result != "0x48656c6c6f" {
		t.Errorf("Expected '0x48656c6c6f', got %s", result)
	}

	// Test empty data
	result = Encode([]byte{})
	if result != "0x" {
		t.Errorf("Expected '0x', got %s", result)
	}
}

// TestDecodeUint64 tests the DecodeUint64 function
func TestDecodeUint64(t *testing.T) {
	// Test valid number
	result, err := DecodeUint64("0xff")
	if err != nil {
		t.Fatalf("DecodeUint64 failed: %v", err)
	}
	if result != 255 {
		t.Errorf("Expected 255, got %d", result)
	}

	// Test empty string
	_, err = DecodeUint64("")
	if err != ErrEmptyString {
		t.Errorf("Expected ErrEmptyString, got %v", err)
	}

	// Test without prefix
	_, err = DecodeUint64("ff")
	if err != ErrMissingPrefix {
		t.Errorf("Expected ErrMissingPrefix, got %v", err)
	}

	// Test empty number
	_, err = DecodeUint64("0x")
	if err != ErrEmptyNumber {
		t.Errorf("Expected ErrEmptyNumber, got %v", err)
	}

	// Test leading zero
	_, err = DecodeUint64("0x0ff")
	if err != ErrLeadingZero {
		t.Errorf("Expected ErrLeadingZero, got %v", err)
	}
}

// TestMustDecodeUint64 tests the MustDecodeUint64 function
func TestMustDecodeUint64(t *testing.T) {
	result := MustDecodeUint64("0xff")
	if result != 255 {
		t.Errorf("Expected 255, got %d", result)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustDecodeUint64 should panic on invalid input")
		}
	}()
	MustDecodeUint64("invalid")
}

// TestEncodeUint64 tests the EncodeUint64 function
func TestEncodeUint64(t *testing.T) {
	result := EncodeUint64(255)
	if result != "0xff" {
		t.Errorf("Expected '0xff', got %s", result)
	}

	result = EncodeUint64(0)
	if result != "0x0" {
		t.Errorf("Expected '0x0', got %s", result)
	}
}

// TestHexutilDecodeBig tests the DecodeBig function
func TestHexutilDecodeBig(t *testing.T) {
	// Test valid number
	result, err := DecodeBig("0xff")
	if err != nil {
		t.Fatalf("DecodeBig failed: %v", err)
	}
	if result.Cmp(big.NewInt(255)) != 0 {
		t.Errorf("Expected 255, got %v", result)
	}

	// Test large number
	result, err = DecodeBig("0xffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatalf("DecodeBig failed: %v", err)
	}

	// Test empty string
	_, err = DecodeBig("")
	if err != ErrEmptyString {
		t.Errorf("Expected ErrEmptyString, got %v", err)
	}

	// Test without prefix
	_, err = DecodeBig("ff")
	if err != ErrMissingPrefix {
		t.Errorf("Expected ErrMissingPrefix, got %v", err)
	}

	// Test empty number
	_, err = DecodeBig("0x")
	if err != ErrEmptyNumber {
		t.Errorf("Expected ErrEmptyNumber, got %v", err)
	}

	// Test invalid hex
	_, err = DecodeBig("0xGH")
	if err != ErrSyntax {
		t.Errorf("Expected ErrSyntax, got %v", err)
	}
}

// TestMustDecodeBig tests the MustDecodeBig function
func TestMustDecodeBig(t *testing.T) {
	result := MustDecodeBig("0xff")
	if result.Cmp(big.NewInt(255)) != 0 {
		t.Errorf("Expected 255, got %v", result)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustDecodeBig should panic on invalid input")
		}
	}()
	MustDecodeBig("invalid")
}

// TestEncodeBig tests the EncodeBig function
func TestEncodeBig(t *testing.T) {
	// Test zero
	result := EncodeBig(big.NewInt(0))
	if result != "0x0" {
		t.Errorf("Expected '0x0', got %s", result)
	}

	// Test positive number
	result = EncodeBig(big.NewInt(255))
	if result != "0xff" {
		t.Errorf("Expected '0xff', got %s", result)
	}

	// Test negative number
	result = EncodeBig(big.NewInt(-255))
	if result != "-0xff" {
		t.Errorf("Expected '-0xff', got %s", result)
	}
}

// TestCheckNumber tests the checkNumber function
func TestCheckNumber(t *testing.T) {
	// Test valid number
	result, err := checkNumber("0xff")
	if err != nil {
		t.Fatalf("checkNumber failed: %v", err)
	}
	if result != "ff" {
		t.Errorf("Expected 'ff', got %s", result)
	}

	// Test empty string
	_, err = checkNumber("")
	if err != ErrEmptyString {
		t.Errorf("Expected ErrEmptyString, got %v", err)
	}

	// Test without prefix
	_, err = checkNumber("ff")
	if err != ErrMissingPrefix {
		t.Errorf("Expected ErrMissingPrefix, got %v", err)
	}

	// Test empty number
	_, err = checkNumber("0x")
	if err != ErrEmptyNumber {
		t.Errorf("Expected ErrEmptyNumber, got %v", err)
	}

	// Test leading zero
	_, err = checkNumber("0x0ff")
	if err != ErrLeadingZero {
		t.Errorf("Expected ErrLeadingZero, got %v", err)
	}

	// Test single zero (allowed)
	result, err = checkNumber("0x0")
	if err != nil {
		t.Errorf("Single zero should be allowed: %v", err)
	}
	if result != "0" {
		t.Errorf("Expected '0', got %s", result)
	}
}

// TestDecodeNibble tests the decodeNibble function
func TestDecodeNibble(t *testing.T) {
	testCases := []struct {
		input    byte
		expected uint64
	}{
		{'0', 0},
		{'5', 5},
		{'9', 9},
		{'A', 10},
		{'F', 15},
		{'a', 10},
		{'f', 15},
	}

	for _, tc := range testCases {
		result := decodeNibble(tc.input)
		if result != tc.expected {
			t.Errorf("decodeNibble('%c') expected %d, got %d", tc.input, tc.expected, result)
		}
	}

	// Test invalid character
	result := decodeNibble('G')
	if result != badNibble {
		t.Errorf("Expected badNibble, got %d", result)
	}
}

// TestMapError tests the mapError function
func TestMapError(t *testing.T) {
	// Test ErrRange
	numErr := &strconv.NumError{
		Func: "ParseUint",
		Num:  "ff",
		Err:  strconv.ErrRange,
	}
	result := mapError(numErr)
	if result != ErrUint64Range {
		t.Errorf("Expected ErrUint64Range, got %v", result)
	}

	// Test ErrSyntax
	numErr.Err = strconv.ErrSyntax
	result = mapError(numErr)
	if result != ErrSyntax {
		t.Errorf("Expected ErrSyntax, got %v", result)
	}

	// Test InvalidByteError
	invalidByteErr := hex.InvalidByteError(0)
	result = mapError(invalidByteErr)
	if result != ErrSyntax {
		t.Errorf("Expected ErrSyntax, got %v", result)
	}

	// Test ErrLength
	result = mapError(hex.ErrLength)
	if result != ErrOddLength {
		t.Errorf("Expected ErrOddLength, got %v", result)
	}

	// Test other error (generic error)
	otherErr := fmt.Errorf("some error")
	result = mapError(otherErr)
	if result != otherErr {
		t.Errorf("Expected original error, got %v", result)
	}
}
