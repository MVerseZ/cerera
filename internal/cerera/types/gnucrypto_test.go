package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
)

func TestKxAddress(t *testing.T) {
	var curve = elliptic.P256()
	var k1, err = ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Error(err)
	}
	var addr = PubkeyToAddress(k1.PublicKey)
	var s, _ = PublicKeyToString(&k1.PublicKey)
	var pk, _ = PublicKeyFromString(s)
	if pk.X == k1.X {
		t.Fatal("diff keys")
	}
	fmt.Printf("=== EXEC	Generating address: %s\r\n", addr)
}
