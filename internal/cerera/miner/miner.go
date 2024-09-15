package miner

import (
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// PROTOTYPE STRUCTURE

type Miner struct {
	difficulty int64
}

func CalculateBlockHash(b block.Block) {

}

func MineBlock(latest *block.Block, addr types.Address) {
	head := &block.Header{
		Ctx:        latest.Header().Ctx,
		Difficulty: latest.Header().Difficulty,
		Extra:      []byte("OP_AUTO_GEN_BLOCK_DAT"),
		Height:     latest.Header().Height + 1,
		Index:      latest.Header().Index + 1,
		Timestamp:  uint64(time.Now().UnixMilli()),
		Number:     big.NewInt(0).Add(latest.Header().Number, big.NewInt(1)),
		// PrevHash:      bc.info.Latest,
		Confirmations: 1,
		Node:          addr,
		Root:          latest.Header().Root,
		GasLimit:      latest.Head.GasLimit, // todo get gas limit dynamically
	}
}
