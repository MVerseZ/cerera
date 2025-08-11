package gigea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// SimpleConsensus is a basic consensus implementation using the template
// This is a simplified leader-based consensus for demonstration
//
// Leadership Management:
//   - When peers are added/removed, leadership is automatically reset to ensure
//     proper re-election with the new network topology
//   - All nodes (including the current leader) step down and start new election cycle
//   - Term is incremented to invalidate previous leadership decisions
//   - Topology change notifications are broadcast to all peers
type SimpleConsensus struct {
	*BaseConsensusNode

	// Simple consensus specific fields
	lastHeartbeat  time.Time
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer
	requestQueue   []string
	commitIndex    int64

	// Leadership state
	votesReceived map[types.Address]bool

	// Voting state
	votedFor      types.Address
	lastVotedTerm int64

	// Stab state

	mu sync.RWMutex
}

// NewSimpleConsensus creates a new simple consensus instance
func NewSimpleConsensus(config ConsensusConfig, networkManager *NetworkManager, engine *Engine) *SimpleConsensus {
	base := NewBaseConsensusNode(config, networkManager, engine)

	sc := &SimpleConsensus{
		BaseConsensusNode: base,
		lastHeartbeat:     time.Now(),
		requestQueue:      make([]string, 0),
		commitIndex:       0,
		votesReceived:     make(map[types.Address]bool),
	}

	return sc
}

// Start implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) Start(ctx context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.running {
		return errors.New("consensus already running")
	}

	sc.running = true
	sc.setState("follower")
	sc.UpdateMetric("start_time", time.Now())
	sc.UpdateMetric("requests_processed", 0)
	sc.UpdateMetric("leadership_changes", 0)

	// Start election timer
	sc.resetElectionTimer()

	// Start main consensus loop
	go sc.consensusLoop()

	fmt.Printf("SimpleConsensus started for node %s\n", sc.config.NodeID.Hex())
	return nil
}

// Stop implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) Stop() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.running {
		return errors.New("consensus not running")
	}

	sc.running = false
	sc.cancel()

	if sc.electionTimer != nil {
		sc.electionTimer.Stop()
	}
	if sc.heartbeatTimer != nil {
		sc.heartbeatTimer.Stop()
	}

	fmt.Printf("SimpleConsensus stopped for node %s\n", sc.config.NodeID.Hex())
	return nil
}

// SubmitRequest implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) SubmitRequest(operation string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.running {
		return errors.New("consensus not running")
	}

	// If we're the leader, add to our queue
	if sc.IsLeader() {
		sc.requestQueue = append(sc.requestQueue, operation)
		sc.UpdateMetric("requests_received", len(sc.requestQueue))
		fmt.Printf("Request queued by leader %s: %s\n", sc.config.NodeID.Hex(), operation)
		return nil
	}

	// If we're not the leader, forward to leader (simplified - just log for now)
	leader := sc.GetLeader()
	if leader == (types.Address{}) {
		return errors.New("no leader available")
	}

	fmt.Printf("Forwarding request to leader %s: %s\n", leader.Hex(), operation)
	return nil
}

// ProposeBlock implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) ProposeBlock(block interface{}) error {
	if !sc.IsLeader() {
		return errors.New("only leader can propose blocks")
	}

	fmt.Printf("Leader %s proposing block\n", sc.config.NodeID.Hex())
	// In a real implementation, this would broadcast the block proposal
	// and wait for consensus from followers

	return nil
}

// IsLeader implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) IsLeader() bool {
	return sc.GetNodeState() == "leader"
}

// AddPeer implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) AddPeer(peer types.Address) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Check if peer already exists
	for _, existingPeer := range sc.config.Peers {
		if existingPeer == peer {
			return nil // Already exists
		}
	}

	// Store current leader before adding peer
	currentLeader := sc.GetLeader()
	wasLeader := sc.IsLeader()

	// Add the new peer
	sc.config.Peers = append(sc.config.Peers, peer)
	sc.emitPeerAdded(peer)

	// Reset leadership for all nodes when topology changes
	// This ensures proper re-election with the new peer count
	if currentLeader != (types.Address{}) {
		// Clear current leader to force re-election
		sc.setLeader(types.Address{})

		// If this node was the leader, step down
		if wasLeader {
			sc.setState("follower")
			sc.emitLeaderLost(sc.config.NodeID, sc.currentTerm)
		}

		// Increment term to invalidate previous leadership
		sc.setTerm(sc.currentTerm + 1)

		// Reset election timer to start new election cycle
		sc.resetElectionTimer()

		// Notify other peers about topology change to trigger re-election
		sc.notifyTopologyChange()

		fmt.Printf("Added peer %s, resetting leadership. New term: %d\n", peer.Hex(), sc.currentTerm)
	} else {
		fmt.Printf("Added peer %s to SimpleConsensus\n", peer.Hex())
	}

	return nil
}

