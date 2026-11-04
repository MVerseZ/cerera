package miner

import (
	"github.com/cerera/core/events"
	"github.com/cerera/core/types"
)

// Handle implements observer.Subscriber for node event bus wiring.
func (m *miner) Handle(ev events.Event) error {
	switch ev.Kind {
	case events.ChainReorged, events.ChainHeadUpdated:
		m.onChainHeadUpdated(ev.Block)
	case events.TxAccepted:
		if ev.Tx != nil {
			m.onTxAccepted(ev.Tx)
		}
	}
	return nil
}

// EventSubscriber handles node lifecycle events (see internal/observer.Subscriber).
type EventSubscriber interface {
	Handle(events.Event) error
}

// AsSubscriber returns m as an event bus subscriber.
func AsSubscriber(m Miner) EventSubscriber {
	if x, ok := m.(*miner); ok {
		return x
	}
	return nil
}

func (m *miner) onTxAccepted(tx *types.GTransaction) {
	if tx == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.mining || m.worker == nil {
		return
	}
	txs := m.pool.GetMiningPackage(m.pool.GetInfo().Size)
	minerLogger().Infow("[MINER] New pool tx, restarting miner", "tx", tx.Hash(), "txs", len(txs))
	m.worker.Compute(m.newTask(txs))
}
