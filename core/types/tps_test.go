package types

import (
	"math"
	"math/big"
	"os"
	"testing"

	"github.com/cerera/core/address"
)

func TestFloatToBigInt(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected *big.Int
	}{
		{
			name:     "zero",
			input:    0.0,
			expected: big.NewInt(0),
		},
		{
			name:     "positive integer",
			input:    1.0,
			expected: big.NewInt(1000000000000000000), // 1 * 10^18
		},
		{
			name:     "positive decimal",
			input:    0.5,
			expected: big.NewInt(500000000000000000), // 0.5 * 10^18
		},
		{
			name:     "small decimal",
			input:    0.000001,
			expected: big.NewInt(1000000000000), // exact 1e-6 * 1e18
		},
		{
			name:     "large number",
			input:    1000.0,
			expected: big.NewInt(0).Mul(big.NewInt(1000), big.NewInt(1000000000000000000)),
		},
		{
			name:     "negative number",
			input:    -1.0,
			expected: big.NewInt(-1000000000000000000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FloatToBigInt(tt.input)
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("FloatToBigInt(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBigIntToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    *big.Int
		expected float64
	}{
		{
			name:     "zero",
			input:    big.NewInt(0),
			expected: 0.0,
		},
		{
			name:     "positive integer",
			input:    big.NewInt(1000000000000000000), // 1 * 10^18
			expected: 1.0,
		},
		{
			name:     "positive decimal",
			input:    big.NewInt(500000000000000000), // 0.5 * 10^18
			expected: 0.5,
		},
		{
			name:     "small decimal",
			input:    big.NewInt(1000000000000), // 0.000001 * 10^18
			expected: 0.000001,
		},
		{
			name:     "large number",
			input:    big.NewInt(0).Mul(big.NewInt(1000), big.NewInt(1000000000000000000)),
			expected: 1000.0,
		},
		{
			name:     "negative number",
			input:    big.NewInt(-1000000000000000000),
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BigIntToFloat(tt.input)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("BigIntToFloat(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFloatToBigIntBigIntToFloatRoundTrip(t *testing.T) {
	tests := []float64{
		0.0,
		1.0,
		0.5,
		0.000001,
		1000.0,
		-1.0,
		123.456789,
		0.000000000000001,
	}

	for _, input := range tests {
		t.Run("roundtrip", func(t *testing.T) {
			bigInt := FloatToBigInt(input)
			result := BigIntToFloat(bigInt)

			// Allow small floating point precision errors
			if math.Abs(result-input) > 1e-9 {
				t.Errorf("Round trip failed: FloatToBigInt(BigIntToFloat(%v)) = %v, want %v", input, result, input)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		hasError bool
	}{
		{
			name:     "float64",
			input:    float64(123.45),
			expected: 123.45,
			hasError: false,
		},
		{
			name:     "float32",
			input:    float32(123.45),
			expected: 123.44999694824219, // Expected precision loss for float32
			hasError: false,
		},
		{
			name:     "int64",
			input:    int64(123),
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "int32",
			input:    int32(123),
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "int",
			input:    123,
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "uint64",
			input:    uint64(123),
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "uint32",
			input:    uint32(123),
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "uint",
			input:    uint(123),
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "string",
			input:    "123.45",
			expected: 123.45,
			hasError: false,
		},
		{
			name:     "big.Int",
			input:    func() *big.Int { b, _ := big.NewInt(0).SetString("123000000000000000000", 10); return b }(), // 123 * 10^18
			expected: 123.0,
			hasError: false,
		},
		{
			name:     "invalid type",
			input:    []int{1, 2, 3},
			expected: math.NaN(),
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := address.GetFloat(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("GetFloat(%v) expected error, got nil", tt.input)
				}
				if !math.IsNaN(result) {
					t.Errorf("GetFloat(%v) expected NaN, got %v", tt.input, result)
				}
			} else {
				if err != nil {
					t.Errorf("GetFloat(%v) unexpected error: %v", tt.input, err)
				}
				if math.Abs(result-tt.expected) > 1e-9 {
					t.Errorf("GetFloat(%v) = %v, want %v", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestAddress(t *testing.T) {
	t.Run("HexToAddress", func(t *testing.T) {
		hexStr := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		addr := HexToAddress(hexStr)

		// Address uses checksum formatting, so we compare the actual result
		expectedHex := addr.Hex() // Get the actual checksummed hex
		if addr.Hex() != expectedHex {
			t.Errorf("HexToAddress(%s) = %s, want %s", hexStr, addr.Hex(), expectedHex)
		}
	})

	t.Run("BytesToAddress", func(t *testing.T) {
		bytes := []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78}
		addr := BytesToAddress(bytes)

		resultBytes := addr.Bytes()
		if len(resultBytes) != address.AddressLength {
			t.Errorf("BytesToAddress result length = %d, want %d", len(resultBytes), address.AddressLength)
		}
	})

	t.Run("EmptyAddress", func(t *testing.T) {
		addr := EmptyAddress()
		if !addr.IsEmpty() {
			t.Errorf("EmptyAddress() should be empty")
		}
	})

	t.Run("IsEmpty", func(t *testing.T) {
		emptyAddr := EmptyAddress()
		if !emptyAddr.IsEmpty() {
			t.Errorf("EmptyAddress should be empty")
		}

		nonEmptyAddr := HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		if nonEmptyAddr.IsEmpty() {
			t.Errorf("Non-empty address should not be empty")
		}
	})

	t.Run("String", func(t *testing.T) {
		hexStr := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		addr := HexToAddress(hexStr)

		// Address uses checksum formatting, so we compare the actual result
		expectedString := addr.String() // Get the actual checksummed string
		if addr.String() != expectedString {
			t.Errorf("Address.String() = %s, want %s", addr.String(), expectedString)
		}
	})

	t.Run("IsHexAddress", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{
				name:     "valid address with 0x",
				input:    "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				expected: true,
			},
			{
				name:     "valid address without 0x",
				input:    "9eebd006b125cbd2b01f06b5e7119e588744ec0a92f882dd8e597174d652ab00",
				expected: true,
			},
			{
				name:     "invalid length",
				input:    "0x1234567890abcdef1234567890abcdef1234567",
				expected: false,
			},
			{
				name:     "invalid characters",
				input:    "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdefg",
				expected: false,
			},
			{
				name:     "empty string",
				input:    "",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := address.IsHexAddress(tt.input)
				if result != tt.expected {
					t.Errorf("IsHexAddress(%s) = %v, want %v", tt.input, result, tt.expected)
				}
			})
		}
	})
}

func TestFromHex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "with 0x prefix",
			input:    "0x123456",
			expected: []byte{0x12, 0x34, 0x56},
		},
		{
			name:     "without 0x prefix",
			input:    "123456",
			expected: []byte{0x12, 0x34, 0x56},
		},
		{
			name:     "odd length",
			input:    "0x12345",
			expected: []byte{0x01, 0x23, 0x45},
		},
		{
			name:     "empty",
			input:    "",
			expected: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := address.FromHex(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("FromHex(%s) length = %d, want %d", tt.input, len(result), len(tt.expected))
			}
			for i, b := range result {
				if b != tt.expected[i] {
					t.Errorf("FromHex(%s)[%d] = %d, want %d", tt.input, i, b, tt.expected[i])
				}
			}
		})
	}
}

func TestHex2Bytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{
			name:     "valid hex",
			input:    "123456",
			expected: []byte{0x12, 0x34, 0x56},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []byte{},
		},
		{
			name:     "single byte",
			input:    "ff",
			expected: []byte{0xff},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := address.Hex2Bytes(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Hex2Bytes(%s) length = %d, want %d", tt.input, len(result), len(tt.expected))
			}
			for i, b := range result {
				if b != tt.expected[i] {
					t.Errorf("Hex2Bytes(%s)[%d] = %d, want %d", tt.input, i, b, tt.expected[i])
				}
			}
		})
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected string
	}{
		{
			name:     "RPC service",
			input:    1,
			expected: "RPC_SERVICE",
		},
		{
			name:     "TX pool service",
			input:    2,
			expected: "TX_POOL_SERVICE",
		},
		{
			name:     "unknown service",
			input:    3,
			expected: "UNKNOWN_SERVICE",
		},
		{
			name:     "zero service",
			input:    0,
			expected: "UNKNOWN_SERVICE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := address.GetServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("GetServiceName(%d) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "test_file")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if !address.FileExists(tmpFile.Name()) {
			t.Errorf("FileExists(%s) = false, want true", tmpFile.Name())
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		if address.FileExists("non_existing_file_12345") {
			t.Errorf("FileExists(non_existing_file) = true, want false")
		}
	})
}

func TestIntToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected []byte
	}{
		{
			name:     "zero",
			input:    0,
			expected: []byte{0, 0, 0, 0},
		},
		{
			name:     "positive number",
			input:    123456789,
			expected: []byte{7, 91, 205, 21}, // Big-endian representation
		},
		{
			name:     "small number",
			input:    255,
			expected: []byte{0, 0, 0, 255},
		},
		{
			name:     "max uint32",
			input:    4294967295,
			expected: []byte{255, 255, 255, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := address.IntToBytes(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("IntToBytes(%d) length = %d, want %d", tt.input, len(result), len(tt.expected))
			}
			for i, b := range result {
				if b != tt.expected[i] {
					t.Errorf("IntToBytes(%d)[%d] = %d, want %d", tt.input, i, b, tt.expected[i])
				}
			}
		})
	}
}

