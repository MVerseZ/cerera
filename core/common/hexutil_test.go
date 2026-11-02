package common

import (
	"bytes"
	"math/big"
	"testing"
)

func TestHexEncodeDecode(t *testing.T) {
	raw, err := Decode("0x6869")
	if err != nil || string(raw) != "hi" {
		t.Fatal(err)
	}
	if Encode([]byte("hi")) != "0x6869" {
		t.Fatal(Encode([]byte("hi")))
	}
	if _, err := Decode(""); err != ErrEmptyString {
		t.Fatal(err)
	}
	if _, err := Decode("6869"); err != ErrMissingPrefix {
		t.Fatal(err)
	}
	if _, err := Decode("0xzz"); err != ErrSyntax {
		t.Fatal(err)
	}
	if _, err := Decode("0xabc"); err != ErrOddLength {
		t.Fatal(err)
	}
	if !Has0xPrefix("0X1") || Has0xPrefix("1") {
		t.Fatal("prefix")
	}
}

func TestMustDecodePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustDecode("nope")
}

func TestHexUint64(t *testing.T) {
	n, err := DecodeUint64("0x2a")
	if err != nil || n != 42 {
		t.Fatal(err)
	}
	if EncodeUint64(42) != "0x2a" || EncodeUint64(0) != "0x0" {
		t.Fatal(EncodeUint64(42))
	}
	if _, err := DecodeUint64("0x01"); err != ErrLeadingZero {
		t.Fatal(err)
	}
	if _, err := DecodeUint64("0x"); err != ErrEmptyNumber {
		t.Fatal(err)
	}
	if _, err := DecodeUint64("0x10000000000000000"); err != ErrUint64Range {
		t.Fatal(err)
	}
	if MustDecodeUint64("0x10") != 16 {
		t.Fatal("must")
	}
}

func TestMustDecodeUint64Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustDecodeUint64("0x00")
}

func TestHexBig(t *testing.T) {
	n, err := DecodeBig("0x10")
	if err != nil || n.Cmp(big.NewInt(16)) != 0 {
		t.Fatal(err)
	}
	if _, err := DecodeBig("0x" + string(bytes.Repeat([]byte("f"), 65))); err != ErrBig256Range {
		t.Fatal(err)
	}
	if _, err := DecodeBig("0xgg"); err != ErrSyntax {
		t.Fatal(err)
	}
	if EncodeBig(big.NewInt(0)) != "0x0" || EncodeBig(big.NewInt(255)) != "0xff" {
		t.Fatal("encode positive")
	}
	if EncodeBig(big.NewInt(-15)) != "-0xf" {
		t.Fatalf("negative %s", EncodeBig(big.NewInt(-15)))
	}
	if MustDecodeBig("0x2").Cmp(big.NewInt(2)) != 0 {
		t.Fatal("must big")
	}
}

func TestMustDecodeBigPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	MustDecodeBig("")
}

func TestDecError(t *testing.T) {
	if ErrSyntax.Error() == "" || ErrUintRange.Error() == "" {
		t.Fatal("empty error text")
	}
}
