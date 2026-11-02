package common

import (
	"math/big"
	"testing"
)

func TestDecimalAndFloatConversion(t *testing.T) {
	dust, err := DecimalStringToDust("1.23")
	if err != nil {
		t.Fatal(err)
	}
	if dust.Cmp(big.NewInt(1_230_000)) != 0 {
		t.Fatalf("dust %s", dust)
	}
	viaAlias, err := DecimalStringToWei("-2")
	if err != nil || viaAlias.Cmp(big.NewInt(-2_000_000)) != 0 {
		t.Fatalf("alias %v %v", viaAlias, err)
	}
	if _, err := DecimalStringToDust("nope"); err == nil {
		t.Fatal("expected bad decimal")
	}
	if _, err := DecimalStringToDust("1.1234567"); err != nil {
		t.Fatal(err)
	}

	if got := FloatToBigInt(1.5); got.Cmp(big.NewInt(1_500_000)) != 0 {
		t.Fatalf("float %s", got)
	}
	if got := BigIntToFloat(big.NewInt(2_500_000)); got != 2.5 {
		t.Fatalf("big float %v", got)
	}
	if got := Uint64ToBigInt(42); got.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("uint %s", got)
	}
}

func TestFromHexAndRandomString(t *testing.T) {
	if got := string(FromHex("0x61")); got != "a" {
		t.Fatalf("from hex %q", got)
	}
	if got := FromHex("abc"); len(got) != 2 || got[0] != 0x0a {
		t.Fatalf("odd hex %#v", got)
	}
	if got := Hex2Bytes("zz"); len(got) != 0 {
		t.Fatalf("invalid hex %#v", got)
	}
	if got := RandStringBytesMaskImprSrcUnsafe(0); got != "" {
		t.Fatalf("empty random %q", got)
	}
	got := RandStringBytesMaskImprSrcUnsafe(32)
	if len(got) != 32 {
		t.Fatalf("len %d", len(got))
	}
	for _, c := range got {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Fatalf("unexpected char %q", c)
		}
	}
}
