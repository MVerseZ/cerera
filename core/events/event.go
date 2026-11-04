package events

import (
	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
)

type Kind int

const (
	ChainReorged Kind = iota
	ChainHeadUpdated
	TxAccepted
	AccountCreated
)

type HeadSource uint8

const (
	HeadFinalized HeadSource = iota
	HeadSynced
)

type Event struct {
	Kind         Kind
	Block        *block.Block
	Blocks       []*block.Block
	Tx           *types.GTransaction
	AccountBytes []byte
	HeadSource   HeadSource
}
