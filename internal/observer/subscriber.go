package observer

import "github.com/cerera/core/events"

// Subscriber handles node lifecycle events from the bus.
type Subscriber interface {
	Handle(events.Event) error
}

// SubscriberFunc adapts a function to Subscriber.
type SubscriberFunc func(events.Event) error

func (f SubscriberFunc) Handle(ev events.Event) error { return f(ev) }
