package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "icenet"
)

var (
	// Peer metrics
	PeersConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "peers_connected",
		Help:      "Number of currently connected peers",
	})

	PeersTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "peers_total",
		Help:      "Total number of peers connected since start",
	})

	PeersDisconnected = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "peers_disconnected_total",
		Help:      "Total number of peer disconnections",
	})

	PeersBanned = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "peers_banned_total",
		Help:      "Total number of peers banned",
	})

	// Block metrics
	BlocksReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_received_total",
		Help:      "Total number of blocks received from peers",
	})

	BlocksValidated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_validated_total",
		Help:      "Total number of blocks validated",
	})

	BlocksRejected = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_rejected_total",
		Help:      "Total number of blocks rejected",
	})

	BlocksBroadcast = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_broadcast_total",
		Help:      "Total number of blocks broadcast to the network",
	})

	BlockHeight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "block_height",
		Help:      "Current block height",
	})

	// Transaction metrics
	TxsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_received_total",
		Help:      "Total number of transactions received from peers",
	})

	TxsValidated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_validated_total",
		Help:      "Total number of transactions validated",
	})

	TxsRejected = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_rejected_total",
		Help:      "Total number of transactions rejected",
	})

	TxsBroadcast = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_broadcast_total",
		Help:      "Total number of transactions broadcast to the network",
	})

	// Sync metrics
	SyncProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "sync_progress",
		Help:      "Synchronization progress percentage (0-100)",
	})

	SyncBlocksRemaining = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "sync_blocks_remaining",
		Help:      "Number of blocks remaining to sync",
	})

	SyncActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "sync_active",
		Help:      "Whether sync is currently active (1) or not (0)",
	})

	SyncDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "sync_duration_seconds",
		Help:      "Duration of sync operations in seconds",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17 minutes
	})

	// Consensus metrics
	ConsensusRoundsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "consensus_rounds_total",
		Help:      "Total number of consensus rounds started",
	})

	ConsensusRoundsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "consensus_rounds_completed_total",
		Help:      "Total number of consensus rounds completed successfully",
	})

	ConsensusRoundsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "consensus_rounds_failed_total",
		Help:      "Total number of consensus rounds that failed",
	})

	ConsensusValidators = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "consensus_validators",
		Help:      "Number of active validators",
	})

	ConsensusPrepareVotes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "consensus_prepare_votes",
		Help:      "Number of prepare votes in current round",
	})

	ConsensusCommitVotes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "consensus_commit_votes",
		Help:      "Number of commit votes in current round",
	})

	// Network metrics
	MessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "messages_received_total",
		Help:      "Total number of messages received by type",
	}, []string{"type"})

	MessagesSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "messages_sent_total",
		Help:      "Total number of messages sent by type",
	}, []string{"type"})

	MessageLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "message_latency_seconds",
		Help:      "Latency of message processing in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
	}, []string{"type"})

	BytesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "bytes_received_total",
		Help:      "Total bytes received from network",
	})

	BytesSent = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "bytes_sent_total",
		Help:      "Total bytes sent to network",
	})

	// DHT metrics
	DHTRoutingTableSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "dht_routing_table_size",
		Help:      "Number of peers in DHT routing table",
	})

	DHTQueries = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "dht_queries_total",
		Help:      "Total number of DHT queries made",
	})

	// PubSub metrics
	PubSubTopicsJoined = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "pubsub_topics_joined",
		Help:      "Number of pubsub topics joined",
	})

	PubSubMessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pubsub_messages_received_total",
		Help:      "Total number of pubsub messages received by topic",
	}, []string{"topic"})

	PubSubMessagesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pubsub_messages_published_total",
		Help:      "Total number of pubsub messages published by topic",
	}, []string{"topic"})

	// Ping/Latency metrics
	PeerLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "peer_latency_seconds",
		Help:      "Latency to peers in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.01, 2, 12), // 10ms to ~40s
	}, []string{"peer"})

	AverageLatency = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "average_latency_seconds",
		Help:      "Average latency to all connected peers in seconds",
	})
)

// Helper functions for updating metrics

// RecordPeerConnected records a peer connection
func RecordPeerConnected() {
	PeersConnected.Inc()
	PeersTotal.Inc()
}

// RecordPeerDisconnected records a peer disconnection
func RecordPeerDisconnected() {
	PeersConnected.Dec()
	PeersDisconnected.Inc()
}

// RecordPeerBanned records a peer ban
func RecordPeerBanned() {
	PeersBanned.Inc()
}