// RemovePeer implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) RemovePeer(peer types.Address) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for i, existingPeer := range sc.config.Peers {
		if existingPeer == peer {
			// Store current leader before removing peer
			currentLeader := sc.GetLeader()
			wasLeader := sc.IsLeader()

			// Remove the peer
			sc.config.Peers = append(sc.config.Peers[:i], sc.config.Peers[i+1:]...)
			sc.emitPeerRemoved(peer)

			// Reset leadership when topology changes
			// This ensures proper re-election with the new peer count
			if currentLeader != (types.Address{}) {
				// Clear current leader to force re-election
				sc.setLeader(types.Address{})

				// If this node was the leader, step down
				if wasLeader {
					sc.setState("follower")
					sc.emitLeaderLost(sc.config.NodeID, sc.currentTerm)
				}

				// Increment term to invalidate previous leadership
				sc.setTerm(sc.currentTerm + 1)

				// Reset election timer to start new election cycle
				sc.resetElectionTimer()

				// Notify other peers about topology change to trigger re-election
				sc.notifyTopologyChange()

				fmt.Printf("Removed peer %s, resetting leadership. New term: %d\n", peer.Hex(), sc.currentTerm)
			} else {
				fmt.Printf("Removed peer %s from SimpleConsensus\n", peer.Hex())
			}

			return nil
		}
	}

	return errors.New("peer not found")
}

// GetConsensusInfo implements ConsensusAlgorithm interface
func (sc *SimpleConsensus) GetConsensusInfo() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	return map[string]interface{}{
		"algorithm":      "SimpleConsensus",
		"node_id":        sc.config.NodeID.Hex(),
		"state":          sc.GetNodeState(),
		"leader":         sc.GetLeader().Hex(),
		"term":           sc.GetCurrentTerm(),
		"peer_count":     len(sc.config.Peers),
		"request_queue":  len(sc.requestQueue),
		"commit_index":   sc.commitIndex,
		"last_heartbeat": sc.lastHeartbeat,
		"running":        sc.running,
	}
}

// Main consensus loop
func (sc *SimpleConsensus) consensusLoop() {
	ticker := time.NewTicker(50 * time.Millisecond) // Process every 50ms
	defer ticker.Stop()

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
			sc.processConsensus()
		}
	}
}

// Process consensus logic
func (sc *SimpleConsensus) processConsensus() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if !sc.running {
		return
	}

	switch sc.GetNodeState() {
	case "leader":
		sc.processAsLeader()
	case "candidate":
		sc.processAsCandidate()
	case "follower":
		sc.processAsFollower()
	}
}

// Process logic when node is leader
func (sc *SimpleConsensus) processAsLeader() {
	// Send heartbeats
	if time.Since(sc.lastHeartbeat) > sc.config.HeartbeatInterval {
		sc.sendHeartbeat()
		sc.lastHeartbeat = time.Now()
	}

	// Process request queue
	if len(sc.requestQueue) > 0 {
		// Simulate processing a request
		request := sc.requestQueue[0]
		sc.requestQueue = sc.requestQueue[1:]
		sc.commitIndex++

		// Emit committed request
		sc.emitRequestCommitted(request, fmt.Sprintf("committed_%d", sc.commitIndex))

		processed := sc.GetMetrics()["requests_processed"].(int) + 1
		sc.UpdateMetric("requests_processed", processed)

		fmt.Printf("Leader %s committed request: %s (index: %d)\n",
			sc.config.NodeID.Hex(), request, sc.commitIndex)
	}
}

// Process logic when node is candidate
func (sc *SimpleConsensus) processAsCandidate() {
	// Check if we have majority votes
	majority := len(sc.config.Peers)/2 + 1
	if len(sc.votesReceived) >= majority {
		sc.becomeLeader()
	}
}

// Process logic when node is follower
func (sc *SimpleConsensus) processAsFollower() {
	// Check for election timeout
	if time.Since(sc.lastHeartbeat) > sc.config.ElectionTimeout {
		sc.startElection()
	}
}

