package gigea

import (
	"context"

	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
	"github.com/cerera/icenet/consensus"
	"github.com/cerera/internal/service"
)

// ConsensusManager is a thin wrapper that owns the consensus engine for a node.
// It lives in the gigea layer and wires together:
//   - ServiceProvider (state / chain access)
//   - Network (abstract transport)
//   - underlying consensus implementation (currently icenet/consensus.Manager)
//
// This allows us to keep icenet focused on transport while the node logic
// lives in gigea.
type ConsensusManager struct {
	engine  *consensus.Manager
	net     Network
	service service.ServiceProvider
}

// NewConsensusManager constructs a consensus manager bound to the given
// transport and service provider.
func NewConsensusManager(
	ctx context.Context,
	net Network,
	localAddr types.Address,
	serviceProvider service.ServiceProvider,
) *ConsensusManager {
	// The underlying engine still needs access to the libp2p host and
	// peer manager; these are provided indirectly by the transport layer
	// where needed. For now we rely on the existing constructor and
	// service provider wiring.
	//
	// Note: we do not create the engine here because it requires concrete
	// libp2p types; instead, the caller passes in a pre-wired engine.
	return &ConsensusManager{
		engine:  nil,
		net:     net,
		service: serviceProvider,
	}
}

// AttachEngine allows wiring an existing icenet/consensus.Manager instance
// into the gigea-level manager. This is an intermediate step while we
// gradually migrate the consensus implementation fully into gigea.
func (m *ConsensusManager) AttachEngine(engine *consensus.Manager) {
	m.engine = engine
	if engine != nil {
		engine.SetServiceProvider(m.service)
	}
}

// ProposeBlock proposes a block for consensus via the underlying engine.
func (m *ConsensusManager) ProposeBlock(b *block.Block) error {
	if m == nil || m.engine == nil {
		return nil
	}
	return m.engine.ProposeBlock(b)
}

