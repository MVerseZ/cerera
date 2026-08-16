package block

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/core/types/trie"
	"golang.org/x/crypto/blake2b"
)

// Header represents a block header in the blockchain.
type Header struct {
	Ctx        int32           `json:"ctx" gencodec:"required"`               // 4 bytes
	Difficulty uint64          `json:"difficulty"       gencodec:"required"`  // 8 bytes
	Extra      [8]byte         `json:"extraData"        gencodec:"required"`  // 8 bytes
	GasLimit   uint64          `json:"gasLimit"         gencodec:"required"`  // 8 bytes
	GasUsed    uint64          `json:"gasUsed"          gencodec:"required"`  // 8 bytes
	Height     int             `json:"height" gencodec:"required"`            // 8 bytes
	Index      uint64          `json:"index" gencodec:"required"`             // 8 bytes
	Node       address.Address `json:"node" gencodec:"required"`              // 20 bytes
	ChainId    int             `json:"chainId"           gencodec:"required"` // 8 bytes
	PrevHash   common.Hash     `json:"prevHash" gencodec:"required"`          // 32 bytes
	Root       common.Hash     `json:"stateRoot"        gencodec:"required"`  // 32 bytes
	Size       int             `json:"size" gencodec:"required"`              // 8 bytes
	Timestamp  uint64          `json:"timestamp"        gencodec:"required"`  // 8 bytes
	V          [8]byte         `json:"version"        gencodec:"required"`    // 8 bytes
	Nonce      uint64          `json:"nonce"        gencodec:"required"`      // 8 bytes
}

func (h *Header) Bytes() []byte {
	var buf bytes.Buffer

	// Используем binary encoding вместо byte() для больших чисел
	binary.Write(&buf, binary.BigEndian, h.Ctx)            // int32 -> 4 bytes
	binary.Write(&buf, binary.BigEndian, h.Difficulty)     // uint64 -> 8 bytes
	buf.Write(h.Extra[:])                                  // [8]byte -> 8 bytes
	binary.Write(&buf, binary.BigEndian, h.GasLimit)       // uint64 -> 8 bytes
	binary.Write(&buf, binary.BigEndian, h.GasUsed)        // uint64 -> 8 bytes
	binary.Write(&buf, binary.BigEndian, int64(h.Height))  // int -> 8 bytes
	binary.Write(&buf, binary.BigEndian, h.Index)          // uint64 -> 8 bytes
	buf.Write(h.Node.Bytes())                              // Address -> 20 bytes
	binary.Write(&buf, binary.BigEndian, int64(h.ChainId)) // int -> 8 bytes
	buf.Write(h.PrevHash.Bytes())                          // Hash -> 32 bytes
	buf.Write(h.Root.Bytes())                              // Hash -> 32 bytes
	binary.Write(&buf, binary.BigEndian, int64(h.Size))    // int -> 8 bytes
	binary.Write(&buf, binary.BigEndian, h.Timestamp)      // uint64 -> 8 bytes
	buf.Write(h.V[:])                                      // [8]byte -> 8 bytes
	binary.Write(&buf, binary.BigEndian, h.Nonce)          // uint64 -> 8 bytes

	return buf.Bytes()
}

type Block struct {
	Head          *Header              `json:"header" gencodec:"required"` //
	Transactions  []types.GTransaction `json:"transactions" gencodec:"required"`
	Confirmations int                  `json:"confirmations" gencodec:"required"`
	Hash          common.Hash          `json:"hash" gencodec:"required"`
}

type UnconfirmedBlock struct {
	Nonce        int                   `json:"nonce" gencodec:"required"`
	Head         *Header               `json:"header" gencodec:"required"`
	Transactions []*types.GTransaction //`json:"transactions" gencodec:"required"`
}

func (b Block) CalculateHash() ([]byte, error) {
	hw, err := blake2b.New256(nil)
	if err != nil {
		return nil, err
	}
	if _, err := hw.Write(b.ToBytes()); err != nil {
		return nil, err
	}
	return hw.Sum(nil), nil
}

