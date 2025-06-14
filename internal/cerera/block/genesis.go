package block

import (
	"time"

	"github.com/cerera/internal/cerera/types"
)

func GenesisHead(chainId int) *Header {
	var genesisHeader = &Header{
		Ctx:        17,
		Difficulty: 1111111111111111, //uint64(111111111111111), // 4 trailng zeros
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     0,
		Timestamp:  uint64(time.Now().UnixMilli()),
		GasLimit:   250000,
		GasUsed:    249999,
		ChainId:    chainId,
		Node:       types.EmptyAddress(),
		Size:       0,
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      5437,
	}
	return genesisHeader
}
