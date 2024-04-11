package pool

import (
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"math/rand"
	"strconv"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

func TestPoolInit(t *testing.T) {
	pool := InitPool(10, 10)
	if pool == nil {
		t.Error("Error while initializing pool!")
	}
}

func TestAddTx(t *testing.T) {
	pool := InitPool(10, 10)

	transaction := types.NewTransaction(7,
		types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
		big.NewInt(1000001),
		500,
		big.NewInt(250),
		[]byte{},
	)
	pool.AddRawTransaction(transaction)
	info := pool.GetInfo()
	if info.Size != 1 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", info.Size, 1)
	}
}

func TestAddTxWithFrom(t *testing.T) {
	pool := InitPool(10, 10)
	from := types.HexToAddress("0xc9C5c06E295d8FB8E97f4df9ddd919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613")
	transaction := types.NewTransaction(7,
		types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
		big.NewInt(1000001),
		500,
		big.NewInt(250),
		[]byte{},
	)
	pool.AddTransaction(from, transaction)
	info := pool.GetInfo()
	if info.Size != 1 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", info.Size, 1)
	}
}

func TestPoolSize(t *testing.T) {
	pool := InitPool(10, 10)

	for i := 0; i < 1000; i++ {
		transaction := types.NewTransaction(7,
			types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
			big.NewInt(1000001),
			500,
			big.NewInt(250),
			[]byte(strconv.Itoa(i)),
		)
		pool.AddRawTransaction(transaction)
	}
	info := pool.GetInfo()
	if info.Size != 10 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", info.Size, 10)
	}
}

func TestAddTxs(t *testing.T) {
	pool := InitPool(10000, 10000)

	for i := 0; i < 1000; i++ {
		transaction := types.NewTransaction(7,
			types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
			big.NewInt(1000001),
			50000000,
			big.NewInt(250),
			[]byte(strconv.Itoa(i)),
		)
		pool.AddRawTransaction(transaction)
	}
	info := pool.GetInfo()
	if info.Size != 1000 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", info.Size, 1)
	}
}

func TestAddSmallerGasTxs(t *testing.T) {
	pool := InitPool(1000004186996573, 10000)

	for i := 0; i < 1000; i++ {
		transaction := types.NewTransaction(7,
			types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
			big.NewInt(1000001),
			uint64(rand.Uint32()),
			big.NewInt(250),
			[]byte(strconv.Itoa(i)),
		)
		// fmt.Printf("tx gas: %d pool min gas: %d\r\n", transaction.Gas(), pool.minGas)
		pool.AddRawTransaction(transaction)
	}
	info := pool.GetInfo()
	if info.Size != 0 {
		t.Errorf("Error pool size! Have %d, need to  %d\r\n", info.Size, 0)
	}
}

func TestPoolSigningProc(t *testing.T) {
	pool := InitPool(1, 1000)

	var hashes []common.Hash
	for i := 0; i < 3; i++ {
		transaction := types.NewTransaction(
			7,
			types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
			big.NewInt(1000001),
			500,
			big.NewInt(250),
			[]byte(strconv.Itoa(i)),
		)
		pool.AddRawTransaction(transaction)
		hashes = append(hashes, transaction.Hash())
	}
	info := pool.GetInfo()
	if info.Size != 3 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", info.Size, 1000)
	}

	pk, _ := types.GenerateAccount()
	signer := types.NewSimpleSignerWithPen(big.NewInt(7), pk)
	x509Encoded, _ := x509.MarshalECPrivateKey(pk)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	newHash := pool.SignRawTransaction(hashes[0], signer, string(pemEncoded))
	if newHash != hashes[0] {
		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[0])
	}

	newHash = pool.SignRawTransaction(hashes[1], signer, string(pemEncoded))
	if newHash != hashes[1] {
		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[1])
	}

	info = pool.GetInfo()
	tx := pool.GetTransaction(info.Hashes[0])
	var r, s, v = tx.RawSignatureValues()
	if r == big.NewInt(0) || s == big.NewInt(0) || v == big.NewInt(0) {
		t.Errorf("Error! Tx not signed! %s\r\n", tx.Hash())
	}
}