func (b Block) Equals(other trie.Content) (bool, error) {
	otherTC, ok := other.(Block)
	if !ok {
		return false, errors.New("value is not of type Block")
	}
	return b.Head.ChainId == otherTC.Head.ChainId &&
		b.Head.Height == otherTC.Head.Height, nil
}

type BlockReader interface {
}

func NewBlock(header *Header) *Block {
	b := &Block{
		Head:         CopyHeader(header),
		Transactions: make([]types.GTransaction, 0),
	}
	return b
}

func NewBlockWithHeaderAndHash(header *Header) *Block {
	b := &Block{
		Head:         CopyHeader(header),
		Transactions: make([]types.GTransaction, 0),
	}
	b.Hash = CrvBlockHash(*b)
	return b
}

func NewBlockWithHeader(header *Header) *Block {
	return &Block{
		Head:         CopyHeader(header),
		Transactions: make([]types.GTransaction, 0),
	}
}

func (b *Block) Header() *Header {
	if b == nil || b.Head == nil {
		return nil
	}
	// return CopyHeader(b.Head)
	return b.Head
}

// Function for compare block headers, may be deprecated later.
func (b *Block) EqHead(other *Header) bool {
	if b == nil || b.Head == nil || other == nil {
		return false
	}
	return b.Head.Ctx == other.Ctx &&
		b.Head.Height == other.Height &&
		b.Head.Difficulty == other.Difficulty &&
		b.Head.GasUsed == other.GasUsed &&
		b.Head.GasLimit == other.GasLimit &&
		b.Head.ChainId == other.ChainId &&
		len(b.Head.Extra) == len(other.Extra) &&
		b.Head.Root == other.Root &&
		b.Head.PrevHash == other.PrevHash &&
		b.Head.Timestamp == other.Timestamp &&
		b.Head.Size == other.Size &&
		b.Head.V == other.V
}

func CopyHeader(h *Header) *Header {
	if h == nil {
		return nil
	}
	cpy := *h
	cpy.Difficulty = h.Difficulty
	cpy.Extra = h.Extra
	cpy.ChainId = h.ChainId
	cpy.Root = h.Root
	cpy.Ctx = h.Ctx
	cpy.GasLimit = h.GasLimit
	cpy.GasUsed = h.GasUsed
	cpy.Height = h.Height
	cpy.Root = h.Root
	cpy.Timestamp = h.Timestamp
	cpy.Size = h.Size
	cpy.Difficulty = h.Difficulty
	cpy.Node = h.Node
	cpy.PrevHash = h.PrevHash
	cpy.Index = h.Index
	cpy.V = h.V
	return &cpy
}

func CalculateSize(b Block) int {
	totalSize := 0

	// Header
	if b.Head != nil {
		totalSize += headerSize()
	}

	// Транзакции
	for _, tx := range b.Transactions {
		totalSize += txSize(tx) // Реализуйте в зависимости от типа tx
	}

	// Hash
	totalSize += len(b.Hash.Bytes())

	// Confirmations
	totalSize += 8

	return totalSize
}

func headerSize() int {
	return 4 + 8 + 8 + 8 + 8 + 8 + 8 + address.AddressSize +
		8 + common.HashSize + common.HashSize + 8 + 8 + 8 + 8
}

func txSize(tx types.GTransaction) int {

	size := 0

	// Основные поля
	size += 8 // nonce (uint64)
	size += 8 // gas (uint64)
	size += 8 // timestamp (int64)

	// big.Int поля (газпрайс, значение, V, R, S)
	size += tx.GasPrice().BitLen()/8 + 1
	size += tx.Value().BitLen()/8 + 1

	v, r, s := tx.RawSignatureValues()
	size += v.BitLen()/8 + 1
	size += r.BitLen()/8 + 1
	size += s.BitLen()/8 + 1

	// data
	size += len(tx.Data())

	// dna
	size += len(tx.Dna())

	// to (Address)
	if to := tx.To(); to != nil {
		size += len(to.Bytes())
	}

	// тип транзакции (1 байт)
	size += 1

	return size
}

