package metrics

import (
	"time"

	"github.com/cerera/internal/icenet/protocol"
)

// RecordMessageReceived records a received message
func (m *Metrics) RecordMessageReceived(msgType protocol.MessageType, size int) {
	m.MessagesReceivedTotal.WithLabelValues(string(msgType)).Inc()
	m.MessageSizeBytes.WithLabelValues(string(msgType)).Observe(float64(size))
}

// RecordMessageSent records a sent message
func (m *Metrics) RecordMessageSent(msgType protocol.MessageType, size int) {
	m.MessagesSentTotal.WithLabelValues(string(msgType)).Inc()
	m.MessageSizeBytes.WithLabelValues(string(msgType)).Observe(float64(size))
}

// RecordMessageProcessingTime records message processing time
func (m *Metrics) RecordMessageProcessingTime(msgType protocol.MessageType, duration time.Duration) {
	m.MessageProcessingTime.WithLabelValues(string(msgType)).Observe(duration.Seconds())
}

// RecordMessageError records a message processing error
func (m *Metrics) RecordMessageError(msgType protocol.MessageType, errorType string) {
	m.MessageErrorsTotal.WithLabelValues(string(msgType), errorType).Inc()
}

