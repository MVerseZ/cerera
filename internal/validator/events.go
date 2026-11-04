package validator

import "github.com/cerera/core/events"

// Handle implements observer.Subscriber for node event bus wiring.
func (v *CoreValidator) Handle(ev events.Event) error {
	if ev.Kind != events.ChainReorged {
		return nil
	}
	return v.ReplayChain(ev.Blocks)
}
