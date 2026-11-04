package observer

import (
	"sync"

	"github.com/cerera/core/events"
	"github.com/cerera/internal/logger"
)

var busLog = logger.Named("observer")

// Bus dispatches events to subscribers synchronously in subscription order.
type Bus struct {
	mu   sync.RWMutex
	subs []Subscriber
}

func NewBus() *Bus {
	return &Bus{subs: make([]Subscriber, 0)}
}

func (b *Bus) Subscribe(s Subscriber) {
	if b == nil || s == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, s)
}

// Publish notifies all subscribers. Returns the first error and stops dispatch.
func (b *Bus) Publish(ev events.Event) error {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	subs := append([]Subscriber(nil), b.subs...)
	b.mu.RUnlock()

	var firstErr error
	for _, s := range subs {
		if err := s.Handle(ev); err != nil {
			busLog.Errorw("event subscriber failed", "kind", ev.Kind, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
