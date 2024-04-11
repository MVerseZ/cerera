package block

import (
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
		Transactions: []*types.GTransaction{tx1, tx2},
	}
	return b
}

func hashForTestBlock(b *Block) common.Hash {
	return CrvBlockHash(*b)
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

// func TestGenerateGenesis(t *testing.T) {
// 	genesisBlock := GenerateGenesis( /* адрес ноды */ )
// 	// проверка полей genesisBlock
// }

// func TestSerialization(t *testing.T) {
// 	block := createTestBlock()
// 	data := block.ToBytes()
// 	decodedBlock, err := FromBytes(data)
// 	if err != nil {
// 		t.Errorf("Error in decoding: %s", err)
// 	}
// 	// if !cmp.Equal(block, decodedBlock) {
// 	if !reflect.DeepEqual(block, &decodedBlock) {
// 		fmt.Printf("1 hash:%s\r\n", block.Hash())
// 		fmt.Printf("2 hash:%s\r\n", decodedBlock.Hash())
// 		fmt.Printf("1 node:%s\r\n", block.Head.Node)
// 		fmt.Printf("2 node:%s\r\n", decodedBlock.Head.Node)
// 		fmt.Printf("1 header hash:%s\r\n", block.Head.Hash())
// 		fmt.Printf("2 header hash:%s\r\n", decodedBlock.Head.Hash())
// 		fmt.Printf("1 tx:%d\r\n", len(block.Transactions))
// 		fmt.Printf("2 tx:%d\r\n", len(decodedBlock.Transactions))
// 		// fmt.Printf("1 tx 1:%s\r\n", block.Transactions[0].Hash())
// 		// fmt.Printf("2 tx 1:%s\r\n", decodedBlock.Transactions[0].Hash())
// 		t.Errorf("Decoded block does not match the original %s\r\n", err)
// 	}
// }

func TestHashFunctions(t *testing.T) {
	block := createTestBlock()
	expectedHash := hashForTestBlock(block)
	if block.Hash() != expectedHash {
		t.Errorf("Hash does not match expected value! Expected: %s, given: %s\r\n", expectedHash, block.Hash())
	}
}
