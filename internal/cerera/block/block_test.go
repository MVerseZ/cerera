package block

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

var nodeAddress = types.HexToAddress("0x94F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")
var addr1 = types.HexToAddress("0x14F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")
var addr2 = types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")

func prepareSignedTx() *types.GTransaction {
	var acc, _ = types.GenerateAccount()

	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = types.HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	txs := &types.PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := types.NewTx(txs)
	signer := types.NewSimpleSignerWithPen(big.NewInt(25331), acc)
	tx, _ := types.SignTx(itx, signer, acc)
	return tx
}

func createTestHeader() *Header {
	header := &Header{
		Ctx:           1,
		Difficulty:    big.NewInt(100),
		Extra:         []byte("extra data"),
		Root:          common.EmptyHash(),
		Number:        big.NewInt(42),
		GasLimit:      5000000,
		GasUsed:       3000000,
		Timestamp:     uint64(time.June),
		Height:        10,
		Node:          nodeAddress,
		Confirmations: 10001,
		PrevHash:      common.EmptyHash(),
		Index:         42,
		Size:          13,
	}
	return header
}

func createTestBlock() *Block {

	tx1 := types.NewTransaction(
		11,
		types.HexToAddress("0x24F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100000000),
		1443,
		big.NewInt(33),
		[]byte{0xa, 0xb},
	)

	tx2 := types.NewTransaction(
		11,
		types.HexToAddress("0x14F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6"),
		big.NewInt(100001011),
		1343,
		big.NewInt(100),
		[]byte{0xe, 0xf},
	)

	header := &Header{
		Ctx:           1,
		Difficulty:    big.NewInt(100),
		Extra:         []byte("extra data"),
		Root:          common.EmptyHash(),
		Number:        big.NewInt(42),
		GasLimit:      5000000,
		GasUsed:       3000000,
		Timestamp:     uint64(time.June),
		Height:        10,
		Node:          nodeAddress,
		Confirmations: 10001,
		PrevHash:      common.EmptyHash(),
		Index:         42,
		Size:          13,
	}

	b := &Block{
		Nonce:        12345,
		Head:         header,
		Transactions: []types.GTransaction{*tx1, *tx2},
	}
	return b
}

func hashForTestBlock(b *Block) common.Hash {
	return CrvBlockHash(*b)
}

func hashForTestHeader(b *Header) common.Hash {
	return CrvHeaderHash(*b)
}

func TestBlockFields(t *testing.T) {
	block := createTestBlock()
	var expectedAddr = types.HexToAddress("0x94F369F35D4323dF9980eDF0E1bEdb882C4705e984Bb01aceE5B80F4b6Ad1A81a976278d1245dC6863CfF8ec7F99b5B6")
	// Проверка значений полей
	if block.Nonce != 12345 {
		t.Errorf("expected Nonce to be 12345, got %d", block.Nonce)
	}

	// Проверка полей в Header
	if block.Head.Ctx != 1 {
		t.Errorf("expected Header Ctx to be 1, got %d", block.Head.Ctx)
	}
	if block.Head.Number.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("expected Header Number to be 42, got %s", block.Head.Number.String())
	}
	if block.Head.Height != 10 {
		t.Errorf("expected Header Height to be 10, got %s", block.Head.Number.String())
	}
	if block.Head.Difficulty.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("expected Difficulty to be 100, got %s", block.Head.Difficulty.String())
	}
	if block.Head.Index != 42 {
		t.Errorf("expected Index to be 42, got %d", block.Head.Index)
	}
	if block.Head.Node != expectedAddr {
		t.Errorf("expected Index to be %s, got %s", expectedAddr, block.Head.Node)
	}
	if block.Head.Confirmations != 10001 {
		t.Errorf("expected Confirmations to be 10001, got %d", block.Head.Confirmations)
	}
	if block.Head.Size != 13 {
		t.Errorf("expected Size to be 13, got %d", block.Head.Size)
	}
	if big.NewInt(5000000).Cmp(big.NewInt(0).SetUint64(block.Head.GasLimit)) != 0 {
		t.Errorf("expected GasLimit to be %d, got %d", big.NewInt(5000000), block.Head.GasLimit)
	}
	if big.NewInt(3000000).Cmp(big.NewInt(0).SetUint64(block.Head.GasUsed)) != 0 {
		t.Errorf("expected GasUsed to be %d, got %d", big.NewInt(3000000), block.Head.GasUsed)
	}
}

func TestNewBlock(t *testing.T) {
	header := createTestHeader()
	block := NewBlock(header)
	if !block.EqHead(header) {
		t.Errorf("Header was not copied correctly! \r\nHave: %x, \r\nexpected: %x\r\n", block.Head, header)
	}
}

func TestCopyHeader(t *testing.T) {
	var header = createTestHeader()
	var block = NewBlockWithHeader(header)
	copy := CopyHeader(block.Header())
	if !reflect.DeepEqual(block.Head, copy) {
		t.Errorf("Copied header does not match the original")
	}
	// Измените исходный заголовок и проверьте, что копия осталась прежней
}

func TestEmptyBlockSerialize(t *testing.T) {
	header := createTestHeader()
	block := NewBlockWithHeader(header)
	blockBytes := block.ToBytes()
	parsedBlock, err := FromBytes(blockBytes)
	if err != nil {
		t.Errorf("Error while parse empty block")
	}
	if parsedBlock.Hash() != block.Hash() {
		t.Errorf("Different hashes! \r\nHave: %s, \r\nexpected: %s\r\n", parsedBlock.Hash(), block.Hash())
	}
}

func TestFilledBlockSerialize(t *testing.T) {
	header := createTestHeader()
	block := NewBlockWithHeader(header)
	block.Transactions = append(block.Transactions, *prepareSignedTx())
	blockBytes := block.ToBytes()
	parsedBlock, err := FromBytes(blockBytes)
	if err != nil {
		t.Errorf("Error while parse empty block")
	}
	fmt.Printf("%+v\r\n", block)
	fmt.Printf("%+v\r\n", parsedBlock)
	if parsedBlock.Hash() != block.Hash() {
		t.Errorf("Different hashes! \r\nHave: %s, \r\nexpected: %s\r\n", parsedBlock.Hash(), block.Hash())
	}
}

func TestHashFunctions(t *testing.T) {
	block := createTestBlock()
	expectedHash := hashForTestBlock(block)
	if block.Hash() != expectedHash {
		t.Errorf("Hash does not match expected value! Expected: %s, given: %s\r\n", expectedHash, block.Hash())
	}

	expectedHeaderHash := hashForTestHeader(block.Head)
	if block.Head.Hash() != expectedHeaderHash {
		t.Errorf("Hash does not match expected value! Expected: %s, given: %s\r\n", expectedHeaderHash, block.Head.Hash())
	}

	header := createTestHeader()
	blockWithTx := NewBlockWithHeader(header)
	blockWithTx.Transactions = append(blockWithTx.Transactions, *prepareSignedTx())
	expectedHash = hashForTestBlock(blockWithTx)
	if blockWithTx.Hash() != expectedHash {
		t.Errorf("Hash does not match expected value! Expected: %s, given: %s\r\n", expectedHash, block.Hash())
	}
}
