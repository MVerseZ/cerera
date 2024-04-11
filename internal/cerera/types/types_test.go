package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"

	"github.com/cerera/internal/cerera/common"
)

func TestHexToAddress(t *testing.T) {
	pk, _ := GenerateAccount()
	currentNodeAddress := PubkeyToAddress(pk.PublicKey)
	var addrStr = currentNodeAddress.Hex()
	var resultAddr = HexToAddress(addrStr)
	if resultAddr != currentNodeAddress {
		t.Errorf("Different addresses: given \r\n%d\r\n, expected \r\n%d\r\n", resultAddr, currentNodeAddress)
		t.Errorf("Different addresses: given \r\n%s\r\n, expected \r\n%s\r\n", resultAddr, currentNodeAddress)
		t.Errorf("Different addresses: given \r\n%x\r\n, expected \r\n%x\r\n", resultAddr, currentNodeAddress)
	}
}

func TestBytesToAddress(t *testing.T) {
	pk, _ := GenerateAccount()
	var currentNodeAddress = PubkeyToAddress(pk.PublicKey)
	var addrBytes = currentNodeAddress.Bytes()
	var resultAddr = BytesToAddress(addrBytes)
	if resultAddr != currentNodeAddress {
		t.Errorf("Different addresses: given %s, expected %s", resultAddr, currentNodeAddress)
	}
}

func TestBytesConversion(t *testing.T) {
	bytes := []byte{5}
	hash := common.BytesToHash(bytes)

	var exp common.Hash

	exp[31] = 5

	if hash != exp {
		t.Errorf("expected %x got %x", exp, hash)
	}
}

func TestIsHexAddress(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		{"0x9642903868c35526a8b34686a5d4184125562e7300a4556790379884e81ee30507352e49266351711164874923761a17", true},
		{"0x9642903868c35526a8b34686a5d4184125562e7300a4556790379884e81ee30507352e49266351711164874923761a17", true},
		{"0x9642903868c35526a8b34686a5d4184125562e7300a4556790379884e81ee30507352e49266351711164874923761a17", true},
		{"0x9642903868c35526a8b34686a5d4184125562e7300a4556790379884e81ee30507352e49266351711164874923761a17", true},
		{"0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", true},
		{"0x000000000000000000000000000000000000000000000000000000000005aaeb6053f3e94c9b9a09f3435e7ef1beaed1", true},
		{"0x5aaeb6053f3e94c9b9a095aaeb6053f3e94c9b9a095aaeb6053f3e94c9b9a095aaeb6053f3e94c9b9f3435e7ef1beae", false},
		{"5aaeb6053f3e94c9b9a09f345aaeb6053f3e94c9b9a0900035aaeb6053f3e94c9b9a095e7ef1beaed11", false},
		{"0xxaaeb6053f3e94c9b9a05aaeb6053f3e94c9b9a095aaeb6053f3e94c9b9a099f3435e7ef1beaed", false},
	}

	for _, test := range tests {
		if result := IsHexAddress(test.str); result != test.exp {
			t.Errorf("IsHexAddress(%s) == %v; expected %v",
				test.str, result, test.exp)
		}
	}
}

func TestAddressUnmarshalJSON(t *testing.T) {
	var tests = []struct {
		Input     string
		ShouldErr bool
		Output    *big.Int
	}{
		{"", true, nil},
		{`""`, true, nil},
		{`"0x"`, true, nil},
		{`"0x00"`, true, nil},
		{`"0xG000000000000000000000000000000000000000"`, true, nil},
		{`"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"`, false, big.NewInt(0)},
	}
	for i, test := range tests {
		var v Address
		err := json.Unmarshal([]byte(test.Input), &v)
		if err != nil && !test.ShouldErr {
			t.Errorf("test #%d: unexpected error: %v", i, err)
		}
		if err == nil {
			if test.ShouldErr {
				t.Errorf("test #%d: expected error, got none", i)
			}
			if got := new(big.Int).SetBytes(v.Bytes()); got.Cmp(test.Output) != 0 {
				fmt.Println(v.Hex())

				t.Errorf("test #%d: address mismatch: have %v, want %v", i, got, test.Output)
			}
		}
	}
}

// func TestMarshallTextAddress(t *testing.T) {
// 	pk, _ := GenerateAccount()
// 	currentNodeAddress := PubkeyToAddress(pk.PublicKey)
// 	var addrMarshal, _ = currentNodeAddress.MarshalText()
// 	err := Address.UnmarshalText(Address(addrMarshal))

// 	if resultAddr != currentNodeAddress {
// 		t.Errorf("Different addresses: given \r\n%d\r\n, expected \r\n%d\r\n", resultAddr, currentNodeAddress)
// 		t.Errorf("Different addresses: given \r\n%s\r\n, expected \r\n%s\r\n", resultAddr, currentNodeAddress)
// 		t.Errorf("Different addresses: given \r\n%x\r\n, expected \r\n%x\r\n", resultAddr, currentNodeAddress)
// 	}
// }

