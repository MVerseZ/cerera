package types

import "log"

type EventFunnel struct {
	logger log.Logger
}

func (ef *EventFunnel) Add(l string, topic string) {

}

func (ef *EventFunnel) Remove(l string) {

}

func (ef *EventFunnel) Notify(msg string, topic string) {

}

func (ef *EventFunnel) Clear() {

}
