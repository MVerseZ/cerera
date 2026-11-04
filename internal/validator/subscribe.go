package validator

import "github.com/cerera/internal/observer"

// AsSubscriber returns v as an event bus subscriber.
func AsSubscriber(v Validator) observer.Subscriber {
	if c, ok := v.(*CoreValidator); ok {
		return c
	}
	return nil
}
