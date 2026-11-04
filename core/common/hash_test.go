package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestHashBytesAndCompare(t *testing.T) {
	raw := bytes.Repeat([]byte{0xab}, HashLength+4)
	h := BytesToHash(raw)
	if !bytes.Equal(h.Bytes(), raw[len(raw)-HashLength:]) {
		t.Fatal("hash was not cropped from the left")
	}
	if h.Hex() != "0x"+strings.Repeat("ab", HashLength) || h.String() != h.Hex() {
		t.Fatalf("hex %s", h.Hex())
	}
	if h.TerminalString() == "" || h.Big().BitLen() == 0 {
		t.Fatal("terminal or big conversion failed")
	}
	if BigToHash(big.NewInt(1)).Hex() == EmptyHash().Hex() {
		t.Fatal("big hash collapsed to empty")
	}
	parsed := HexToHash("0x01")
	if parsed.Bytes()[HashLength-1] != 1 {
		t.Fatal("hex hash")
	}
	if h.Compare(h) != 0 || BytesToHash([]byte{1}).Compare(BytesToHash([]byte{2})) >= 0 {
		t.Fatal("compare")
	}
	if BytesToHash([]byte{3}).Compare(BytesToHash([]byte{2})) <= 0 {
		t.Fatal("compare greater")
	}
	if EmptyHash().Compare(Hash{}) != 0 {
		t.Fatal("empty hash")
	}
	if EmptyRootHash == (Hash{}) || EmptyCodeRootHash != (Hash{}) {
		t.Fatal("known hashes")
	}
}

func TestHashFormatSQLAndJSON(t *testing.T) {
	h := BytesToHash([]byte{0x0a, 0xff})
	if !strings.HasPrefix(fmt.Sprintf("%x", h), "000") {
		t.Fatalf("%%x %s", fmt.Sprintf("%x", h))
	}
	if !strings.Contains(fmt.Sprintf("%#x", h), "0x") {
		t.Fatal("missing prefix")
	}
	if fmt.Sprintf("%X", h) != strings.ToUpper(fmt.Sprintf("%x", h)) {
		t.Fatal("upper hex")
	}
	if !strings.HasPrefix(fmt.Sprintf("%q", h), `"`) {
		t.Fatal("quoted")
	}
	if !strings.Contains(fmt.Sprintf("%d", h), "10") {
		t.Fatalf("decimal format %s", fmt.Sprintf("%d", h))
	}
	if !strings.Contains(fmt.Sprintf("%t", h), "%!t") {
		t.Fatalf("default format %s", fmt.Sprintf("%t", h))
	}
	if fmt.Sprintf("%s", h) != fmt.Sprintf("%v", h) {
		t.Fatal("string and verb formats diverged")
	}

	val, err := h.Value()
	if err != nil || len(val.([]byte)) != HashLength {
		t.Fatal(err)
	}
	var scanned Hash
	if err := scanned.Scan(h.Bytes()); err != nil || scanned != h {
		t.Fatal(err)
	}
	if err := scanned.Scan("nope"); err == nil {
		t.Fatal("expected type error")
	}
	if err := scanned.Scan([]byte{1}); err == nil {
		t.Fatal("expected length error")
	}

	text, err := h.MarshalText()
	if err != nil || !bytes.HasPrefix(text, []byte("0x")) {
		t.Fatal(err)
	}
	var out Hash
	if err := out.UnmarshalText(text); err != nil || out != h {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Hash
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != h {
		t.Fatal(err)
	}
	if err := decoded.UnmarshalJSON([]byte("1")); err == nil {
		t.Fatal("expected non-string json")
	}

	generated := h.Generate(rand.New(rand.NewSource(1)), 0)
	if generated.Type() != reflect.TypeOf(Hash{}) {
		t.Fatal("generate")
	}
}
