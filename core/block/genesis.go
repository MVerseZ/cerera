package block

import (
	"time"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
)

func GenesisHead(chainId int) *Header {
	var genesisHeader = &Header{
		Ctx:        17,
		Difficulty: uint64(11111111), //uint64(111111111111111), // 4 trailng zeros
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     0,
		Index:      0,
		Timestamp:  uint64(time.Date(2022, 6, 24, 0, 0, 0, 0, time.UTC).UnixMilli()),
		GasLimit:   250000,
		GasUsed:    0,
		ChainId:    chainId,
		Node:       address.EmptyAddress(),
		PrevHash:   common.EmptyHash(),
		Root:       common.EmptyHash(),
		Size:       0,
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      20122872,
	}
	return genesisHeader
}
