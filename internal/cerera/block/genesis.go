package block

import (
	"math/big"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/types"
)

func Genesis(chainId *big.Int) Block {
	var genesisHeader = &Header{
		Ctx:        17,
		Difficulty: 111111111111111,
		Extra:      []byte("GENESYS BLOCK VAVILOV PROTOCOL"),
		Height:     0,
		Timestamp:  uint64(time.Now().UnixMilli()),
		GasLimit:   250000,
		GasUsed:    249999,
		ChainId:    chainId,
		Node:       types.EmptyAddress(),
		Size:       0,
		V:          "ALPHA-0.0.1",
		Nonce:      5377,
	}

	// genesisHeader.HashH = rlpHeaderHash(*genesisHeader)
	var genesisBlock = Block{
		Head: genesisHeader,
	}
	// genesisBlock.HashB = rlpBlockHash(*genesisBlock)
	genesisBlock.Transactions = []*types.GTransaction{}
	//make([]common.Hash, 0)
	var finalSize = unsafe.Sizeof(genesisBlock)
	genesisBlock.Head.Size = int(finalSize)
	return genesisBlock
}
