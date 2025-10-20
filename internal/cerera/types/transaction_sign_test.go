package types

import (
	"math/big"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
)

func TestSigningTx(t *testing.T) {

	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	// signerAcc, err := GenerateAccount()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	addr := PubkeyToAddress(accPrivKey.PublicKey)

	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	signer := NewSimpleSigner(big.NewInt(25331)) //, signerAcc)

	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.IsSigned() {
		t.Fatal("tx should be signed!")
	}

	from, err := Sender(signer, tx)
	if tx.Type() != txs.txType() {
		t.Errorf("exected from and address to be equal. Got %x want %x", tx.Type(), txs.txType())
	}
	if err != nil {
		t.Fatal(err)
	}
	if from != addr {
		t.Errorf("exected from and address to be equal. Got %x want %x", from, addr)
	}
	if itx.Hash() != tx.Hash() {
		t.Errorf("different hashes!! Got %s want %s", itx.Hash(), tx.Hash())
	}
	if itx.Value().Cmp(tx.Value()) != 0 {
		t.Errorf("different inner values!! Got %d want %d", itx.Value(), tx.Value())
	}
	if !reflect.DeepEqual(itx.Data(), tx.Data()) {
		t.Errorf("different data!! Got %s want %s", itx.Data(), tx.Data())
	}
	if itx.Gas() != tx.Gas() {
		t.Errorf("different gas!! Got %f want %f", itx.Gas(), tx.Gas())
	}
	if itx.GasPrice().Cmp(tx.GasPrice()) != 0 {
		t.Errorf("different gas!! Got %d want %d", itx.GasPrice(), tx.GasPrice())
	}
	if itx.Nonce() != tx.Nonce() {
		t.Errorf("different nonce!! Got %d want %d", itx.Nonce(), tx.Nonce())
	}
}

func TestHashTx(t *testing.T) {
	transaction := NewTransaction(7,
		HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
		big.NewInt(1000001),
		500,
		big.NewInt(250),
		[]byte{},
	)
	txHash := crvTxHash(transaction.inner)
	if transaction.Hash() != txHash {
		t.Errorf("different hashes! Have %s\r\n want %s\r\n", transaction.Hash(), txHash)
	}
	otherTxHash := crvTxHash(transaction.inner)
	if transaction.Hash() != otherTxHash {
		t.Errorf("different hashes! Have %s\r\n want %s\r\n", transaction.Hash(), otherTxHash)
	}
	time.Sleep(time.Second * 2)
	// we wait 3 sec and create other tx with timestamp = +3 secs
	// hash check creation time - hashes should be different
	otherTransaction := NewTransaction(
		7,
		HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
		big.NewInt(1000001),
		500,
		big.NewInt(250),
		[]byte{},
	)
	crvTxHash(otherTransaction.inner)
	if otherTransaction.Hash() == transaction.Hash() {
		t.Errorf("similar hashes! Have %s\r\n want %s\r\n", otherTransaction.Hash(), transaction.Hash())
	}
}

func TestGetSender(t *testing.T) {
	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	addr := PubkeyToAddress(accPrivKey.PublicKey)

	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	signer := NewSimpleSigner(big.NewInt(25331)) //, signerAcc)

	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	senderAddr, err := signer.Sender(tx)
	if err != nil {
		t.Fatal(err)
	}
	if senderAddr.Hex() != addr.Hex() {
		t.Errorf("Different addresses! Have %s, expected %s\r\n", senderAddr.Hex(), addr.Hex())
	}
}

func TestSizeSigning(t *testing.T) {
	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	addr := PubkeyToAddress(accPrivKey.PublicKey)

	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	// Expected size for unsigned transaction:

	// Total: 81 bytes
	expectedUnsignedSize := uint64(81)

	if itx.Size() != expectedUnsignedSize {
		t.Errorf("diff sizes for unsigned tx: expected %d, actual: %d", expectedUnsignedSize, itx.Size())
	}

	signer := NewSimpleSigner(big.NewInt(25331))
	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.IsSigned() {
		t.Fatal("tx should be signed!")
	}

	// Expected size for signed transaction:

	// Total: 161 bytes
	expectedSignedSize := uint64(161)

	if tx.Size() != expectedSignedSize {
		t.Errorf("diff sizes for signed tx: expected %d, actual: %d", expectedSignedSize, tx.Size())
	}

	from, err := Sender(signer, tx)
	if tx.Type() != txs.txType() {
		t.Errorf("exected from and address to be equal. Got %x want %x", tx.Type(), txs.txType())
	}
	if err != nil {
		t.Fatal(err)
	}
	if from != addr {
		t.Errorf("exected from and address to be equal. Got %x want %x", from, addr)
	}
	bhash, err := tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash.Compare(tx.Hash()) != 0 {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n", tx.Hash(), sbHash)
	}

	// next part
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx1 = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)
	txSize := uint64(0)
	if runtime.GOOS == "windows" {
		// fmt.Println("Hello from Windows")
		txSize = uint64(92)
	} else {
		txSize = uint64(91)
	}
	if tx1.Size() != txSize || txSize == 0 {
		t.Errorf("diff sizes: expected %d, actual: %d", txSize, tx1.Size())
	}
}
