package address

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestHexToAddress(t *testing.T) {
	tests := []struct {
		name     string
		hex      string
		expected string
	}{
		{"valid with 0x", "0x0000000000000000000000000000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000000000000000000000000000001"},
		{"valid without 0x", "0000000000000000000000000000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000000000000000000000000000001"},
		{"empty", "0x", "0x0000000000000000000000000000000000000000000000000000000000000000"},
		{"short hex", "0x01", "0x0000000000000000000000000000000000000000000000000000000000000001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := HexToAddress(tt.hex)
			got := addr.Hex()
			if got != tt.expected {
				t.Errorf("HexToAddress(%q) = %q, want %q", tt.hex, got, tt.expected)
			}
		})
	}
}

func TestBytesToAddress(t *testing.T) {
	tests := []struct {
		name     string
		bytes    []byte
		expected []byte
	}{
		{"32 bytes", make([]byte, 32), make([]byte, 32)},
		{"short bytes", []byte{0x01, 0x02, 0x03}, append(make([]byte, 29), 0x01, 0x02, 0x03)},
		{"long bytes", append(make([]byte, 10), make([]byte, 40)...), make([]byte, 32)}, // last 32 taken
		{"nil", nil, make([]byte, 32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := BytesToAddress(tt.bytes)
			got := addr.Bytes()
			if len(got) != AddressLength {
				t.Errorf("BytesToAddress: got len %d, want %d", len(got), AddressLength)
			}
			if tt.expected != nil && len(tt.expected) == 32 {
				for i := range tt.expected {
					if got[i] != tt.expected[i] {
						t.Errorf("BytesToAddress: at index %d got 0x%02x, want 0x%02x", i, got[i], tt.expected[i])
						break
					}
				}
			}
		})
	}
}

func TestAddress_SetBytes(t *testing.T) {
	var addr Address
	addr.SetBytes([]byte{0xab, 0xcd, 0xef})
	b := addr.Bytes()
	// Should be right-aligned
	if b[31] != 0xef || b[30] != 0xcd || b[29] != 0xab {
		t.Errorf("SetBytes: got %x, expected right-aligned abcdef", b)
	}
}

func TestAddress_Bytes(t *testing.T) {
	addr := BytesToAddress([]byte{0x01, 0x02, 0x03})
	b := addr.Bytes()
	if len(b) != AddressLength {
		t.Errorf("Bytes() len = %d, want %d", len(b), AddressLength)
	}
	if b[31] != 0x03 || b[30] != 0x02 || b[29] != 0x01 {
		t.Errorf("Bytes() = %x, expected ...010203", b)
	}
}

func TestEmptyAddress(t *testing.T) {
	addr := EmptyAddress()
	if !addr.IsEmpty() {
		t.Error("EmptyAddress() should return empty address")
	}
}

func TestAddress_Hex(t *testing.T) {
	addr := HexToAddress("0x00000000000000000000000000000000000000000000000000000000000000ff")
	hex := addr.Hex()
	if hex[:2] != "0x" {
		t.Errorf("Hex() should start with 0x, got %s", hex)
	}
	if len(hex) != 2+AddressLength*2 {
		t.Errorf("Hex() len = %d, want %d", len(hex), 2+AddressLength*2)
	}
}

func TestAddress_String(t *testing.T) {
	addr := HexToAddress("0x01")
	s := addr.String()
	if s != addr.Hex() {
		t.Errorf("String() = %s, Hex() = %s", s, addr.Hex())
	}
}

func TestAddress_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		addr   Address
		empty  bool
	}{
		{"zero address", Address{}, true},
		{"empty from bytes", BytesToAddress([]byte{}), true},
		{"non-empty", BytesToAddress([]byte{0x01}), false},
		{"EmptyAddress()", EmptyAddress(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.addr.IsEmpty(); got != tt.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.empty)
			}
		})
	}
}

func TestAddress_MarshalText(t *testing.T) {
	addr := HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	text, err := addr.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if len(text) == 0 {
		t.Error("MarshalText() returned empty")
	}
	if text[0] != '0' || text[1] != 'x' {
		t.Errorf("MarshalText() should start with 0x, got %s", string(text))
	}
}

func TestAddress_UnmarshalText(t *testing.T) {
	hexStr := "0x0000000000000000000000000000000000000000000000000000000000000001"
	var addr Address
	err := addr.UnmarshalText([]byte(hexStr))
	if err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if addr.Hex() != hexStr {
		t.Errorf("UnmarshalText() got %s, want %s", addr.Hex(), hexStr)
	}
}

func TestAddress_UnmarshalJSON(t *testing.T) {
	jsonStr := `"0x0000000000000000000000000000000000000000000000000000000000000001"`
	var addr Address
	err := json.Unmarshal([]byte(jsonStr), &addr)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	expected := HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	if addr != expected {
		t.Errorf("UnmarshalJSON() got %s, want %s", addr.Hex(), expected.Hex())
	}
}

func TestAddress_Format(t *testing.T) {
	addr := HexToAddress("0x01")
	tests := []struct {
		verb string
	}{
		{"%v"}, {"%s"}, {"%q"}, {"%x"}, {"%X"}, {"%d"},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			s := fmt.Sprintf(tt.verb, addr)
			if len(s) == 0 {
				t.Errorf("Format(%s) returned empty", tt.verb)
			}
		})
	}
}

