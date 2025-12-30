package metrics

import (
	"time"
)

// RecordConnectionEstablished records a new connection
func (m *Metrics) RecordConnectionEstablished(connType string, status string) {
	m.ConnectionsTotal.WithLabelValues(connType, status).Inc()
	m.ConnectionsActive.Inc()
}

// RecordConnectionClosed records a closed connection
func (m *Metrics) RecordConnectionClosed(connType string) {
	m.ConnectionsActive.Dec()
}

// RecordConnectionError records a connection error
func (m *Metrics) RecordConnectionError(errorType string) {
	m.ConnectionsErrorsTotal.WithLabelValues(errorType).Inc()
}

// RecordConnectionLatency records connection latency
func (m *Metrics) RecordConnectionLatency(remoteAddr string, latency time.Duration) {
	m.ConnectionLatency.WithLabelValues(remoteAddr).Observe(latency.Seconds())
}

// UpdateActiveConnections updates the active connections gauge
func (m *Metrics) UpdateActiveConnections(count int) {
	m.ConnectionsActive.Set(float64(count))
}

// UpdateChannelBufferUtilization updates the channel buffer utilization
func (m *Metrics) UpdateChannelBufferUtilization(utilization float64) {
	m.ChannelBufferUtilization.Set(utilization)
}

// RecordMessageDropped records a dropped message
func (m *Metrics) RecordMessageDropped(msgType string, reason string) {
	m.MessagesDroppedTotal.WithLabelValues(msgType, reason).Inc()
}