// Start election process
func (sc *SimpleConsensus) startElection() {
	fmt.Printf("Node %s starting election (term %d)\n", sc.config.NodeID.Hex(), sc.currentTerm+1)

	// If we were a leader, emit leader lost event and update legacy state
	if sc.GetNodeState() == "leader" {
		sc.emitLeaderLost(sc.config.NodeID, sc.currentTerm)
		G.state = Follower
	}

	sc.setTerm(sc.currentTerm + 1)
	sc.setState("candidate")
	// Reflect candidate state in legacy consensus view
	G.state = Candidate
	sc.votesReceived = make(map[types.Address]bool)
	sc.votesReceived[sc.config.NodeID] = true // Vote for self

	// Reset election timer
	sc.resetElectionTimer()

	// Broadcast vote request via pubsub if available; otherwise simulate
	if PublishConsensus != nil {
		req := map[string]interface{}{
			"term":      sc.currentTerm,
			"candidate": sc.config.NodeID.Hex(),
		}
		content, _ := json.Marshal(req)
		PublishConsensus(fmt.Sprintf("%s_CONS_VOTE_REQ:%s", sc.config.NodeID.String(), string(content)))
	} else {
		go sc.simulateVoteCollection()
	}
}

// Simulate vote collection (in real implementation, this would be network communication)
func (sc *SimpleConsensus) simulateVoteCollection() {
	time.Sleep(100 * time.Millisecond) // Simulate network delay

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.GetNodeState() != "candidate" {
		return
	}

	// Simulate receiving votes from peers (randomly)
	for _, peer := range sc.config.Peers {
		if peer != sc.config.NodeID {
			// 70% chance of receiving vote (simplified)
			if rand.Float32() < 0.7 {
				sc.votesReceived[peer] = true
			}
		}
	}
}

// Become leader
func (sc *SimpleConsensus) becomeLeader() {
	fmt.Printf("Node %s became leader (term %d)\n", sc.config.NodeID.Hex(), sc.currentTerm)

	sc.setState("leader")
	sc.setLeader(sc.config.NodeID)
	sc.lastHeartbeat = time.Now()

	// Start heartbeat timer
	if sc.heartbeatTimer != nil {
		sc.heartbeatTimer.Stop()
	}

	sc.emitLeaderElected(sc.config.NodeID, sc.currentTerm)
	// Keep legacy state in sync
	G.state = Leader

	changes := sc.GetMetrics()["leadership_changes"].(int) + 1
	sc.UpdateMetric("leadership_changes", changes)

	// Announce leadership to converge network
	if PublishConsensus != nil {
		ann := map[string]interface{}{
			"term":   sc.currentTerm,
			"leader": sc.config.NodeID.Hex(),
		}
		content, _ := json.Marshal(ann)
		PublishConsensus(fmt.Sprintf("%s_CONS_LEADER:%s", sc.config.NodeID.String(), string(content)))
		// optionally send an immediate heartbeat
		sc.sendHeartbeat()
	}
}

// Send heartbeat to followers
func (sc *SimpleConsensus) sendHeartbeat() {
	// Publish heartbeat via pubsub transport
	fmt.Printf("Leader %s sending heartbeat (term %d)\n", sc.config.NodeID.Hex(), sc.currentTerm)
	if PublishConsensus != nil {
		content := sc.generateHeartbeatContent()
		PublishConsensus(fmt.Sprintf("%s_CONS_HB:%s", sc.config.NodeID.String(), string(content)))
	}
}

// NotifyHeartbeat updates local follower state on receiving a heartbeat via pubsub
func (sc *SimpleConsensus) NotifyHeartbeat(term int64, leader types.Address) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if term < sc.currentTerm {
		return
	}
	sc.setTerm(term)
	// If we were leader and receive another leader's heartbeat, emit leader lost
	if sc.GetNodeState() == "leader" && leader != sc.config.NodeID {
		sc.emitLeaderLost(sc.config.NodeID, term)
		G.state = Follower
	}
	sc.setLeader(leader)
	sc.lastHeartbeat = time.Now()
	if sc.GetNodeState() != "leader" {
		sc.setState("follower")
		G.state = Follower
	}
}

// NotifyVoteRequest handles incoming vote requests from candidates via pubsub
func (sc *SimpleConsensus) NotifyVoteRequest(term int64, candidate types.Address) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Ignore stale term
	if term < sc.currentTerm {
		return
	}

	// If higher term, update and become follower
	if term > sc.currentTerm {
		sc.setTerm(term)
		sc.setState("follower")
		sc.votedFor = types.Address{}
		sc.lastVotedTerm = 0
	}

	granted := false
	if sc.lastVotedTerm != term {
		// Grant vote if candidate is known or allow anyway in simple mode
		granted = true
		sc.votedFor = candidate
		sc.lastVotedTerm = term
		sc.setLeader(types.Address{})
	}

	// Publish vote response via pubsub
	if PublishConsensus != nil {
		resp := map[string]interface{}{
			"term":    term,
			"voter":   sc.config.NodeID.Hex(),
			"granted": granted,
		}
		content, _ := json.Marshal(resp)
		PublishConsensus(fmt.Sprintf("%s_CONS_VOTE_RESP:%s", sc.config.NodeID.String(), string(content)))
	}
}

