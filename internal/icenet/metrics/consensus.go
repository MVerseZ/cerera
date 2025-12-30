package metrics

// UpdateConsensusStatus updates the consensus status
func (m *Metrics) UpdateConsensusStatus(status int) {
	m.ConsensusStatus.Set(float64(status))
}

// UpdateConsensusNodes updates the number of nodes in consensus
func (m *Metrics) UpdateConsensusNodes(count int) {
	m.ConsensusNodesTotal.Set(float64(count))
}

// UpdateConsensusVoters updates the number of voters in consensus
func (m *Metrics) UpdateConsensusVoters(count int) {
	m.ConsensusVotersTotal.Set(float64(count))
}

// UpdateConsensusNonce updates the consensus nonce
func (m *Metrics) UpdateConsensusNonce(nonce uint64) {
	m.ConsensusNonce.Set(float64(nonce))
}

// RecordConsensusError records a consensus error
func (m *Metrics) RecordConsensusError(errorType string) {
	m.ConsensusErrorsTotal.WithLabelValues(errorType).Inc()
}