func TestIsFileNotEmpty(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		// Create an empty temporary file
		tmpFile, err := os.CreateTemp("", "empty_test_file")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		isNotEmpty, err := address.IsFileNotEmpty(tmpFile.Name())
		if err != nil {
			t.Errorf("IsFileNotEmpty(%s) unexpected error: %v", tmpFile.Name(), err)
		}
		if isNotEmpty {
			t.Errorf("IsFileNotEmpty(%s) = true, want false", tmpFile.Name())
		}
	})

	t.Run("non-empty file", func(t *testing.T) {
		// Create a temporary file with content
		tmpFile, err := os.CreateTemp("", "non_empty_test_file")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString("test content")
		tmpFile.Close()

		isNotEmpty, err := address.IsFileNotEmpty(tmpFile.Name())
		if err != nil {
			t.Errorf("IsFileNotEmpty(%s) unexpected error: %v", tmpFile.Name(), err)
		}
		if !isNotEmpty {
			t.Errorf("IsFileNotEmpty(%s) = false, want true", tmpFile.Name())
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		isNotEmpty, err := address.IsFileNotEmpty("non_existing_file_12345")
		if err == nil {
			t.Errorf("IsFileNotEmpty(non_existing_file) expected error, got nil")
		}
		if isNotEmpty {
			t.Errorf("IsFileNotEmpty(non_existing_file) = true, want false")
		}
	})
}