func GenerateGenesis(nodeAddress address.Address) *Block {
	var genesisHeader = &Header{
		Ctx:        17,
		Difficulty: 11111111111,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     0,
		Index:      0,
		GasLimit:   250000,
		GasUsed:    1,
		ChainId:    11,
		Node:       nodeAddress,
		Size:       0,
		// V:          "ALPHA-0.0.1",
		Nonce: 1,
	}

	var genesisBlock = &Block{
		Head: genesisHeader,
	}
	genesisBlock.Transactions = []types.GTransaction{}
	// Используем ToBytes() вместо JSON для вычисления размера
	blockBytes := genesisBlock.ToBytes()
	genesisBlock.Head.Size = len(blockBytes)
	return genesisBlock
}

// ToBytes serialises the block to a deterministic byte slice.
// It is a pure function: it does NOT mutate any field on b.
// Callers that need Head.Size to reflect the serialised length must update it
// explicitly:
//
//	b.Head.Size = len(b.ToBytes())
func (b *Block) ToBytes() []byte {
	var buf bytes.Buffer

	if b.Head != nil {
		buf.Write(b.Head.Bytes())
	}

	binary.Write(&buf, binary.BigEndian, uint32(len(b.Transactions)))

	for _, tx := range b.Transactions {
		buf.Write(tx.Hash().Bytes())
	}

	binary.Write(&buf, binary.BigEndian, int32(b.Confirmations))

	return buf.Bytes()
}

// get nonce as [8]byte from header
func (b *Block) GetNonceBytes() []byte {
	bts := make([]byte, 8)
	binary.BigEndian.PutUint64(bts, uint64(b.Head.Nonce))
	return bts
}

// get nonce as [4]byte from header
func (b *Block) SetNonceBytes(newNonce []byte) {
	bts := make([]byte, 8)
	copy(bts[:], newNonce[:])
	b.Head.Nonce = binary.BigEndian.Uint64(bts)
}

func FromBytes(b []byte) (*Block, error) {
	var blockData *Block
	err := json.Unmarshal(b, &blockData)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	return blockData, nil
}

// ???
func (Block) Read(p []byte) (n int, err error) {
	return 1, nil
}

type Blocks []*Block

func CrvBlockHash(block Block) (h common.Hash) {
	hw, _ := blake2b.New256(nil)

	var data = make([]byte, 0)
	for _, v := range block.Transactions {
		data = append(data, v.Hash().Bytes()...)
	}
	data = append(data, block.Head.Bytes()...)
	hw.Write(data)

	h.SetBytes(hw.Sum(nil))
	return h
}

func CrvHeaderHash(header Header) (h common.Hash) {
	buf := new(bytes.Buffer)

	// Записываем все поля последовательно
	binary.Write(buf, binary.BigEndian, header.Ctx)            // 4 bytes
	binary.Write(buf, binary.BigEndian, header.Difficulty)     // 8 bytes
	buf.Write(header.Extra[:])                                 // 8 bytes
	binary.Write(buf, binary.BigEndian, header.GasLimit)       // 8 bytes
	binary.Write(buf, binary.BigEndian, header.GasUsed)        // 8 bytes
	binary.Write(buf, binary.BigEndian, int64(header.Height))  // 8 bytes
	binary.Write(buf, binary.BigEndian, header.Index)          // 8 bytes
	buf.Write(header.Node.Bytes())                             // 20 bytes
	binary.Write(buf, binary.BigEndian, int64(header.ChainId)) // 8 bytes
	buf.Write(header.PrevHash.Bytes())                         // 32 bytes
	buf.Write(header.Root.Bytes())                             // 32 bytes
	binary.Write(buf, binary.BigEndian, header.Timestamp)      // 8 bytes
	buf.Write(header.V[:])                                     // 8 bytes
	binary.Write(buf, binary.BigEndian, header.Nonce)          // 8 bytes

	// Вычисляем хеш
	hash := blake2b.Sum256(buf.Bytes())
	h.SetBytes(hash[:])
	return h
}

// HASH METHODS
func (b *Block) GetHash() common.Hash {
	return b.Hash
}

var EmptyBlock = Block{}
