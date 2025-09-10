package gigea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/cerera/internal/cerera/types"
)

type GigeaConsensus struct {
	*BaseConsensusNode

	// Leadership state
	votesReceived map[types.Address]bool

	lastHeartbeat  time.Time
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer
}

func NewGigeaConsensus(config ConsensusConfig, networkManager *NetworkManager, engine *Engine) *GigeaConsensus {
	baseNode := NewBaseConsensusNode(config, networkManager, engine)

	return &GigeaConsensus{
		BaseConsensusNode: baseNode,
		lastHeartbeat:     time.Now(),
		votesReceived:     make(map[types.Address]bool),
	}
}

// AddPeer implements ConsensusAlgorithm.
func (g *GigeaConsensus) AddPeer(peer types.Address) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if peer already exists
	for _, existingPeer := range g.config.Peers {
		if existingPeer == peer {
			return nil // Already exists
		}
	}

	g.config.Peers = append(g.config.Peers, peer)
	//TODO

	return nil
}

// GetConfig implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).GetConfig of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) GetConfig() ConsensusConfig {
	return g.config
}

// GetConsensusInfo implements ConsensusAlgorithm.
func (g *GigeaConsensus) GetConsensusInfo() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"algorithm":  "SimpleConsensus",
		"node_id":    g.config.NodeID.Hex(),
		"state":      g.GetNodeState(),
		"leader":     g.GetLeader().Hex(),
		"term":       g.GetCurrentTerm(),
		"peer_count": len(g.config.Peers),
		// "request_queue":  len(g.requestQueue),
		// "commit_index":   g.commitIndex,
		// "last_heartbeat": g.lastHeartbeat,
		"running": g.running,
	}
}

// GetCurrentTerm implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).GetCurrentTerm of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) GetCurrentTerm() int64 {
	panic("unimplemented")
}

// // GetLeader implements ConsensusAlgorithm.
// // Subtle: this method shadows the method (*BaseConsensusNode).GetLeader of GigeaConsensus.BaseConsensusNode.
// func (g *GigeaConsensus) GetLeader() types.Address {
// 	panic("unimplemented")
// }

// GetMetrics implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).GetMetrics of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) GetMetrics() map[string]interface{} {
	panic("unimplemented")
}

// GetNodeState implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).GetNodeState of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) GetNodeState() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nodeState
}

// GetPeers implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).GetPeers of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) GetPeers() []types.Address {
	panic("unimplemented")
}

// IsLeader implements ConsensusAlgorithm.
func (g *GigeaConsensus) IsLeader() bool {
	return g.GetNodeState() == "Leader"
}

// IsRunning implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).IsRunning of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) IsRunning() bool {
	panic("unimplemented")
}

// ProposeBlock implements ConsensusAlgorithm.
func (g *GigeaConsensus) ProposeBlock(block interface{}) error {
	panic("unimplemented")
}

// RemovePeer implements ConsensusAlgorithm.
func (g *GigeaConsensus) RemovePeer(peer types.Address) error {
	panic("unimplemented")
}

// Start implements ConsensusAlgorithm.
func (g *GigeaConsensus) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return errors.New("consensus already running")
	}
	g.running = true

	go g.cLoop()

	return nil
}

// Main consensus loop
func (g *GigeaConsensus) cLoop() {
	ticker := time.NewTicker(50 * time.Millisecond) // Process every 50ms
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.processConsensus()
		}
	}
}

// Process consensus logic
func (g *GigeaConsensus) processConsensus() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return
	}

	switch g.GetNodeState() {
	// case "leader":
	// 	g.processAsLeader()
	case "candidate":
		g.processAsCandidate()
	case "follower":
		g.processAsFollower()
	}
}

func (g *GigeaConsensus) processAsFollower() {
	if time.Since(g.lastHeartbeat) > g.config.ElectionTimeout {
		g.startElection()
	}
}

func (g *GigeaConsensus) processAsCandidate() {
	majority := (len(g.config.Peers) / 2) + 1
	if len(g.votesReceived) >= majority {
		fmt.Printf("Node %s became leader for term %d with %d votes\n", g.config.NodeID.Hex(), g.currentTerm, len(g.votesReceived))
		g.setState("leader")
		// Reflect leader state in legacy consensus view
		G.state = Leader

	}
}

func (g *GigeaConsensus) startElection() {
	fmt.Printf("Node %s starting election (term %d)\n", g.config.NodeID.Hex(), g.currentTerm+1)

	// If we were a leader, emit leader lost event and update legacy state
	if g.GetNodeState() == "leader" {
		g.emitLeaderLost(g.config.NodeID, g.currentTerm)
		G.state = Follower
	}

	g.setTerm(g.currentTerm + 1)
	g.setState("candidate")
	// Reflect candidate state in legacy consensus view
	G.state = Candidate
	g.votesReceived = make(map[types.Address]bool)
	g.votesReceived[g.config.NodeID] = true // Vote for self

	// Reset election timer
	g.resetElectionTimer()

	// Broadcast vote request via pubsub if available; otherwise simulate
	if PublishConsensus != nil {
		req := map[string]interface{}{
			"term":      g.currentTerm,
			"candidate": g.config.NodeID.Hex(),
		}
		content, _ := json.Marshal(req)
		PublishConsensus(fmt.Sprintf("%s_CONS_VOTE_REQ:%s", g.config.NodeID.String(), string(content)))
	}
	// else {
	// 	go g.simulateVoteCollection()
	// }
}

func (g *GigeaConsensus) resetElectionTimer() {
	if g.electionTimer != nil {
		g.electionTimer.Stop()
	}
	// Random timeout to prevent split votes
	timeout := g.config.ElectionTimeout + time.Duration(rand.Int63n(int64(g.config.ElectionTimeout)))
	g.electionTimer = time.AfterFunc(timeout, func() {
		if g.GetNodeState() == "follower" {
			g.startElection()
		}
	})
}

// Stop implements ConsensusAlgorithm.
func (g *GigeaConsensus) Stop() error {
	panic("unimplemented")
}

// SubmitRequest implements ConsensusAlgorithm.
func (g *GigeaConsensus) SubmitRequest(operation string) error {
	panic("unimplemented")
}

// UpdateConfig implements ConsensusAlgorithm.
// Subtle: this method shadows the method (*BaseConsensusNode).UpdateConfig of GigeaConsensus.BaseConsensusNode.
func (g *GigeaConsensus) UpdateConfig(config ConsensusConfig) error {
	panic("unimplemented")
}