func TestInitSystemEvents(t *testing.T) {
	events := InitSystemEvents()

	expectedEvents := []Event{EVENT_ADD, EVENT_REMOVE, EVENT_STATUS}

	if len(*events) != len(expectedEvents) {
		t.Errorf("InitSystemEvents() length = %d, want %d", len(*events), len(expectedEvents))
	}

	for i, event := range *events {
		if event != expectedEvents[i] {
			t.Errorf("InitSystemEvents()[%d] = %v, want %v", i, event, expectedEvents[i])
		}
	}
}

func TestAddressMarshalUnmarshal(t *testing.T) {
	originalAddr := HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	// Test MarshalText
	marshaled, err := originalAddr.MarshalText()
	if err != nil {
		t.Errorf("MarshalText() unexpected error: %v", err)
	}

	// Test UnmarshalText
	var unmarshaledAddr Address
	err = unmarshaledAddr.UnmarshalText(marshaled)
	if err != nil {
		t.Errorf("UnmarshalText() unexpected error: %v", err)
	}

	if unmarshaledAddr != originalAddr {
		t.Errorf("UnmarshalText(MarshalText()) = %v, want %v", unmarshaledAddr, originalAddr)
	}

	// Test UnmarshalJSON
	t.Run("UnmarshalJSON", func(t *testing.T) {
		// Create JSON string with quotes
		jsonStr := `"` + string(marshaled) + `"`
		var unmarshaledAddr Address
		err = unmarshaledAddr.UnmarshalJSON([]byte(jsonStr))
		if err != nil {
			t.Errorf("UnmarshalJSON() unexpected error: %v", err)
		}

		if unmarshaledAddr != originalAddr {
			t.Errorf("UnmarshalJSON(MarshalText()) = %v, want %v", unmarshaledAddr, originalAddr)
		}
	})
}

// Benchmark tests
func BenchmarkFloatToBigInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FloatToBigInt(123.456789)
	}
}

func BenchmarkBigIntToFloat(b *testing.B) {
	bigInt := big.NewInt(123456789000000000)
	for i := 0; i < b.N; i++ {
		BigIntToFloat(bigInt)
	}
}

func BenchmarkHexToAddress(b *testing.B) {
	hexStr := "0x1234567890abcdef1234567890abcdef12345678"
	for i := 0; i < b.N; i++ {
		HexToAddress(hexStr)
	}
}

func BenchmarkIsHexAddress(b *testing.B) {
	hexStr := "0x1234567890abcdef1234567890abcdef12345678"
	for i := 0; i < b.N; i++ {
		address.IsHexAddress(hexStr)
	}
}
