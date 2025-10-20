package block

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"
	"golang.org/x/crypto/blake2b"
)

// Header represents a block header in the blockchain.
type Header struct {
	Ctx        int32         `json:"ctx" gencodec:"required"`               // 4 bytes
	Difficulty uint64        `json:"difficulty"       gencodec:"required"`  // 8 bytes
	Extra      [8]byte       `json:"extraData"        gencodec:"required"`  // 8 bytes
	GasLimit   uint64        `json:"gasLimit"         gencodec:"required"`  // 8 bytes
	GasUsed    uint64        `json:"gasUsed"          gencodec:"required"`  // 8 bytes
	Height     int           `json:"height" gencodec:"required"`            // 8 bytes
	Index      uint64        `json:"index" gencodec:"required"`             // 8 bytes
	Node       types.Address `json:"node" gencodec:"required"`              // 20 bytes
	ChainId    int           `json:"chainId"           gencodec:"required"` // 8 bytes
	PrevHash   common.Hash   `json:"prevHash" gencodec:"required"`          // 32 bytes
	Root       common.Hash   `json:"stateRoot"        gencodec:"required"`  // 32 bytes
	Size       int           `json:"size" gencodec:"required"`              // 8 bytes
	Timestamp  uint64        `json:"timestamp"        gencodec:"required"`  // 8 bytes
	V          [8]byte       `json:"version"        gencodec:"required"`    // 8 bytes
	Nonce      uint64        `json:"nonce"        gencodec:"required"`      // 8 bytes
}

func (h *Header) Bytes() []byte {
	var b = make([]byte, 0)
	b = append(b, byte(h.Difficulty))
	b = append(b, h.Extra[:]...)
	b = append(b, byte(h.GasLimit))
	b = append(b, byte(h.GasUsed))
	b = append(b, byte(h.Index))
	b = append(b, byte(h.ChainId))
	b = append(b, h.PrevHash.Bytes()...)
	b = append(b, h.Root.Bytes()...)
	b = append(b, byte(h.Size))
	b = append(b, byte(h.Timestamp))
	b = append(b, h.V[:]...)
	return b
}

type Block struct {
	Head          *Header              `json:"header" gencodec:"required"` //
	Transactions  []types.GTransaction `json:"transactions," gencodec:"required"`
	Confirmations int                  `json:"confirmations," gencodec:"required"`
	Hash          common.Hash          `json:"hash," gencodec:"required"`
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

func (b *Block) Header() *Header { return CopyHeader(b.Head) }

// Function for compare block headers, may be deprecated later.
func (b *Block) EqHead(other *Header) bool {
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
	cpy := *h
	cpy.Difficulty = h.Difficulty
	// if len(h.Extra) > 0 {
	// 	cpy.Extra = make([]byte, len(h.Extra))
	// 	copy(cpy.Extra, h.Extra)
	// }
	cpy.Extra = h.Extra
	cpy.ChainId = h.ChainId
	// if cpy.ChainId = new(big.Int); h.ChainId != nil {
	// 	cpy.ChainId.Set(h.ChainId)
	// }
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

	// genesisHeader.HashH = rlpHeaderHash(*genesisHeader)
	var genesisBlock = &Block{
		Head: genesisHeader,
	}
	// genesisBlock.HashB = rlpBlockHash(*genesisBlock)
	genesisBlock.Transactions = []types.GTransaction{}
	//make([]common.Hash, 0)
	gs, _ := json.Marshal(genesisBlock)
	genesisBlock.Head.Size = len(gs)
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

// get nonce as [8]byte from header
func (b *Block) GetNonceBytes() []byte {
	bts := make([]byte, 8)
	binary.BigEndian.PutUint64(bts, uint64(b.Head.Nonce))
	return bts
}

func (b *Block) IncNonce() {
	b.Head.Nonce += 1
}

// get nonce as [4]byte from header
func (b *Block) SetNonceBytes(newNonce []byte) {
	bts := make([]byte, 8)
	copy(bts[:], newNonce[:])
	b.Head.Nonce = binary.BigEndian.Uint64(bts)
}

func (b *Block) UpdateHash() {
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
	// hw.Write(header.Extra)
	// hw.Write(hea)
	h.SetBytes(hw.Sum(nil))
	return h
}

// HASH METHODS
func (b *Block) GetHash() common.Hash {
	return b.Hash
}

// func (h *Header) Hash() common.Hash {
// 	return CrvHeaderHash(*h)
// }

var EmptyBlock = Block{}
