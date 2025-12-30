package metrics

import (
	"time"
)

// RecordNetworkLatency records network latency
func (m *Metrics) RecordNetworkLatency(remoteAddr string, latency time.Duration) {
	m.NetworkLatency.WithLabelValues(remoteAddr).Observe(latency.Seconds())
}

// RecordNetworkThroughput records network throughput
func (m *Metrics) RecordNetworkThroughput(direction string, bytes int64) {
	m.NetworkThroughput.WithLabelValues(direction).Add(float64(bytes))
}

// UpdateNetworkTopologyNodes updates the number of nodes in network topology
func (m *Metrics) UpdateNetworkTopologyNodes(count int) {
	m.NetworkTopologyNodes.Set(float64(count))
}

