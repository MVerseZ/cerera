package block

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"unsafe"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"
	"golang.org/x/crypto/blake2b"
)

// Header represents a block header in the blockchain.
type Header struct {
	Ctx        int32         `json:"ctx" gencodec:"required"`
	Difficulty *big.Int      `json:"difficulty"       gencodec:"required"`
	Extra      []byte        `json:"extraData"        gencodec:"required"`
	GasLimit   uint64        `json:"gasLimit"         gencodec:"required"`
	GasUsed    uint64        `json:"gasUsed"          gencodec:"required"`
	Height     int           `json:"height" gencodec:"required"`
	Index      uint64        `json:"index" gencodec:"required"`
	Node       types.Address `json:"node" gencodec:"required"`
	Number     *big.Int      `json:"number"           gencodec:"required"`
	PrevHash   common.Hash   `json:"prevHash" gencodec:"required"`
	Root       common.Hash   `json:"stateRoot"        gencodec:"required"`
	Size       int           `json:"size" gencodec:"required"`
	Timestamp  uint64        `json:"timestamp"        gencodec:"required"`
	V          string        `json:"version"        gencodec:"required"`
}

func (h *Header) Bytes() []byte {
	var b = make([]byte, 0)
	b = append(b, h.Difficulty.Bytes()...)
	b = append(b, h.Extra...)
	b = append(b, byte(h.GasLimit))
	b = append(b, byte(h.GasUsed))
	b = append(b, byte(h.Index))
	b = append(b, h.Number.Bytes()...)
	b = append(b, h.PrevHash.Bytes()...)
	b = append(b, h.Root.Bytes()...)
	b = append(b, byte(h.Size))
	b = append(b, byte(h.Timestamp))
	b = append(b, []byte(h.V)...)
	return b
}

type Block struct {
	Nonce         int                   `json:"nonce" gencodec:"required"`
	Head          *Header               `json:"header" gencodec:"required"`
	Transactions  []*types.GTransaction `json:"transactions," gencodec:"required"`
	Confirmations int                   `json:"confirmations," gencodec:"required"`
}

type UnconfirmedBlock struct {
	Nonce        int                   `json:"nonce" gencodec:"required"`
	Head         *Header               `json:"header" gencodec:"required"`
	Transactions []*types.GTransaction //`json:"transactions" gencodec:"required"`
}

func (b Block) CalculateHash() ([]byte, error) {
	h := sha256.New()
	if _, err := h.Write(b.ToBytes()); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func (b Block) Equals(other trie.Content) (bool, error) {
	otherTC, ok := other.(Block)
	if !ok {
		return false, errors.New("value is not of type Block")
	}
	return b.Head.Number.Cmp(otherTC.Head.Number) == 0, nil
}

type BlockReader interface {
}

func NewBlock(header *Header) *Block {
	b := &Block{
		Head:         CopyHeader(header),
		Transactions: make([]*types.GTransaction, 0),
	}
	return b
}

func NewBlockWithHeader(header *Header) *Block {
	return &Block{
		Head:         CopyHeader(header),
		Transactions: make([]*types.GTransaction, 0),
	}
}

func (b *Block) Header() *Header { return CopyHeader(b.Head) }

// Function for compare block headers, may be deprecated later.
func (b *Block) EqHead(other *Header) bool {
	return b.Head.Ctx == other.Ctx &&
		b.Head.Height == other.Height &&
		b.Head.Difficulty.Cmp(other.Difficulty) == 0 &&
		b.Head.GasUsed == other.GasUsed &&
		b.Head.GasLimit == other.GasLimit &&
		b.Head.Number.Cmp(other.Number) == 0 &&
		len(b.Head.Extra) == len(other.Extra) &&
		b.Head.Root == other.Root &&
		b.Head.PrevHash == other.PrevHash &&
		b.Head.Timestamp == other.Timestamp &&
		b.Head.Size == other.Size &&
		b.Head.V == other.V
}

func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
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
	var txSize = 0
	if len(b.Transactions) > 0 {
		for _, tx := range b.Transactions {
			txSize += int(unsafe.Sizeof(tx))
		}
	}
	return txSize + int(unsafe.Sizeof(b))
}

func GenerateGenesis(nodeAddress types.Address) *Block {
	var genesisHeader = &Header{
		Ctx:        17,
		Difficulty: big.NewInt(11111111111),
		Extra:      []byte("GENESYS BLOCK VAVILOV PROTOCOL"),
		Height:     0,
		GasLimit:   250000,
		GasUsed:    1,
		Number:     big.NewInt(0),
		Node:       nodeAddress,
		Size:       0,
		V:          "ALPHA-0.0.1",
	}

	// genesisHeader.HashH = rlpHeaderHash(*genesisHeader)
	var genesisBlock = &Block{
		Head:  genesisHeader,
		Nonce: 11,
	}
	// genesisBlock.HashB = rlpBlockHash(*genesisBlock)
	genesisBlock.Transactions = []*types.GTransaction{}
	//make([]common.Hash, 0)
	var finalSize = unsafe.Sizeof(genesisBlock)
	genesisBlock.Head.Size = int(finalSize)
	return genesisBlock
}

func (b *Block) ToBytes() []byte {
	jsonBytes, err := json.Marshal(b)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}
	return jsonBytes
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

// func DetectGenesis(data []byte) (Block, error) {
// 	var genesisBlock Block

// 	var hash = common.BytesToHash(data)
// 	fmt.Println(hash)

// 	// common.Bytes.MarshalText(data)
// 	err := json.Unmarshal(data, &genesisBlock)
// 	if err != nil {
// 		return genesisBlock, fmt.Errorf("error detect genesis block:: %s", err)
// 	}
// 	return genesisBlock, nil
// }

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
	hw, _ := blake2b.New256(nil)
	hw.Write(header.Extra)
	hw.Write(header.Difficulty.Bytes())
	h.SetBytes(hw.Sum(nil))
	return h
}

// HASH METHODS
func (b *Block) Hash() common.Hash {
	return CrvBlockHash(*b)
}

func (h *Header) Hash() common.Hash {
	return CrvHeaderHash(*h)
}
