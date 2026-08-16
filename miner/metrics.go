package miner

import "github.com/prometheus/client_golang/prometheus"

var (
	minerBlocksMinedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_blocks_mined_total",
		Help: "Total number of blocks successfully mined",
	})
	minerMiningAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_mining_attempts_total",
		Help: "Total number of mining attempts",
	})
	minerMiningErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_mining_errors_total",
		Help: "Total number of mining errors",
	})
	minerMiningDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_mining_duration_seconds",
		Help:    "Time spent mining a block in seconds",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	})
	minerPendingTxsInBlock = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_pending_txs_in_block",
		Help: "Number of pending transactions included in the last mined block",
	})
	minerStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_status",
		Help: "Miner status (0=stopped, 1=active)",
	})
	// Метрики проверки хэша
	minerHashValidationTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_validation_total",
		Help: "Total number of hash validations",
	})
	minerHashValidTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_valid_total",
		Help: "Total number of valid hashes",
	})
	minerHashInvalidTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_hash_invalid_total",
		Help: "Total number of invalid hashes",
	})
	// Метрики поиска nonce
	minerNonceSearchAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "miner_nonce_search_attempts_total",
		Help: "Total number of nonce search attempts",
	})
	minerNonceSearchAttempts = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_nonce_search_attempts",
		Help:    "Distribution of nonce search attempts per block",
		Buckets: []float64{1, 10, 100, 1000, 10000, 100000, 1000000},
	})
	minerNonceSearchDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "miner_nonce_search_duration_seconds",
		Help:    "Time spent searching for valid nonce in seconds",
		Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
	})
	// Метрики difficulty и target
	minerCurrentDifficulty = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_current_difficulty",
		Help: "Current block difficulty",
	})
	minerCurrentTarget = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_current_target",
		Help: "Current target value (2^256 / difficulty)",
	})
	minerHashToTargetRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "miner_hash_to_target_ratio",
		Help: "Ratio of block hash to target (for monitoring proximity to validity)",
	})
)
