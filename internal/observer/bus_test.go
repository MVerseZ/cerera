package observer

import (
	"errors"
	"testing"

	"github.com/cerera/core/events"
)

type recordingSub struct {
	kinds []events.Kind
	errOn events.Kind
}

func (r *recordingSub) Handle(ev events.Event) error {
	r.kinds = append(r.kinds, ev.Kind)
	if ev.Kind == r.errOn {
		return errors.New("fail")
	}
	return nil
}

func TestBusPublishOrder(t *testing.T) {
	bus := NewBus()
	a := &recordingSub{}
	b := &recordingSub{}
	bus.Subscribe(a)
	bus.Subscribe(b)

	if err := bus.Publish(events.Event{Kind: events.TxAccepted}); err != nil {
		t.Fatal(err)
	}
	if len(a.kinds) != 1 || len(b.kinds) != 1 {
		t.Fatalf("a=%v b=%v", a.kinds, b.kinds)
	}
}

func TestBusPublishStopsOnError(t *testing.T) {
	bus := NewBus()
	first := &recordingSub{}
	second := &recordingSub{errOn: events.ChainReorged}
	third := &recordingSub{}
	bus.Subscribe(first)
	bus.Subscribe(second)
	bus.Subscribe(third)

	err := bus.Publish(events.Event{Kind: events.ChainReorged})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(first.kinds) != 1 {
		t.Fatal("first should run")
	}
	if len(third.kinds) != 0 {
		t.Fatal("third should not run after error")
	}
}
