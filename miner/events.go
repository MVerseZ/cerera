package miner

import (
	"github.com/cerera/core/events"
	"github.com/cerera/core/types"
)

// Handle implements observer.Subscriber for node event bus wiring.
func (m *miner) Handle(ev events.Event) error {
	switch ev.Kind {
	case events.ChainReorged:
		m.onChainHeadUpdated(ev.Block)
	case events.ChainHeadUpdated:
		if ev.Block == nil {
			return nil
		}
		switch ev.HeadSource {
		case events.HeadSynced:
			// Sync updates chain only; refresh validator state without restarting PoW.
			m.applyValidatorForBlock(ev.Block)
		case events.HeadFinalized:
			m.onChainHeadUpdated(ev.Block)
		}
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
	if !m.mining || m.worker == nil || m.pool == nil {
		m.mu.Unlock()
		return
	}
	packageSize := m.pool.GetInfo().Size
	worker := m.worker
	m.mu.Unlock()

	txs := m.pool.GetMiningPackage(packageSize)
	minerLogger().Infow("[MINER] New pool tx, restarting miner", "tx", tx.Hash(), "txs", len(txs))
	if worker != nil {
		task := m.newTask(txs)
		if task != nil {
			worker.Compute(task)
		}
	}
}
