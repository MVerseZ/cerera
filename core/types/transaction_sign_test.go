package types

import (
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"golang.org/x/crypto/blake2b"
)

func TestSigningTx(t *testing.T) {
	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	addr := PubkeyToAddress(&accPrivKey.PublicKey)

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

	signer := NewSimpleSigner(big.NewInt(25331))

	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.IsSigned() {
		t.Fatal("tx should be signed!")
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}

	// Исправлено: сравниваем from с addr
	if from != addr {
		t.Errorf("expected from and address to be equal. Got %x want %x", from, addr)
	}

	// Проверяем тип транзакции
	if tx.Type() != txs.txType() {
		t.Errorf("expected tx type %d, got %d", txs.txType(), tx.Type())
	}

	if itx.Hash() != tx.Hash() {
		t.Errorf("different hashes!! Got %s want %s", itx.Hash().Hex(), tx.Hash().Hex())
	}
	if itx.Value().Cmp(tx.Value()) != 0 {
		t.Errorf("different inner values!! Got %d want %d", itx.Value(), tx.Value())
	}
	if !reflect.DeepEqual(itx.Data(), tx.Data()) {
		t.Errorf("different data!! Got %x want %x", itx.Data(), tx.Data())
	}
	if itx.Gas() != tx.Gas() {
		t.Errorf("different gas!! Got %d want %d", itx.Gas(), tx.Gas())
	}
	if itx.GasPrice().Cmp(tx.GasPrice()) != 0 {
		t.Errorf("different gas price!! Got %d want %d", itx.GasPrice(), tx.GasPrice())
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
		t.Errorf("different hashes! Have %s\r\n want %s\r\n", transaction.Hash().Hex(), txHash.Hex())
	}

	otherTxHash := crvTxHash(transaction.inner)
	if transaction.Hash() != otherTxHash {
		t.Errorf("different hashes! Have %s\r\n want %s\r\n", transaction.Hash().Hex(), otherTxHash.Hex())
	}

	time.Sleep(time.Second * 2)

	// Создаем другую транзакцию с другим временем
	otherTransaction := NewTransaction(
		7,
		HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
		big.NewInt(1000001),
		500,
		big.NewInt(250),
		[]byte{},
	)

	// // Убеждаемся, что время другое
	// if otherTransaction.Time().Unix() == transaction.Time().Unix() {
	// 	t.Log("WARNING: Transactions have same timestamp, hash may be same")
	// }

	if otherTransaction.Hash() == transaction.Hash() {
		t.Errorf("similar hashes! Have %s\r\n want %s\r\n",
			otherTransaction.Hash().Hex(), transaction.Hash().Hex())
	}
}

func TestGetSender(t *testing.T) {
	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	addr := PubkeyToAddress(&accPrivKey.PublicKey)

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

	signer := NewSimpleSigner(big.NewInt(25331))

	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	senderAddr, err := signer.Sender(tx)
	if err != nil {
		t.Fatal(err)
	}

	if senderAddr.Hex() != addr.Hex() {
		t.Errorf("Different addresses! Have %s, expected %s\r\n",
			senderAddr.Hex(), addr.Hex())
	}
}

func TestSizeSigning(t *testing.T) {
	var accPrivKey, err = GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	addr := PubkeyToAddress(&accPrivKey.PublicKey)

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

	// Ожидаемый размер для неподписанной транзакции
	expectedUnsignedSize := uint64(68)
	if itx.Size() != expectedUnsignedSize {
		t.Errorf("diff sizes for unsigned tx: expected %d, actual: %d",
			expectedUnsignedSize, itx.Size())
	}

	signer := NewSimpleSigner(big.NewInt(25331))
	tx, err := SignTx(itx, signer, accPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.IsSigned() {
		t.Fatal("tx should be signed!")
	}

	// Проверяем, что размер подписанной транзакции больше
	expectedSignedSize := tx.Size()
	if expectedSignedSize <= expectedUnsignedSize {
		t.Errorf("signed tx size should be greater than unsigned: unsigned=%d signed=%d",
			expectedUnsignedSize, expectedSignedSize)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != addr {
		t.Errorf("expected from and address to be equal. Got %x want %x", from, addr)
	}

	bhash, err := tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash != tx.Hash() {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n",
			tx.Hash().Hex(), sbHash.Hex())
	}

	// Тестируем другую транзакцию
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx1 = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)

	// Ожидаемый размер зависит от ОС
	expectedSize := uint64(76)
	if runtime.GOOS == "windows" {
		// На Windows размер может быть другим
		expectedSize = uint64(76)
	}

	if tx1.Size() != expectedSize {
		t.Errorf("diff sizes: expected %d, actual: %d", expectedSize, tx1.Size())
	}
}

func TestDecodeSignatureTooShort(t *testing.T) {
	// Слишком короткая подпись должна давать нулевые значения
	sig := []byte{0x01, 0x02, 0x03}
	r, s, v := decodeSignature(sig)

	if r.Sign() != 0 || s.Sign() != 0 || v.Sign() != 0 {
		t.Fatalf("expected zero values for r,s,v on short signature, got r=%d s=%d v=%d",
			r, s, v)
	}
}

func TestSenderOnUnsignedTxReturnsError(t *testing.T) {
	tx := NewTransaction(
		1,
		HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(100),
		3,
		big.NewInt(1),
		[]byte("test"),
	)

	signer := NewSimpleSigner(big.NewInt(1))
	_, err := signer.Sender(tx)
	if err == nil {
		t.Fatal("expected error for unsigned transaction")
	}
}

func TestRecoverPlain(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(crypto.ChainElliptic(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("test message")
	sig, err := Sign(msg, privKey)
	if err != nil {
		t.Fatal(err)
	}

	r, s, v := decodeSignature(sig)
	digest := blake2b.Sum256(msg)
	addr, err := recoverPlain(digest[:], r, s, v)
	if err != nil {
		t.Fatal(err)
	}

	expectedAddr := PubkeyToAddress(&privKey.PublicKey)
	if addr != expectedAddr {
		t.Errorf("address mismatch: got %s, expected %s",
			addr.Hex(), expectedAddr.Hex())
	}

	t.Logf("✅ Recovery test passed: %s", addr.Hex())
}

// Бенчмарк для проверки производительности
func BenchmarkSignTx(b *testing.B) {
	privKey, err := GenerateAccount()
	if err != nil {
		b.Fatal(err)
	}

	tx := NewTransaction(1,
		HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(100), 10, big.NewInt(1), []byte("test"))

	signer := NewSimpleSigner(big.NewInt(1))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SignTx(tx, signer, privKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Бенчмарк для проверки восстановления адреса
func BenchmarkSender(b *testing.B) {
	privKey, err := GenerateAccount()
	if err != nil {
		b.Fatal(err)
	}

	tx := NewTransaction(1,
		HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(100), 10, big.NewInt(1), []byte("test"))

	signer := NewSimpleSigner(big.NewInt(1))
	tx, err = SignTx(tx, signer, privKey)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Sender(signer, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
