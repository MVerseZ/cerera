package events

// Publisher emits node lifecycle events synchronously.
type Publisher interface {
	Publish(Event) error
}
