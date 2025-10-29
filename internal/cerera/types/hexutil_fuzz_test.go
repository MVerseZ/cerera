package types

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// FuzzDecodeNibble tests the decodeNibble function with fuzzing
func FuzzDecodeNibble(f *testing.F) {
	// Add seed values
	f.Add(byte('0'))
	f.Add(byte('9'))
	f.Add(byte('a'))
	f.Add(byte('f'))
	f.Add(byte('A'))
	f.Add(byte('F'))
	f.Add(byte('G'))
	f.Add(byte('@'))

	f.Fuzz(func(t *testing.T, b byte) {
		result := decodeNibble(b)

		// Valid nibbles should be 0-15
		if result != badNibble {
			if result > 15 {
				t.Errorf("decodeNibble returned invalid value: %d", result)
			}
		}

		// All valid hex characters should decode correctly
		switch {
		case b >= '0' && b <= '9':
			expected := uint64(b - '0')
			if result != expected {
				t.Errorf("Expected %d for %c, got %d", expected, b, result)
			}
		case b >= 'a' && b <= 'f':
			expected := uint64(b - 'a' + 10)
			if result != expected {
				t.Errorf("Expected %d for %c, got %d", expected, b, result)
			}
		case b >= 'A' && b <= 'F':
			expected := uint64(b - 'A' + 10)
			if result != expected {
				t.Errorf("Expected %d for %c, got %d", expected, b, result)
			}
		default:
			// Invalid characters should return badNibble
			if result != badNibble {
				t.Errorf("Expected badNibble for invalid char %c, got %d", b, result)
			}
		}
	})
}

// FuzzCheckNumber tests the checkNumber function with various inputs
func FuzzCheckNumber(f *testing.F) {
	// Add seed values
	f.Add("0x123")
	f.Add("0x0")
	f.Add("0xabcdef")
	f.Add("0xff")
	f.Add("")
	f.Add("123")
	f.Add("0x")
	f.Add("0x01")

	f.Fuzz(func(t *testing.T, input string) {
		result, err := checkNumber(input)

		// Empty string should return error
		if len(input) == 0 {
			if err == nil {
				t.Errorf("Expected error for empty string")
			}
			return
		}

		// Must start with 0x
		if !strings.HasPrefix(input, "0x") {
			if err == nil {
				t.Errorf("Expected error for string without 0x prefix")
			}
			return
		}

		// 0x alone should return error
		if input == "0x" {
			if err == nil {
				t.Errorf("Expected error for '0x' alone")
			}
			return
		}

		// Valid cases
		if err == nil {
			if len(result) == 0 && len(input) > 2 {
				t.Errorf("checkNumber should return non-empty result for valid input")
			}
		}
	})
}

// FuzzMapError tests the mapError function with various error types
func FuzzMapError(f *testing.F) {
	// Add seed values
	f.Add("range")
	f.Add("syntax")
	f.Add("invalid")
	f.Add("length")
	f.Add("other")

	f.Fuzz(func(t *testing.T, errType string) {
		var err error
		switch errType {
		case "range":
			err = &strconv.NumError{
				Func: "ParseUint",
				Num:  "999999999999999999999",
				Err:  strconv.ErrRange,
			}
		case "syntax":
			err = &strconv.NumError{
				Func: "ParseUint",
				Num:  "abc",
				Err:  strconv.ErrSyntax,
			}
		case "invalid":
			err = hex.InvalidByteError(0)
		case "length":
			err = hex.ErrLength
		default:
			err = fmt.Errorf("generic error")
		}

		result := mapError(err)
		if result == nil && err != nil {
			t.Errorf("mapError should not return nil for non-nil error")
		}
	})
}

// FuzzEncode tests the Encode function with fuzzing
func FuzzEncode(f *testing.F) {
	// Add seed values
	f.Add([]byte("hello"))
	f.Add([]byte{})
	f.Add([]byte{0x00, 0xff, 0xab})
	f.Add(make([]byte, 1000))

	f.Fuzz(func(t *testing.T, data []byte) {
		result := Encode(data)

		// Result should start with 0x
		if !strings.HasPrefix(result, "0x") {
			t.Errorf("Encode result should start with 0x")
		}

		// Result without 0x should have even length (2 hex chars per byte)
		hexPart := result[2:]
		if len(hexPart)%2 != 0 && len(data) > 0 {
			t.Errorf("Hex part should have even length")
		}

		// Length check
		expectedLen := 2 + len(data)*2
		if len(result) != expectedLen {
			t.Errorf("Expected length %d, got %d", expectedLen, len(result))
		}
	})
}

// FuzzDecodeString tests various strings for decoding
func FuzzDecodeString(f *testing.F) {
	// Add seed values
	f.Add("0x48656c6c6f")
	f.Add("0x")
	f.Add("0x01")
	f.Add("0xff")
	f.Add("48656c6c6f") // without 0x
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		result, err := Decode(input)

		// Empty string should return error
		if len(input) == 0 {
			if err == nil {
				t.Errorf("Expected error for empty string")
			}
			return
		}

		// Without 0x prefix should error
		if !strings.HasPrefix(input, "0x") {
			if err == nil {
				t.Errorf("Expected error for string without 0x")
			}
			return
		}

		// Valid decode
		if err == nil {
			// If decoded successfully, encoding back should work
			encoded := Encode(result)
			if !strings.HasPrefix(encoded, "0x") {
				t.Errorf("Encoded result should have 0x prefix")
			}
		}
	})
}