func TestIsHexAddress(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid with 0x", "0x" + strings.Repeat("00", AddressLength), true},
		{"valid without 0x", strings.Repeat("00", AddressLength), true},
		{"invalid length", "0x01", false},
		{"invalid chars", "0x" + strings.Repeat("00", 31) + "zz", false},
		{"odd length", "0x" + strings.Repeat("0", AddressLength*2-1), false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHexAddress(tt.input); got != tt.valid {
				t.Errorf("IsHexAddress(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestFromHex(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
	}{
		{"0x01", []byte{0x01}},
		{"01", []byte{0x01}},
		{"0x", []byte{}},
		{"0x0102", []byte{0x01, 0x02}},
		{"1", []byte{0x01}}, // odd length padded
	}
	for _, tt := range tests {
		got := FromHex(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("FromHex(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
		}
		for i := range tt.want {
			if i < len(got) && got[i] != tt.want[i] {
				t.Errorf("FromHex(%q)[%d] = 0x%02x, want 0x%02x", tt.input, i, got[i], tt.want[i])
				break
			}
		}
	}
}

func TestHex2Bytes(t *testing.T) {
	got := Hex2Bytes("0102")
	if len(got) != 2 || got[0] != 0x01 || got[1] != 0x02 {
		t.Errorf("Hex2Bytes(0102) = %x, want 0102", got)
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		srv  byte
		want string
	}{
		{1, "RPC_SERVICE"},
		{2, "TX_POOL_SERVICE"},
		{0, "UNKNOWN_SERVICE"},
		{3, "UNKNOWN_SERVICE"},
		{255, "UNKNOWN_SERVICE"},
	}
	for _, tt := range tests {
		if got := GetServiceName(tt.srv); got != tt.want {
			t.Errorf("GetServiceName(%d) = %q, want %q", tt.srv, got, tt.want)
		}
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantErr bool
	}{
		{"float64", float64(3.14), 3.14, false},
		{"float32", float32(2.5), 2.5, false},
		{"int64", int64(100), 100, false},
		{"int", 42, 42, false},
		{"uint64", uint64(999), 999, false},
		{"string", "1.5", 1.5, false},
		{"big.Int", new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), 1.0, false}, // 1e18 wei = 1 CER
		{"zero", float64(0), 0, false},
		{"invalid", struct{}{}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetFloat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				diff := got - tt.want
				if diff < 0 {
					diff = -diff
				}
				if diff > 0.0001 {
					t.Errorf("GetFloat() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	// Non-existent file
	if FileExists("/nonexistent/path/12345") {
		t.Error("FileExists(nonexistent) should return false")
	}
	// Current directory exists
	if !FileExists(".") {
		t.Error("FileExists(.) should return true")
	}
	// Create temp file
	f, err := os.CreateTemp("", "address_test_*")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	defer os.Remove(f.Name())
	f.Close()
	if !FileExists(f.Name()) {
		t.Error("FileExists(temp) should return true")
	}
}

func TestIntToBytes(t *testing.T) {
	tests := []struct {
		num  int
		want []byte
	}{
		{0, []byte{0, 0, 0, 0}},
		{1, []byte{0, 0, 0, 1}},
		{256, []byte{0, 0, 1, 0}},
		{65536, []byte{0, 1, 0, 0}},
		{16777216, []byte{1, 0, 0, 0}},
	}
	for _, tt := range tests {
		got := IntToBytes(tt.num)
		if len(got) != 4 {
			t.Errorf("IntToBytes(%d) len = %d, want 4", tt.num, len(got))
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("IntToBytes(%d) = %v, want %v", tt.num, got, tt.want)
				break
			}
		}
	}
}

func TestIsFileNotEmpty(t *testing.T) {
	// Non-existent
	_, err := IsFileNotEmpty("/nonexistent/path/12345")
	if err == nil {
		t.Error("IsFileNotEmpty(nonexistent) should return error")
	}

	// Empty file
	f, err := os.CreateTemp("", "address_test_empty_*")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	emptyPath := f.Name()
	f.Close()
	defer os.Remove(emptyPath)

	notEmpty, err := IsFileNotEmpty(emptyPath)
	if err != nil {
		t.Fatalf("IsFileNotEmpty(empty) error = %v", err)
	}
	if notEmpty {
		t.Error("IsFileNotEmpty(empty file) should return false")
	}

	// Non-empty file
	f2, err := os.CreateTemp("", "address_test_full_*")
	if err != nil {
		t.Skip("Cannot create temp file:", err)
	}
	f2.Write([]byte("test"))
	f2.Close()
	defer os.Remove(f2.Name())

	notEmpty, err = IsFileNotEmpty(f2.Name())
	if err != nil {
		t.Fatalf("IsFileNotEmpty(non-empty) error = %v", err)
	}
	if !notEmpty {
		t.Error("IsFileNotEmpty(non-empty file) should return true")
	}
}

func TestAddress_RoundTrip(t *testing.T) {
	original := HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	bytes := original.Bytes()
	restored := BytesToAddress(bytes)
	if original != restored {
		t.Errorf("RoundTrip: %s != %s", original.Hex(), restored.Hex())
	}
}

func TestMaxAddress(t *testing.T) {
	if MaxAddress.IsEmpty() {
		t.Error("MaxAddress should not be empty")
	}
	hex := MaxAddress.Hex()
	if len(hex) != 2+AddressLength*2 {
		t.Errorf("MaxAddress hex len = %d", len(hex))
	}
}
