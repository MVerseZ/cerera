package icenet

import (
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
	"github.com/cerera/gigea"
	"github.com/libp2p/go-libp2p/core/peer"
)

// NetworkAdapter implements gigea.Network on top of the Ice transport.
// It is a thin wrapper that translates between libp2p-specific types
// and the abstract types expected by the node logic.
type NetworkAdapter struct {
	ice *Ice
}

// NewNetworkAdapter creates a new adapter bound to the given Ice instance.
func NewNetworkAdapter(ice *Ice) *NetworkAdapter {
	return &NetworkAdapter{ice: ice}
}

func (a *NetworkAdapter) BroadcastBlock(b *block.Block) error {
	if a.ice == nil {
		return fmt.Errorf("ice transport not initialized")
	}
	return a.ice.BroadcastBlock(b)
}

func (a *NetworkAdapter) BroadcastTx(tx *types.GTransaction) error {
	if a.ice == nil {
		return fmt.Errorf("ice transport not initialized")
	}
	return a.ice.BroadcastTx(tx)
}

func (a *NetworkAdapter) BroadcastConsensus(msgType int, data []byte, signature []byte) error {
	if a.ice == nil {
		return fmt.Errorf("ice transport not initialized")
	}
	// Reuse existing consensus broadcasting helper on Ice.
	return a.ice.broadcastConsensusMsg(msgType, data, signature)
}

func (a *NetworkAdapter) OnBlock(handler func(b *block.Block, from gigea.PeerID)) {
	if a.ice == nil || a.ice.PubSub == nil {
		return
	}
	a.ice.PubSub.SetOnBlock(func(b *block.Block, from peer.ID) {
		if handler != nil {
			handler(b, gigea.PeerID(from.String()))
		}
	})
}

func (a *NetworkAdapter) OnTx(handler func(tx *types.GTransaction, from gigea.PeerID)) {
	if a.ice == nil || a.ice.PubSub == nil {
		return
	}
	a.ice.PubSub.SetOnTx(func(tx *types.GTransaction, from peer.ID) {
		if handler != nil {
			handler(tx, gigea.PeerID(from.String()))
		}
	})
}

func (a *NetworkAdapter) OnConsensus(handler func(msgType int, data []byte, from gigea.PeerID)) {
	if a.ice == nil || a.ice.PubSub == nil {
		return
	}
	a.ice.PubSub.SetOnConsensus(func(msgType int, data []byte, from peer.ID) {
		if handler != nil {
			handler(msgType, data, gigea.PeerID(from.String()))
		}
	})
}

func (a *NetworkAdapter) PeerCount() int {
	if a.ice == nil {
		return 0
	}
	return a.ice.GetPeerCount()
}

