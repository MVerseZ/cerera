package gigea

import (
	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
)

// PeerID is an abstract identifier of a remote peer in the network.
// Concrete transport implementations (e.g. libp2p) can map their own
// peer identity type to this representation.
type PeerID string

// Network defines the minimal transport interface that the node
// logic (consensus, sync, etc.) expects from the networking layer.
//
// It intentionally hides concrete transport details (libp2p host,
// streams, topics) and works only with domain types and opaque PeerID.
type Network interface {
	// BroadcastBlock broadcasts a full block to the network.
	BroadcastBlock(b *block.Block) error

	// BroadcastTx broadcasts a transaction to the network.
	BroadcastTx(tx *types.GTransaction) error

	// BroadcastConsensus broadcasts a consensus message that is already
	// serialized by the consensus layer.
	BroadcastConsensus(msgType int, data []byte, signature []byte) error

	// OnBlock registers a handler for received blocks.
	OnBlock(handler func(b *block.Block, from PeerID))

	// OnTx registers a handler for received transactions.
	OnTx(handler func(tx *types.GTransaction, from PeerID))

	// OnConsensus registers a handler for received consensus messages.
	OnConsensus(handler func(msgType int, data []byte, from PeerID))

	// PeerCount returns the current number of connected peers known
	// to the transport layer.
	PeerCount() int
}

