package types

import (
	"encoding/json"
	"math/big"
	"reflect"
	"testing"
)

func TestBytesTextAndJSONRoundTrip(t *testing.T) {
	want := Bytes{0x01, 0xab, 0xff}
	text, err := want.MarshalText()
	if err != nil || string(text) != "0x01abff" {
		t.Fatalf("MarshalText() = %q, %v", text, err)
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Bytes
	if err := json.Unmarshal(encoded, &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Bytes round trip = %x, %v; want %x", got, err, want)
	}
	for _, input := range []string{`1`, `"123"`, `"0x0"`, `"0xzz"`} {
		var value Bytes
		if err := json.Unmarshal([]byte(input), &value); err == nil {
			t.Errorf("Bytes accepted %s", input)
		}
	}
}

func TestFixedHexUnmarshalHelpers(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]byte, []byte) error
	}{
		{"prefixed", func(input, out []byte) error { return UnmarshalFixedText("fixed", input, out) }},
		{"unprefixed", func(input, out []byte) error { return UnmarshalFixedUnprefixedText("fixed", input, out) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := make([]byte, 2)
			input := []byte("0x01ab")
			if tc.name == "unprefixed" {
				input = []byte("01ab")
			}
			if err := tc.fn(input, out); err != nil || !reflect.DeepEqual(out, []byte{1, 0xab}) {
				t.Fatalf("decode = %x, %v", out, err)
			}
			for _, bad := range [][]byte{[]byte("zz"), []byte("01"), []byte("010203")} {
				if err := tc.fn(bad, make([]byte, 2)); err == nil {
					t.Errorf("accepted invalid fixed hex %q", bad)
				}
			}
		})
	}
	var value Bytes
	if err := json.Unmarshal([]byte(`"0x0"`), &value); err == nil {
		t.Fatal("odd-length byte string unexpectedly decoded")
	}
}

func TestBigUintAndGraphQLConversions(t *testing.T) {
	var number Big
	if err := json.Unmarshal([]byte(`"0x2a"`), &number); err != nil || number.ToInt().Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("Big unmarshal = %v, %v", number.ToInt(), err)
	}
	if err := json.Unmarshal([]byte(`42`), &number); err == nil {
		t.Fatal("Big accepted a JSON number instead of a string")
	}
	if err := number.UnmarshalGraphQL(int32(7)); err != nil || number.ToInt().Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("Big GraphQL int32 conversion failed: %v", err)
	}
	if err := number.UnmarshalGraphQL(true); err == nil {
		t.Fatal("Big accepted unsupported GraphQL input")
	}
	if !number.ImplementsGraphQLType("BigInt") || number.ImplementsGraphQLType("String") {
		t.Fatal("Big GraphQL type recognition is incorrect")
	}

	var long Uint64
	if err := json.Unmarshal([]byte(`"0x2a"`), &long); err != nil || long != 42 {
		t.Fatalf("Uint64 unmarshal = %d, %v", long, err)
	}
	if err := long.UnmarshalGraphQL(int32(9)); err != nil || long != 9 {
		t.Fatalf("Uint64 GraphQL conversion = %d, %v", long, err)
	}
	if err := long.UnmarshalGraphQL([]byte("bad")); err == nil {
		t.Fatal("Uint64 accepted unsupported GraphQL input")
	}
	if !long.ImplementsGraphQLType("Long") || long.ImplementsGraphQLType("Int") {
		t.Fatal("Uint64 GraphQL type recognition is incorrect")
	}

	var unsigned Uint
	if err := json.Unmarshal([]byte(`"0x2a"`), &unsigned); err != nil || unsigned != 42 {
		t.Fatalf("Uint unmarshal = %d, %v", unsigned, err)
	}
	if err := json.Unmarshal([]byte(`42`), &unsigned); err == nil {
		t.Fatal("Uint accepted a JSON number instead of a string")
	}
}