// TODO fix this later - write own parser or use smth lib
// func TestHashJsonValidation(t *testing.T) {
// 	var tests = []struct {
// 		Prefix string
// 		Size   int
// 		Error  string
// 	}{
// 		{"", 62, "json: cannot unmarshal hex string without 0x prefix into Go value of type types.Hash"},
// 		{"0x", 66, "hex string has length 66, want 64 for types.Hash"},
// 		{"0x", 63, "json: cannot unmarshal hex string of odd length into Go value of type types.Hash"},
// 		{"0x", 0, "hex string has length 0, want 64 for types.Hash"},
// 		{"0x", 64, ""},
// 		{"0X", 64, ""},
// 	}
// 	for _, test := range tests {
// 		input := `"` + test.Prefix + strings.Repeat("0", test.Size) + `"`
// 		var v common.Hash
// 		err := json.Unmarshal([]byte(input), &v)
// 		if err == nil {
// 			if test.Error != "" {
// 				t.Errorf("%s: error mismatch: have nil, want %q", input, test.Error)
// 			}
// 		} else {
// 			if err.Error() != test.Error {
// 				t.Errorf("%s: error mismatch: have %q, want %q", input, err, test.Error)
// 			}
// 		}
// 	}
// }

func TestAddressHexChecksum(t *testing.T) {
	var tests = []struct {
		Input  string
		Output string
	}{

		{"0xb6053F3E94C9B9A09F33669435E7EF1bEAed", "0x000000000000000000000000000000000000000000000000000000000000B6053f3E94c9B9A09f33669435e7ef1BEaEd"},     //"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"},
		{"0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359", "0x00000000000000000000000000000000000000000000000000000000FB6916095ca1dF60Bb79ce92Ce3EA74c37c5D359"}, // "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"},
		{"0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb", "0x00000000000000000000000000000000000000000000000000000000DBf03B407c01e7cD3cBea99509D93F8dddc8C6Fb"}, //"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB"},
		{"0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb", "0x00000000000000000000000000000000000000000000000000000000D1220A0CF47C7B9BE7A2e6ba89F429762e7B9aDb"}, // "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb"},
		// Ensure that non-standard length input values are handled correctly
		{"0x1111111111111111111111111111111111111111", "0x000000000000000000000000000000000000000000000000000000001111111111111111111111111111111111111111"},
		{"0x2222222222222222222222222222222222222234", "0x000000000000000000000000000000000000000000000000000000002222222222222222222222222222222222222234"},
		{"0xa", "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"},
		{"0x0a", "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"},
		{"0x00a", "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"},
		{"0x00000000000000000000000000000000000a", "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a"},
	}
	for i, test := range tests {
		output := HexToAddress(test.Input).Hex()
		if output != test.Output {
			t.Errorf("test #%d: failed to match when it should (%s != %s)", i, output, test.Output)
		}
	}
}

func BenchmarkAddressHex(b *testing.B) {
	testAddr := HexToAddress("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed")
	for n := 0; n < b.N; n++ {
		testAddr.Hex()
	}
}

func TestGeneratingAddress(t *testing.T) {
	for n := 0; n < 50; n++ {
		pk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		// addr := PrivKeyToAddress(*pk)

		// file, _ := os.Create("PK_" + strconv.Itoa(n))
		// defer file.Close()

		pkbts, _ := x509.MarshalECPrivateKey(pk)
		bts := pem.EncodeToMemory(&pem.Block{
			Type:  "EC PK",
			Bytes: pkbts,
		})
		if len(bts) < 0 {
			t.Error("Wrong key size")
		}
		// _, err := file.Write(bts)
		// if err != nil {
		// 	panic(err)
		// }
	}
}

func TestDecode(t *testing.T) {
	var tests = []struct {
		Input  string
		Output string
	}{

		// {"",
		// 	""},
		// {"b6053F3E94C9B9A09F33669435E7EF1bEAed", ""},
		{"0xb6053F3E94C9B9A09F33669435E7EF1bEAed",
			"0xb6053f3e94c9b9a09f33669435e7ef1beaed"},
		{"0xb605ffffffffffffffffffffffffffffffff",
			"0xb605ffffffffffffffffffffffffffffffff"},
	}
	for i, test := range tests {
		output, err := Decode(test.Input)
		if err != nil {
			t.Errorf("test #%d: Exception: %s, failed to match when it should (%s != %s)", i, err, output, test.Output)
		}
		out := Encode(output)

		if test.Output != out {
			t.Errorf("test #%d: failed to match when it should (%s != %s)", i, out, test.Output)
		}
	}
}

func TestDecodeBig(t *testing.T) {
	var tests = []struct {
		Input     string
		ShouldErr bool
		Output    *big.Int
	}{
		{"", true, nil},
		{"0x", true, nil},
		{"0x0x", true, nil},
		{`""`, true, nil},
		{`"0x"`, true, nil},
		{`"0x00"`, true, nil},
		{`"232323333ffs33333333BF239232"`, true, nil},
		{`"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"`, false, big.NewInt(0)},
		{`"0xf0000000000000000000000000000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"`, false, big.NewInt(0)},
	}
	for i, test := range tests {
		out, err := DecodeBig(test.Input)
		if err == nil {
			if test.ShouldErr {
				t.Errorf("test #%d: expected error, got none", i)
			}
			if got := new(big.Int).SetBytes(out.Bytes()); got.Cmp(test.Output) != 0 {

				t.Errorf("test #%d: address mismatch: have %v, want %v", i, got, test.Output)
			}
		}
	}
}