// NotifyVoteResponse handles incoming vote responses in current election
func (sc *SimpleConsensus) NotifyVoteResponse(term int64, voter types.Address, granted bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.GetNodeState() != "candidate" || term != sc.currentTerm || !granted {
		return
	}
	sc.votesReceived[voter] = true
	majority := len(sc.config.Peers)/2 + 1
	if len(sc.votesReceived) >= majority {
		sc.becomeLeader()
	}
}

// NotifyLeaderAnnouncement handles leader announcements to converge state
func (sc *SimpleConsensus) NotifyLeaderAnnouncement(term int64, leader types.Address) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if term < sc.currentTerm {
		return
	}
	sc.setTerm(term)
	// If stepping down from leader due to another leader announcement
	if sc.GetNodeState() == "leader" && leader != sc.config.NodeID {
		sc.emitLeaderLost(sc.config.NodeID, term)
		G.state = Follower
	}
	sc.setLeader(leader)
	if sc.config.NodeID == leader {
		sc.setState("leader")
		G.state = Leader
	} else {
		sc.setState("follower")
		G.state = Follower
	}
}

// NotifyTopologyChange handles topology change events from other nodes
// This ensures all nodes reset leadership when network topology changes
func (sc *SimpleConsensus) NotifyTopologyChange(term int64, nodeID types.Address, peerCount int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Only process topology changes from higher or equal terms
	if term < sc.currentTerm {
		return
	}

	// If we receive a topology change notification, we should also reset leadership
	if term > sc.currentTerm {
		sc.setTerm(term)
	} else if term == sc.currentTerm {
		// Same term, but topology changed - increment our term
		sc.setTerm(term + 1)
	}

	// Clear current leader and become follower
	if sc.IsLeader() {
		sc.setState("follower")
		sc.emitLeaderLost(sc.config.NodeID, sc.currentTerm)
	}
	sc.setLeader(types.Address{})

	// Reset election timer to start new election cycle
	sc.resetElectionTimer()

	fmt.Printf("Received topology change notification from %s (term %d, %d peers). Reset leadership.\n",
		nodeID.Hex(), term, peerCount)
}

func (sc *SimpleConsensus) generateHeartbeatContent() json.RawMessage {
	var data = map[string]interface{}{}
	data["term"] = sc.currentTerm
	data["leader"] = sc.config.NodeID.Hex()
	content, _ := json.Marshal(data)
	return content
}

// Reset election timer
func (sc *SimpleConsensus) resetElectionTimer() {
	if sc.electionTimer != nil {
		sc.electionTimer.Stop()
	}

	// Random timeout to prevent split votes
	timeout := sc.config.ElectionTimeout + time.Duration(rand.Int63n(int64(sc.config.ElectionTimeout)))
	sc.electionTimer = time.AfterFunc(timeout, func() {
		if sc.GetNodeState() == "follower" {
			sc.startElection()
		}
	})
}

// ForceResetLeadership forces a leadership reset and starts a new election cycle
// This is useful when topology changes or when manual intervention is needed
func (sc *SimpleConsensus) ForceResetLeadership() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	wasLeader := sc.IsLeader()
	currentLeader := sc.GetLeader()

	if currentLeader != (types.Address{}) {
		// Clear current leader
		sc.setLeader(types.Address{})

		// If this node was the leader, step down
		if wasLeader {
			sc.setState("follower")
			sc.emitLeaderLost(sc.config.NodeID, sc.currentTerm)
		}

		// Increment term to invalidate previous leadership
		sc.setTerm(sc.currentTerm + 1)

		// Reset election timer to start new election cycle
		sc.resetElectionTimer()

		fmt.Printf("Leadership forcefully reset. New term: %d\n", sc.currentTerm)
	}
}

// notifyTopologyChange notifies other peers about topology changes
// This triggers re-election across the network
func (sc *SimpleConsensus) notifyTopologyChange() {
	if PublishConsensus != nil {
		// Publish topology change event
		event := map[string]interface{}{
			"type":       "topology_change",
			"node_id":    sc.config.NodeID.Hex(),
			"term":       sc.currentTerm,
			"peer_count": len(sc.config.Peers),
			"timestamp":  time.Now(),
		}
		content, _ := json.Marshal(event)
		PublishConsensus(fmt.Sprintf("%s_CONS_TOPOLOGY:%s", sc.config.NodeID.String(), string(content)))

		fmt.Printf("Notified peers about topology change (term %d, %d peers)\n", sc.currentTerm, len(sc.config.Peers))
	}
}
