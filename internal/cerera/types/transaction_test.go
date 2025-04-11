package types

import (
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
)

var nonce, txValue, txData, gasLimit, gasPrice, to = uint64(1337), FloatToBigInt(11.55), []byte("test data"), uint64(16438), big.NewInt(63992), HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")

func TestCreate(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    txValue,
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	if itx.Value().Cmp(txValue) != 0 {
		t.Errorf("Different values! Have %d, want %d", itx.Value(), big.NewInt(10))
	}
	if itx.GasPrice().Cmp(big.NewInt(15)) != 0 {
		t.Errorf("Different gas price! Have %d, want %d", itx.Value(), big.NewInt(15))
	}
	if itx.Gas() != 1000000 {
		t.Errorf("Different gas price! Have %d, want %d", itx.Value(), big.NewInt(1000000))
	}

	var ttx, err = CreateTransaction(1337, to, BigIntToFloat(txValue), 100001, "test message")
	if err != nil {
		t.Errorf("Error while create tx, %s", err)
	}
	if ttx.Value().Cmp(txValue) != 0 {
		t.Errorf("Different values! Have %d, want %d", ttx.Value(), txValue)
	}
}

func TestCost(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
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

	var calcCost = itx.Cost()
	var expectedCost = big.NewInt(15000010)
	if calcCost.Cmp(expectedCost) != 0 {
		t.Errorf("Differenet sizes! Have %d, want %d", calcCost, expectedCost)
	}
}

func TestSerialize(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
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

	txBytes, err := itx.MarshalJSON()
	if err != nil {
		t.Error("Error while parse transaction to bytes!")
	}

	var tx GTransaction
	tx.UnmarshalJSON(txBytes)

	if tx.size != itx.size {
		t.Errorf("Differenet sizes! Have %d, want %d", tx.Size(), itx.Size())
	}
}

func TestEquals(t *testing.T) {
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)
	var bhash, err = tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash.Compare(tx.Hash()) != 0 {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n", tx.Hash(), sbHash)
	}
	fmt.Println(sbHash)
	fmt.Println(tx.Hash())
}

func TestSize(t *testing.T) {
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)
	var bhash, err = tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash.Compare(tx.Hash()) != 0 {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n", tx.Hash(), sbHash)
	}
	txSize := uint64(0)
	if runtime.GOOS == "windows" {
		fmt.Println("Hello from Windows")
		txSize = uint64(393)
	} else {
		txSize = uint64(392)
	}
	if tx.Size() != txSize || txSize == 0 {
		t.Errorf("diff sizes: expected %d, actual: %d", txSize, tx.Size())
	}
}

func TestCreateCoinbase(t *testing.T) {
	NewCoinBaseTransaction(nonce, to, txValue, gasLimit, gasPrice, txData)
}

func TestCreateFaucet(t *testing.T) {
	NewFaucetTransaction(nonce, to, txValue)
}

func TestDna(t *testing.T) {
	var tx = NewTransaction(
		nonce,
		to,
		txValue,
		gasLimit,
		gasPrice,
		txData,
	)
	tx.Dna()
}
