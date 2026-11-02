package common

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func TestBytesJSONAndGraphQL(t *testing.T) {
	var b Bytes
	if err := json.Unmarshal([]byte(`"0x6869"`), &b); err != nil || string(b) != "hi" {
		t.Fatal(err)
	}
	if b.String() != "0x6869" {
		t.Fatal(b.String())
	}
	encoded, err := json.Marshal(b)
	if err != nil || string(encoded) != `"0x6869"` {
		t.Fatalf("%s %v", encoded, err)
	}
	if err := json.Unmarshal([]byte(`"0xzz"`), &b); err == nil {
		t.Fatal("expected syntax error")
	}
	if err := json.Unmarshal([]byte(`"0xabc"`), &b); err == nil {
		t.Fatal("expected odd length")
	}
	if err := json.Unmarshal([]byte(`"01"`), &b); err == nil {
		t.Fatal("expected missing prefix")
	}
	if err := json.Unmarshal([]byte("1"), &b); err == nil {
		t.Fatal("expected non-string")
	}
	if err := b.UnmarshalText([]byte("")); err != nil || len(b) != 0 {
		t.Fatal(err)
	}
	if !b.ImplementsGraphQLType("Bytes") || b.ImplementsGraphQLType("other") {
		t.Fatal("graphql type")
	}
	if err := b.UnmarshalGraphQL("0x61"); err != nil || string(b) != "a" {
		t.Fatal(err)
	}
	if err := b.UnmarshalGraphQL(1); err == nil {
		t.Fatal("expected graphql type error")
	}
	if err := b.UnmarshalGraphQL("zz"); err == nil {
		t.Fatal("expected graphql decode error")
	}
}

func TestFixedHexText(t *testing.T) {
	out := make([]byte, 2)
	if err := UnmarshalFixedText("Hash", []byte("0x6162"), out); err != nil || string(out) != "ab" {
		t.Fatal(err)
	}
	if err := UnmarshalFixedUnprefixedText("Hash", []byte("6162"), out); err != nil || string(out) != "ab" {
		t.Fatal(err)
	}
	if err := UnmarshalFixedText("Hash", []byte("6162"), out); err == nil {
		t.Fatal("expected prefix")
	}
	if err := UnmarshalFixedText("Hash", []byte("0x61"), out); err == nil {
		t.Fatal("expected length error")
	}
	if err := UnmarshalFixedText("Hash", []byte("0x61zz"), out); err == nil {
		t.Fatal("expected syntax error")
	}
	if err := UnmarshalFixedJSON(bytesT, []byte(`"0x6162"`), out); err != nil {
		t.Fatal(err)
	}
	if err := UnmarshalFixedJSON(bytesT, []byte("1"), out); err == nil {
		t.Fatal("expected non-string")
	}
	if err := UnmarshalFixedText("Hash", []byte(""), out[:0]); err != nil {
		t.Fatal(err)
	}
}

func TestBigJSONAndGraphQL(t *testing.T) {
	var b Big
	if err := json.Unmarshal([]byte(`"0x10"`), &b); err != nil || b.ToInt().Cmp(big.NewInt(16)) != 0 {
		t.Fatal(err)
	}
	if b.String() != "0x10" {
		t.Fatal(b.String())
	}
	text, err := b.MarshalText()
	if err != nil || string(text) != "0x10" {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(&b)
	if err != nil || !bytes.Contains(encoded, []byte("0x10")) {
		t.Fatal(string(encoded))
	}
	if err := b.UnmarshalText([]byte("0x" + strings.Repeat("f", 65))); err != ErrBig256Range {
		t.Fatal(err)
	}
	if err := b.UnmarshalText([]byte("0xzz")); err != ErrSyntax {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte("1"), &b); err == nil {
		t.Fatal("expected non-string")
	}
	if !b.ImplementsGraphQLType("BigInt") {
		t.Fatal("graphql")
	}
	if err := b.UnmarshalGraphQL("0x2"); err != nil || b.ToInt().Cmp(big.NewInt(2)) != 0 {
		t.Fatal(err)
	}
	if err := b.UnmarshalGraphQL(int32(7)); err != nil || b.ToInt().Cmp(big.NewInt(7)) != 0 {
		t.Fatal(err)
	}
	if err := b.UnmarshalGraphQL(1.5); err == nil {
		t.Fatal("expected graphql type error")
	}
}

func TestUintJSONAndGraphQL(t *testing.T) {
	var u Uint64
	if err := json.Unmarshal([]byte(`"0x2a"`), &u); err != nil || u != 42 {
		t.Fatal(err)
	}
	if u.String() != "0x2a" {
		t.Fatal(u.String())
	}
	encoded, err := json.Marshal(u)
	if err != nil || string(encoded) != `"0x2a"` {
		t.Fatal(string(encoded))
	}
	if err := u.UnmarshalText([]byte("0x" + strings.Repeat("f", 17))); err != ErrUint64Range {
		t.Fatal(err)
	}
	if err := u.UnmarshalText([]byte("0xzz")); err != ErrSyntax {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte("1"), &u); err == nil {
		t.Fatal("expected non-string")
	}
	if !u.ImplementsGraphQLType("Long") {
		t.Fatal("graphql")
	}
	if err := u.UnmarshalGraphQL(int32(3)); err != nil || u != 3 {
		t.Fatal(err)
	}
	if err := u.UnmarshalGraphQL(true); err == nil {
		t.Fatal("expected graphql type error")
	}

	var n Uint
	if err := json.Unmarshal([]byte(`"0x2a"`), &n); err != nil || n != 42 {
		t.Fatal(err)
	}
	if n.String() != "0x2a" {
		t.Fatal(n.String())
	}
	if _, err := n.MarshalText(); err != nil {
		t.Fatal(err)
	}
	if err := n.UnmarshalText([]byte("0x" + strings.Repeat("f", 17))); err != ErrUintRange {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{}`), &n); err == nil {
		t.Fatal("expected non-string")
	}
}
