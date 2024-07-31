package block

import (
	"math/big"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/types"
)

func Genesis() Block {
	var genesisHeader = &Header{
		Ctx:           17,
		Difficulty:    big.NewInt(11111111111),
		Extra:         []byte("GENESYS BLOCK VAVILOV PROTOCOL"),
		Height:        0,
		Timestamp:     uint64(time.Now().UnixMilli()),
		GasLimit:      250000,
		GasUsed:       249999,
		Number:        big.NewInt(0),
		Confirmations: 1,
		Node:          types.EmptyAddress(),
		Size:          0,
		V:             "ALPHA-0.0.1",
	}

	// genesisHeader.HashH = rlpHeaderHash(*genesisHeader)
	var genesisBlock = Block{
		Head:  genesisHeader,
		Nonce: 11,
	}
	// genesisBlock.HashB = rlpBlockHash(*genesisBlock)
	genesisBlock.Transactions = []types.GTransaction{}
	//make([]common.Hash, 0)
	var finalSize = unsafe.Sizeof(genesisBlock)
	genesisBlock.Head.Size = int(finalSize)
	return genesisBlock
}
