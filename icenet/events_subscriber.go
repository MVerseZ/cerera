package icenet

import (
	"github.com/cerera/core/events"
	"github.com/cerera/icenet/metrics"
)

// EventSubscriber reacts to node lifecycle events for P2P metrics and gossip.
type EventSubscriber struct {
	ice *Ice
}

func NewEventSubscriber(ice *Ice) *EventSubscriber {
	return &EventSubscriber{ice: ice}
}

func (s *EventSubscriber) Handle(ev events.Event) error {
	if s == nil || s.ice == nil {
		return nil
	}
	switch ev.Kind {
	case events.ChainHeadUpdated:
		if ev.Block != nil && ev.Block.Head != nil {
			metrics.SetBlockHeight(ev.Block.Head.Height)
		}
		if ev.HeadSource == events.HeadFinalized && ev.Block != nil && s.ice.PubSub != nil {
			if err := s.ice.BroadcastBlock(ev.Block); err != nil {
				iceLogger().Warnw("BroadcastBlock after finalize failed", "err", err)
			}
		}
	case events.AccountCreated:
		if len(ev.AccountBytes) > 0 && s.ice.PubSub != nil {
			if err := s.ice.PubSub.BroadcastAccountState(ev.AccountBytes); err != nil {
				iceLogger().Warnw("BroadcastAccountState failed", "err", err)
			}
		}
	}
	return nil
}
