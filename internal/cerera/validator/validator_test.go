package validator

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/types"
)

func TestPoolSigningProc(t *testing.T) {
	pool := pool.InitPool(1, 1000)

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
	if len(info.Txs) != 3 {
		t.Errorf("Error pool size! Have %d, want %d\r\n", len(info.Txs), 3)
	}

	// pk, _ := types.GenerateAccount()
	// signer := types.NewSimpleSignerWithPen(big.NewInt(7), pk)
	// x509Encoded, _ := x509.MarshalECPrivateKey(pk)
	// pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	// 	// now validator sign tx
	// 	newHash, err := pool.SignRawTransaction(hashes[0], signer, string(pemEncoded))
	// 	if err != nil {
	// 		t.Errorf("Error while sign tx: %s\r\n", err)
	// 	}
	// 	if newHash != hashes[0] {
	// 		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[0])
	// 	}

	// 	newHash, err = pool.SignRawTransaction(hashes[1], signer, string(pemEncoded))
	// 	if err != nil {
	// 		t.Errorf("Error while sign tx: %s\r\n", err)
	// 	}
	// 	if newHash != hashes[1] {
	// 		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[1])
	// 	}

	// info = pool.GetInfo()
	// tx := pool.GetTransaction(info.Hashes[0])
	// var r, s, v = tx.RawSignatureValues()
	//
	//	if r == big.NewInt(0) || s == big.NewInt(0) || v == big.NewInt(0) {
	//		t.Errorf("Error! Tx not signed! %s\r\n", tx.Hash())
	//	}
}