// RecordBlockReceived records a received block
func RecordBlockReceived() {
	BlocksReceived.Inc()
}

// RecordBlockValidated records a validated block
func RecordBlockValidated() {
	BlocksValidated.Inc()
}

// RecordBlockRejected records a rejected block
func RecordBlockRejected() {
	BlocksRejected.Inc()
}

// RecordBlockBroadcast records a broadcast block
func RecordBlockBroadcast() {
	BlocksBroadcast.Inc()
}

// SetBlockHeight sets the current block height
func SetBlockHeight(height int) {
	BlockHeight.Set(float64(height))
}

// RecordTxReceived records a received transaction
func RecordTxReceived() {
	TxsReceived.Inc()
}

// RecordTxValidated records a validated transaction
func RecordTxValidated() {
	TxsValidated.Inc()
}

// RecordTxRejected records a rejected transaction
func RecordTxRejected() {
	TxsRejected.Inc()
}

// RecordTxBroadcast records a broadcast transaction
func RecordTxBroadcast() {
	TxsBroadcast.Inc()
}

// SetSyncProgress sets the sync progress
func SetSyncProgress(progress float64) {
	SyncProgress.Set(progress)
}

// SetSyncBlocksRemaining sets the remaining blocks to sync
func SetSyncBlocksRemaining(blocks int) {
	SyncBlocksRemaining.Set(float64(blocks))
}

// SetSyncActive sets whether sync is active
func SetSyncActive(active bool) {
	if active {
		SyncActive.Set(1)
	} else {
		SyncActive.Set(0)
	}
}

// RecordSyncDuration records sync duration
func RecordSyncDuration(seconds float64) {
	SyncDuration.Observe(seconds)
}

// RecordConsensusRoundStarted records a consensus round start
func RecordConsensusRoundStarted() {
	ConsensusRoundsTotal.Inc()
}

// RecordConsensusRoundCompleted records a consensus round completion
func RecordConsensusRoundCompleted() {
	ConsensusRoundsCompleted.Inc()
}

// RecordConsensusRoundFailed records a consensus round failure
func RecordConsensusRoundFailed() {
	ConsensusRoundsFailed.Inc()
}

// SetValidatorCount sets the validator count
func SetValidatorCount(count int) {
	ConsensusValidators.Set(float64(count))
}

// SetPrepareVotes sets the prepare vote count
func SetPrepareVotes(count int) {
	ConsensusPrepareVotes.Set(float64(count))
}

// SetCommitVotes sets the commit vote count
func SetCommitVotes(count int) {
	ConsensusCommitVotes.Set(float64(count))
}

// RecordMessageReceived records a received message
func RecordMessageReceived(msgType string) {
	MessagesReceived.WithLabelValues(msgType).Inc()
}

// RecordMessageSent records a sent message
func RecordMessageSent(msgType string) {
	MessagesSent.WithLabelValues(msgType).Inc()
}

// RecordMessageLatency records message processing latency
func RecordMessageLatency(msgType string, seconds float64) {
	MessageLatency.WithLabelValues(msgType).Observe(seconds)
}

// RecordBytesReceived records bytes received
func RecordBytesReceived(bytes int64) {
	BytesReceived.Add(float64(bytes))
}

// RecordBytesSent records bytes sent
func RecordBytesSent(bytes int64) {
	BytesSent.Add(float64(bytes))
}

// SetDHTRoutingTableSize sets the DHT routing table size
func SetDHTRoutingTableSize(size int) {
	DHTRoutingTableSize.Set(float64(size))
}

// RecordDHTQuery records a DHT query
func RecordDHTQuery() {
	DHTQueries.Inc()
}

// SetPubSubTopicsJoined sets the number of pubsub topics joined
func SetPubSubTopicsJoined(count int) {
	PubSubTopicsJoined.Set(float64(count))
}

// RecordPubSubMessageReceived records a received pubsub message
func RecordPubSubMessageReceived(topic string) {
	PubSubMessagesReceived.WithLabelValues(topic).Inc()
}

// RecordPubSubMessagePublished records a published pubsub message
func RecordPubSubMessagePublished(topic string) {
	PubSubMessagesPublished.WithLabelValues(topic).Inc()
}

// RecordPeerLatency records peer latency
func RecordPeerLatency(peerID string, seconds float64) {
	PeerLatency.WithLabelValues(peerID).Observe(seconds)
}

// SetAverageLatency sets the average latency
func SetAverageLatency(seconds float64) {
	AverageLatency.Set(seconds)
}
