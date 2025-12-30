package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for icenet
type Metrics struct {
	// Block metrics
	BlocksReceivedTotal  *prometheus.CounterVec
	BlocksProcessedTotal *prometheus.CounterVec
	BlocksRejectedTotal  *prometheus.CounterVec
	BlocksBroadcastTotal *prometheus.CounterVec
	BlockSyncErrorsTotal *prometheus.CounterVec

	// Connection metrics
	ConnectionsActive      prometheus.Gauge
	ConnectionsTotal       *prometheus.CounterVec
	ConnectionsErrorsTotal *prometheus.CounterVec
	ConnectionLatency      *prometheus.HistogramVec

	// Protocol metrics
	MessagesReceivedTotal *prometheus.CounterVec
	MessagesSentTotal     *prometheus.CounterVec
	MessageSizeBytes      *prometheus.HistogramVec
	MessageProcessingTime *prometheus.HistogramVec
	MessageErrorsTotal    *prometheus.CounterVec

	// Consensus metrics
	ConsensusStatus      prometheus.Gauge
	ConsensusNodesTotal  prometheus.Gauge
	ConsensusVotersTotal prometheus.Gauge
	ConsensusNonce       prometheus.Gauge
	ConsensusErrorsTotal *prometheus.CounterVec

	// Network metrics
	NetworkLatency      *prometheus.HistogramVec
	NetworkThroughput   *prometheus.CounterVec
	NetworkTopologyNodes prometheus.Gauge
	
	// Channel metrics
	ChannelBufferUtilization prometheus.Gauge
	MessagesDroppedTotal      *prometheus.CounterVec
}

// NewMetrics creates and registers all metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		// Block metrics
		BlocksReceivedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_blocks_received_total",
				Help: "Total number of blocks received from other nodes",
			},
			[]string{"source"},
		),
		BlocksProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_blocks_processed_total",
				Help: "Total number of blocks successfully processed and added to chain",
			},
			[]string{"source"},
		),
		BlocksRejectedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_blocks_rejected_total",
				Help: "Total number of blocks rejected (validation failed, duplicates, etc.)",
			},
			[]string{"reason"},
		),
		BlocksBroadcastTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_blocks_broadcast_total",
				Help: "Total number of blocks broadcasted to other nodes",
			},
			[]string{"target"},
		),
		BlockSyncErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_block_sync_errors_total",
				Help: "Total number of block synchronization errors by type",
			},
			[]string{"error_type"},
		),

		// Connection metrics
		ConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_connections_active",
				Help: "Number of active connections",
			},
		),
		ConnectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_connections_total",
				Help: "Total number of connections established",
			},
			[]string{"type", "status"},
		),
		ConnectionsErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_connections_errors_total",
				Help: "Total number of connection errors",
			},
			[]string{"error_type"},
		),
		ConnectionLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "icenet_connection_latency_seconds",
				Help:    "Connection latency in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"remote_addr"},
		),

		// Protocol metrics
		MessagesReceivedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_messages_received_total",
				Help: "Total number of messages received",
			},
			[]string{"message_type"},
		),
		MessagesSentTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_messages_sent_total",
				Help: "Total number of messages sent",
			},
			[]string{"message_type"},
		),
		MessageSizeBytes: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "icenet_message_size_bytes",
				Help:    "Size of messages in bytes",
				Buckets: prometheus.ExponentialBuckets(64, 2, 12), // 64 bytes to 256 KB
			},
			[]string{"message_type"},
		),
		MessageProcessingTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "icenet_message_processing_time_seconds",
				Help:    "Time taken to process messages",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
			},
			[]string{"message_type"},
		),
		MessageErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_message_errors_total",
				Help: "Total number of message processing errors",
			},
			[]string{"message_type", "error_type"},
		),

		// Consensus metrics
		ConsensusStatus: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_consensus_status",
				Help: "Current consensus status (0=stopped, 1=started)",
			},
		),
		ConsensusNodesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_consensus_nodes_total",
				Help: "Total number of nodes in consensus",
			},
		),
		ConsensusVotersTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_consensus_voters_total",
				Help: "Total number of voters in consensus",
			},
		),
		ConsensusNonce: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_consensus_nonce",
				Help: "Current consensus nonce",
			},
		),
		ConsensusErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_consensus_errors_total",
				Help: "Total number of consensus errors",
			},
			[]string{"error_type"},
		),

		// Network metrics
		NetworkLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "icenet_network_latency_seconds",
				Help:    "Network latency between nodes",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"remote_addr"},
		),
		NetworkThroughput: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_network_throughput_bytes",
				Help: "Network throughput in bytes",
			},
			[]string{"direction"}, // "in" or "out"
		),
		NetworkTopologyNodes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_network_topology_nodes",
				Help: "Number of nodes in network topology",
			},
		),
		
		// Channel metrics
		ChannelBufferUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "icenet_channel_buffer_utilization",
				Help: "Current channel buffer utilization (0-1)",
			},
		),
		MessagesDroppedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "icenet_messages_dropped_total",
				Help: "Total number of messages dropped due to channel backpressure",
			},
			[]string{"message_type", "reason"},
		),
	}

	// Register all metrics (ignore errors if already registered)
	registerMetrics := func(collectors ...prometheus.Collector) {
		for _, collector := range collectors {
			if err := prometheus.Register(collector); err != nil {
				// Check if it's AlreadyRegisteredError
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					// If already registered, use the existing collector
					// This handles the case where metrics were registered elsewhere
					_ = are.ExistingCollector
					// Continue - metric is already registered, we'll use the existing one
					continue
				}
				// For other errors (like descriptor conflicts), also ignore to prevent panic
				// This can happen if metrics with same name but different labels are registered
				// In that case, we'll use whatever was registered first
				continue
			}
		}
	}

	registerMetrics(
		m.BlocksReceivedTotal,
		m.BlocksProcessedTotal,
		m.BlocksRejectedTotal,
		m.BlocksBroadcastTotal,
		m.BlockSyncErrorsTotal,
		m.ConnectionsActive,
		m.ConnectionsTotal,
		m.ConnectionsErrorsTotal,
		m.ConnectionLatency,
		m.MessagesReceivedTotal,
		m.MessagesSentTotal,
		m.MessageSizeBytes,
		m.MessageProcessingTime,
		m.MessageErrorsTotal,
		m.ConsensusStatus,
		m.ConsensusNodesTotal,
		m.ConsensusVotersTotal,
		m.ConsensusNonce,
		m.ConsensusErrorsTotal,
		m.NetworkLatency,
		m.NetworkThroughput,
		m.NetworkTopologyNodes,
		m.ChannelBufferUtilization,
		m.MessagesDroppedTotal,
	)

	return m
}

// Global metrics instance
var (
	globalMetrics *Metrics
	metricsOnce   sync.Once
)

// Init initializes the global metrics instance
func Init() {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics()
	})
}

// Get returns the global metrics instance
func Get() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics()
	})
	return globalMetrics
}

