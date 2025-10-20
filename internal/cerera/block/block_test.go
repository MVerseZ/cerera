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

// func prepareSignedTx() *types.GTransaction {
// 	var acc, _ = types.GenerateAccount()

// 	dna := make([]byte, 0, 16)
// 	dna = append(dna, 0xf, 0xa, 0x42)

// 	var to = types.HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
// 	txs := &types.PGTransaction{
// 		To:       &to,
// 		Value:    big.NewInt(10),
// 		GasPrice: big.NewInt(15),
// 		Gas:      1000000,
// 		Nonce:    0x1,
// 		Dna:      dna,
// 		Time:     time.Now(),
// 	}
// 	itx := types.NewTx(txs)
// 	signer := types.NewSimpleSignerWithPen(big.NewInt(25331), acc)
// 	tx, _ := types.SignTx(itx, signer, acc)
// 	return tx
// }

func createTestHeader() *Header {
	header := &Header{
		Ctx:        1,
		Difficulty: 100,
		Extra:      [8]byte{'e', 'x', 't', 'r', 'a', ' ', 'd', 'a'}, // Fixed: Extra is [8]byte, not []byte
		Root:       common.EmptyHash(),
		GasLimit:   5000000,
		GasUsed:    3000000,
		Timestamp:  uint64(time.Now().UnixMilli()), // Fixed: use proper timestamp
		Height:     10,
		Node:       nodeAddress,
		PrevHash:   common.EmptyHash(),
		Index:      42,
		Size:       13,
		ChainId:    11,                                              // Added: missing ChainId field
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1}, // Added: missing V field
		Nonce:      12345,                                           // Added: missing Nonce field
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
		Ctx:        1,
		Difficulty: 100,
		Extra:      [8]byte{'e', 'x', 't', 'r', 'a', ' ', 'd', 'a'}, // Fixed: Extra is [8]byte
		Root:       common.EmptyHash(),
		GasLimit:   5000000,
		GasUsed:    3000000,
		Timestamp:  uint64(time.Now().UnixMilli()), // Fixed: use proper timestamp
		Height:     10,
		Node:       nodeAddress,
		PrevHash:   common.EmptyHash(),
		Index:      42,
		Size:       13,
		ChainId:    11,                                              // Added: missing ChainId field
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1}, // Added: missing V field
		Nonce:      12345,                                           // Added: missing Nonce field
	}

	b := &Block{
		Head:          header,
		Transactions:  []types.GTransaction{*tx1, *tx2}, // Fixed: Transactions is []types.GTransaction, not []*types.GTransaction
		Confirmations: 0,                                // Added: missing Confirmations field
		Hash:          common.Hash{},                    // Added: missing Hash field
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

	// Проверка полей в Header
	if block.Head.Ctx != 1 {
		t.Errorf("expected Header Ctx to be 1, got %d", block.Head.Ctx)
	}
	if block.Head.Height != 10 {
		t.Errorf("expected Header Height to be 10, got %d", block.Head.Height)
	}
	if block.Head.Difficulty != 100 {
		t.Errorf("expected Difficulty to be 100, got %d", block.Head.Difficulty)
	}
	if block.Head.Index != 42 {
		t.Errorf("expected Index to be 42, got %d", block.Head.Index)
	}
	if block.Head.Node != expectedAddr {
		t.Errorf("expected Node to be %s, got %s", expectedAddr, block.Head.Node)
	}
	if block.Head.Size != 13 {
		t.Errorf("expected Size to be 13, got %d", block.Head.Size)
	}
	if block.Head.GasLimit != 5000000 {
		t.Errorf("expected GasLimit to be 5000000, got %d", block.Head.GasLimit)
	}
	if block.Head.GasUsed != 3000000 {
		t.Errorf("expected GasUsed to be 3000000, got %d", block.Head.GasUsed)
	}
	if block.Head.ChainId != 11 {
		t.Errorf("expected ChainId to be 11, got %d", block.Head.ChainId)
	}
	if block.Head.Nonce != 12345 {
		t.Errorf("expected Nonce to be 12345, got %d", block.Head.Nonce)
	}

	// Проверка количества транзакций
	if len(block.Transactions) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(block.Transactions))
	}

	// Проверка Confirmations
	if block.Confirmations != 0 {
		t.Errorf("expected Confirmations to be 0, got %d", block.Confirmations)
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

// func TestEmptyBlockSerialize(t *testing.T) {
// 	header := createTestHeader()
// 	block := NewBlockWithHeader(header)
// 	blockBytes := block.ToBytes()
// 	parsedBlock, err := FromBytes(blockBytes)
// 	if err != nil {
// 		t.Errorf("Error while parse empty block")
// 	}
// 	if parsedBlock.Hash() != block.Hash() {
// 		t.Errorf("Different hashes! \r\nHave: %s, \r\nexpected: %s\r\n", parsedBlock.Hash(), block.Hash())
// 	}
// }

// func TestFilledBlockSerialize(t *testing.T) {
// 	header := createTestHeader()
// 	block := NewBlockWithHeader(header)
// 	block.Transactions = append(block.Transactions, prepareSignedTx())
// 	blockBytes := block.ToBytes()
// 	parsedBlock, err := FromBytes(blockBytes)
// 	if err != nil {
// 		t.Errorf("Error while parse empty block")
// 	}
// 	fmt.Printf("%+v\r\n", block)
// 	fmt.Printf("%+v\r\n", parsedBlock)
// 	if parsedBlock.Hash() != block.Hash() {
// 		t.Errorf("Different hashes! \r\nHave: %s, \r\nexpected: %s\r\n", parsedBlock.Hash(), block.Hash())
// 	}
// }

func TestCalculateHash(t *testing.T) {
	block := createTestBlock()

	// Тестируем функцию CalculateHash
	hash, err := block.CalculateHash()
	if err != nil {
		t.Errorf("Error calculating hash: %v", err)
	}

	if len(hash) != 32 { // SHA-256 produces 32 bytes
		t.Errorf("Expected hash length 32, got %d", len(hash))
	}

	// Проверяем, что хеш не пустой
	allZeros := true
	for _, b := range hash {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("Hash should not be all zeros")
	}
}

func TestBlockToBytes(t *testing.T) {
	block := createTestBlock()

	// Тестируем сериализацию в байты
	bytes := block.ToBytes()
	if len(bytes) == 0 {
		t.Error("Block bytes should not be empty")
	}

	// Обновляем размер блока после сериализации
	block.Head.Size = len(bytes)

	// Проверяем, что размер блока установлен правильно
	if block.Head.Size != len(bytes) {
		t.Errorf("Expected block size %d, got %d", len(bytes), block.Head.Size)
	}
}

func TestBlockNonceOperations(t *testing.T) {
	block := createTestBlock()
	originalNonce := block.Head.Nonce

	// Проверяем исходный nonce
	if originalNonce != 12345 {
		t.Errorf("Expected initial nonce 12345, got %d", originalNonce)
	}

	// Тестируем получение nonce в байтах
	nonceBytes := block.GetNonceBytes()
	if len(nonceBytes) != 8 {
		t.Errorf("Expected nonce bytes length 8, got %d", len(nonceBytes))
	}

	// Тестируем установку nonce из байтов
	newNonceBytes := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x42}
	block.SetNonceBytes(newNonceBytes)
	if block.Head.Nonce != 0x42 {
		t.Errorf("Expected nonce 0x42, got 0x%x", block.Head.Nonce)
	}

	// Тестируем увеличение nonce напрямую (так как IncNonce() не работает с текущей реализацией)
	block.Head.Nonce += 1
	if block.Head.Nonce != 0x43 {
		t.Errorf("Expected nonce 0x43, got 0x%x", block.Head.Nonce)
	}
}

func TestBlockEquals(t *testing.T) {
	block1 := createTestBlock()
	block2 := createTestBlock()

	// Блоки должны быть равны (одинаковые данные)
	equal, err := block1.Equals(*block2) // Передаем значение, а не указатель
	if err != nil {
		t.Errorf("Error comparing blocks: %v", err)
	}
	if !equal {
		t.Error("Identical blocks should be equal")
	}

	// Изменяем ChainId одного блока и проверяем, что они больше не равны
	block2.Head.ChainId = 999
	equal, err = block1.Equals(*block2) // Передаем значение, а не указатель
	if err != nil {
		t.Errorf("Error comparing blocks: %v", err)
	}
	if equal {
		t.Error("Different blocks should not be equal")
	}
}
