package gigea

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// RaftNodeState represents the state of a Raft node
type RaftNodeState int

const (
	RaftFollower RaftNodeState = iota
	RaftCandidate
	RaftLeader
)

func (s RaftNodeState) String() string {
	switch s {
	case RaftFollower:
		return "RaftFollower"
	case RaftCandidate:
		return "RaftCandidate"
	case RaftLeader:
		return "RaftLeader"
	default:
		return "Unknown"
	}
}

// LogEntry represents a log entry in Raft
type LogEntry struct {
	Index   int64
	Term    int64
	Command interface{}
}

// RaftNode represents a node in the Raft consensus
type RaftNode struct {
	// Persistent state
	NodeID      types.Address
	CurrentTerm int64
	VotedFor    *types.Address
	Log         []LogEntry

	// Volatile state
	CommitIndex int64
	LastApplied int64

	// Leader state
	NextIndex  map[types.Address]int64
	MatchIndex map[types.Address]int64

	// Configuration
	Peers             []types.Address
	HeartbeatInterval time.Duration
	ElectionTimeout   time.Duration

	// State
	State    RaftNodeState
	LeaderID *types.Address

	// Timers
	ElectionTimer  *time.Timer
	HeartbeatTimer *time.Ticker

	// Channels
	RequestVoteChan       chan *RequestVoteRequest
	RequestVoteRespChan   chan *RequestVoteResponse
	AppendEntriesChan     chan *AppendEntriesRequest
	AppendEntriesRespChan chan *AppendEntriesResponse
	ClientRequestChan     chan *ClientRequest

	// Network manager
	NetworkManager *NetworkManager

	// Mutex for thread safety
	mu sync.RWMutex

	// Engine reference
	engine *Engine
}

// RequestVoteRequest represents a request vote RPC
type RequestVoteRequest struct {
	Term         int64
	CandidateID  types.Address
	LastLogIndex int64
	LastLogTerm  int64
}

// RequestVoteResponse represents a request vote response
type RequestVoteResponse struct {
	Term        int64
	VoteGranted bool
}

// AppendEntriesRequest represents an append entries RPC
type AppendEntriesRequest struct {
	Term         int64
	LeaderID     types.Address
	PrevLogIndex int64
	PrevLogTerm  int64
	Entries      []LogEntry
	LeaderCommit int64
}

// AppendEntriesResponse represents an append entries response
type AppendEntriesResponse struct {
	Term    int64
	Success bool
}

// ClientRequest represents a client request
type ClientRequest struct {
	Command interface{}
	ID      string
}

// NewRaftNode creates a new Raft node
func NewRaftNode(nodeID types.Address, peers []types.Address, engine *Engine, nm *NetworkManager) *RaftNode {
	raft := &RaftNode{
		NodeID:                nodeID,
		CurrentTerm:           0,
		VotedFor:              nil,
		Log:                   make([]LogEntry, 0),
		CommitIndex:           0,
		LastApplied:           0,
		NextIndex:             make(map[types.Address]int64),
		MatchIndex:            make(map[types.Address]int64),
		Peers:                 peers,
		HeartbeatInterval:     100 * time.Millisecond,
		ElectionTimeout:       time.Duration(150+rand.Intn(150)) * time.Millisecond,
		State:                 RaftFollower,
		LeaderID:              nil,
		RequestVoteChan:       make(chan *RequestVoteRequest, 100),
		RequestVoteRespChan:   make(chan *RequestVoteResponse, 100),
		AppendEntriesChan:     make(chan *AppendEntriesRequest, 100),
		AppendEntriesRespChan: make(chan *AppendEntriesResponse, 100),
		ClientRequestChan:     make(chan *ClientRequest, 100),
		engine:                engine,
	}

	// Create network manager
	// port := 30000 + int(nodeID[0]) // Simple port calculation
	// raft.NetworkManager = NewNetworkManager(nodeID, port)
	raft.NetworkManager = nm

	// Set up network callbacks
	raft.NetworkManager.OnRequestVote = raft.handleRequestVoteFromNetwork
	raft.NetworkManager.OnRequestVoteResp = raft.handleRequestVoteResponseFromNetwork
	raft.NetworkManager.OnAppendEntries = raft.handleAppendEntriesFromNetwork
	raft.NetworkManager.OnAppendEntriesResp = raft.handleAppendEntriesResponseFromNetwork

	return raft
}

// Start starts the Raft consensus
func (r *RaftNode) Start() {
	// Start network manager
	if err := r.NetworkManager.Start(); err != nil {
		fmt.Printf("Failed to start network manager: %v\n", err)
		return
	}

	// Add peers to network manager
	for _, peer := range r.Peers {
		if peer != r.NodeID {
			r.NetworkManager.AddPeer(peer)
		}
	}

	// Initialize nextIndex and matchIndex for all peers
	for _, peer := range r.Peers {
		if peer != r.NodeID {
			r.NextIndex[peer] = 1
			r.MatchIndex[peer] = 0
		}
	}

	// Start election timer
	r.resetElectionTimer()

	// Start handlers
	go r.handleRequestVote()
	go r.handleAppendEntries()
	go r.handleClientRequests()
	go r.runStateMachine()
}

// runStateMachine runs the main state machine
func (r *RaftNode) runStateMachine() {
	for {
		switch r.State {
		case RaftFollower:
			r.runFollower()
		case RaftCandidate:
			r.runCandidate()
		case RaftLeader:
			r.runLeader()
		}
	}
}

// runFollower runs the follower state
func (r *RaftNode) runFollower() {
	for r.State == RaftFollower {
		select {
		case <-r.ElectionTimer.C:
			r.startElection()
		case req := <-r.RequestVoteChan:
			r.processRequestVote(req)
		case req := <-r.AppendEntriesChan:
			r.processAppendEntries(req)
		}
	}
}

// runCandidate runs the candidate state
func (r *RaftNode) runCandidate() {
	r.CurrentTerm++
	r.VotedFor = &r.NodeID
	r.State = RaftCandidate

	// Request votes from all peers
	votes := make(map[types.Address]bool)
	votes[r.NodeID] = true // Vote for self

	for _, peer := range r.Peers {
		if peer != r.NodeID {
			go r.sendRequestVote(peer, votes)
		}
	}

	// Wait for election timeout or majority
	electionTimer := time.NewTimer(r.ElectionTimeout)
	defer electionTimer.Stop()

	for r.State == RaftCandidate {
		select {
		case <-electionTimer.C:
			r.startElection() // Start new election
			return
		case resp := <-r.RequestVoteRespChan:
			if resp.VoteGranted {
				votes[r.NodeID] = true
				if len(votes) > len(r.Peers)/2 {
					r.becomeLeader()
					return
				}
			}
		case req := <-r.RequestVoteChan:
			r.processRequestVote(req)
		case req := <-r.AppendEntriesChan:
			r.processAppendEntries(req)
		}
	}
}

// runLeader runs the leader state
func (r *RaftNode) runLeader() {
	r.LeaderID = &r.NodeID
	r.HeartbeatTimer = time.NewTicker(r.HeartbeatInterval)
	defer r.HeartbeatTimer.Stop()

	// Send initial heartbeat
	r.sendHeartbeat()

	for r.State == RaftLeader {
		select {
		case <-r.HeartbeatTimer.C:
			r.sendHeartbeat()
		case req := <-r.RequestVoteChan:
			r.processRequestVote(req)
		case req := <-r.AppendEntriesChan:
			r.processAppendEntries(req)
		case clientReq := <-r.ClientRequestChan:
			r.handleClientRequest(clientReq)
		}
	}
}

// startElection starts a new election
func (r *RaftNode) startElection() {
	r.State = RaftCandidate
	r.resetElectionTimer()
}

// becomeLeader transitions to leader state
func (r *RaftNode) becomeLeader() {
	r.State = RaftLeader
	r.LeaderID = &r.NodeID

	// Initialize leader state
	for _, peer := range r.Peers {
		if peer != r.NodeID {
			r.NextIndex[peer] = int64(len(r.Log) + 1)
			r.MatchIndex[peer] = 0
		}
	}

	fmt.Printf("Node %s became leader for term %d\n", r.NodeID.Hex(), r.CurrentTerm)
}

// resetElectionTimer resets the election timer
func (r *RaftNode) resetElectionTimer() {
	if r.ElectionTimer != nil {
		r.ElectionTimer.Stop()
	}
	r.ElectionTimeout = time.Duration(150+rand.Intn(150)) * time.Millisecond
	r.ElectionTimer = time.NewTimer(r.ElectionTimeout)
}

// handleRequestVote handles request vote RPCs
func (r *RaftNode) handleRequestVote() {
	for req := range r.RequestVoteChan {
		r.processRequestVote(req)
	}
}

// processRequestVote processes a request vote
func (r *RaftNode) processRequestVote(req *RequestVoteRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reply false if term < currentTerm
	if req.Term < r.CurrentTerm {
		r.sendRequestVoteResponse(req.CandidateID, false)
		return
	}

	// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if req.Term > r.CurrentTerm {
		r.CurrentTerm = req.Term
		r.State = RaftFollower
		r.VotedFor = nil
		r.LeaderID = nil
	}

	// Grant vote if votedFor is null or candidateId, and candidate's log is at least as up-to-date
	lastLogIndex := int64(len(r.Log))
	lastLogTerm := int64(0)
	if lastLogIndex > 0 {
		lastLogTerm = r.Log[lastLogIndex-1].Term
	}

	voteGranted := false
	if (r.VotedFor == nil || *r.VotedFor == req.CandidateID) &&
		(req.LastLogTerm > lastLogTerm || (req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)) {
		r.VotedFor = &req.CandidateID
		voteGranted = true
		r.resetElectionTimer()
	}

	r.sendRequestVoteResponse(req.CandidateID, voteGranted)
}

// handleAppendEntries handles append entries RPCs
func (r *RaftNode) handleAppendEntries() {
	for req := range r.AppendEntriesChan {
		r.processAppendEntries(req)
	}
}

// processAppendEntries processes an append entries request
func (r *RaftNode) processAppendEntries(req *AppendEntriesRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reply false if term < currentTerm
	if req.Term < r.CurrentTerm {
		r.sendAppendEntriesResponse(req.LeaderID, false)
		return
	}

	// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if req.Term > r.CurrentTerm {
		r.CurrentTerm = req.Term
		r.State = RaftFollower
		r.VotedFor = nil
	}

	// Update leader ID and reset election timer
	r.LeaderID = &req.LeaderID
	r.State = RaftFollower
	r.resetElectionTimer()

	// Reply false if log doesn't contain an entry at prevLogIndex whose term matches prevLogTerm
	if req.PrevLogIndex > int64(len(r.Log)) ||
		(req.PrevLogIndex > 0 && r.Log[req.PrevLogIndex-1].Term != req.PrevLogTerm) {
		r.sendAppendEntriesResponse(req.LeaderID, false)
		return
	}

	// Append any new entries not already in the log
	if len(req.Entries) > 0 {
		// Truncate log if there's a conflict
		if req.PrevLogIndex < int64(len(r.Log)) {
			r.Log = r.Log[:req.PrevLogIndex]
		}
		r.Log = append(r.Log, req.Entries...)
	}

	// If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if req.LeaderCommit > r.CommitIndex {
		lastNewEntryIndex := int64(len(r.Log))
		if req.LeaderCommit < lastNewEntryIndex {
			r.CommitIndex = req.LeaderCommit
		} else {
			r.CommitIndex = lastNewEntryIndex
		}

		// Apply committed entries
		r.applyCommittedEntries()
	}

	r.sendAppendEntriesResponse(req.LeaderID, true)
}

// handleClientRequests handles client requests
func (r *RaftNode) handleClientRequests() {
	for req := range r.ClientRequestChan {
		r.handleClientRequest(req)
	}
}

// handleClientRequest processes a client request
func (r *RaftNode) handleClientRequest(req *ClientRequest) {
	if r.State != RaftLeader {
		// Redirect to leader
		fmt.Printf("Not leader, redirecting request to leader\n")
		return
	}

	// Append entry to local log
	entry := LogEntry{
		Index:   int64(len(r.Log) + 1),
		Term:    r.CurrentTerm,
		Command: req.Command,
	}

	r.mu.Lock()
	r.Log = append(r.Log, entry)
	r.mu.Unlock()

	// Replicate to followers
	r.replicateLog()
}

// replicateLog replicates the log to followers
func (r *RaftNode) replicateLog() {
	for _, peer := range r.Peers {
		if peer != r.NodeID {
			go r.sendAppendEntries(peer)
		}
	}
}

// sendHeartbeat sends heartbeat to all peers
func (r *RaftNode) sendHeartbeat() {
	for _, peer := range r.Peers {
		if peer != r.NodeID {
			go r.sendAppendEntries(peer)
		}
	}
}

// sendRequestVote sends a request vote to a peer
func (r *RaftNode) sendRequestVote(peer types.Address, votes map[types.Address]bool) {
	lastLogIndex := int64(len(r.Log))
	lastLogTerm := int64(0)
	if lastLogIndex > 0 {
		lastLogTerm = r.Log[lastLogIndex-1].Term
	}

	req := &RequestVoteRequest{
		Term:         r.CurrentTerm,
		CandidateID:  r.NodeID,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	if r.NetworkManager != nil {
		r.NetworkManager.SendRequestVote(req, peer)
	} else {
		// Fallback to console output
		fmt.Printf("Sending request vote to %s\n", peer.Hex())
	}
}

// sendRequestVoteResponse sends a request vote response
func (r *RaftNode) sendRequestVoteResponse(peer types.Address, granted bool) {
	resp := &RequestVoteResponse{
		Term:        r.CurrentTerm,
		VoteGranted: granted,
	}

	if r.NetworkManager != nil {
		r.NetworkManager.SendRequestVoteResponse(resp, peer)
	} else {
		// Fallback to console output
		fmt.Printf("Sending request vote response to %s: %t\n", peer.Hex(), granted)
	}
}

// sendAppendEntries sends append entries to a peer
func (r *RaftNode) sendAppendEntries(peer types.Address) {
	r.mu.RLock()
	nextIndex := r.NextIndex[peer]
	entries := r.Log[nextIndex-1:]
	r.mu.RUnlock()

	prevLogIndex := nextIndex - 1
	prevLogTerm := int64(0)
	if prevLogIndex > 0 && prevLogIndex <= int64(len(r.Log)) {
		prevLogTerm = r.Log[prevLogIndex-1].Term
	}

	req := &AppendEntriesRequest{
		Term:         r.CurrentTerm,
		LeaderID:     r.NodeID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: r.CommitIndex,
	}

	if r.NetworkManager != nil {
		r.NetworkManager.SendAppendEntries(req, peer)
	} else {
		// Fallback to console output
		fmt.Printf("Sending append entries to %s\n", peer.Hex())
	}
}

// sendAppendEntriesResponse sends an append entries response
func (r *RaftNode) sendAppendEntriesResponse(peer types.Address, success bool) {
	resp := &AppendEntriesResponse{
		Term:    r.CurrentTerm,
		Success: success,
	}

	if r.NetworkManager != nil {
		r.NetworkManager.SendAppendEntriesResponse(resp, peer)
	} else {
		// Fallback to console output
		fmt.Printf("Sending append entries response to %s: %t\n", peer.Hex(), success)
	}
}

// applyCommittedEntries applies committed entries to the state machine
func (r *RaftNode) applyCommittedEntries() {
	for r.LastApplied < r.CommitIndex {
		r.LastApplied++
		entry := r.Log[r.LastApplied-1]

		// Apply the command (create a block)
		r.applyCommand(entry.Command)
	}
}

// applyCommand applies a command to the state machine
func (r *RaftNode) applyCommand(command interface{}) {
	// Convert command to block
	// This is a simplified implementation
	header := &block.Header{
		Height:     int(r.LastApplied),
		Index:      uint64(r.LastApplied),
		Node:       r.NodeID,
		Ctx:        17,
		Difficulty: 11111111111,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		GasLimit:   250000,
		GasUsed:    1,
		ChainId:    11,
		Size:       0,
		Timestamp:  uint64(time.Now().Unix()),
		V:          [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x1, 0x0},
		Nonce:      uint64(r.LastApplied),
	}

	newBlock := block.NewBlock(header)

	// Send to engine
	if r.engine != nil {
		r.engine.BlockFunnel <- newBlock
	}

	fmt.Printf("Applied command at index %d\n", r.LastApplied)
}

// SubmitRequest submits a client request to the consensus
func (r *RaftNode) SubmitRequest(command interface{}) {
	req := &ClientRequest{
		Command: command,
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	r.ClientRequestChan <- req
}

// GetConsensusState returns the current consensus state
func (r *RaftNode) GetConsensusState() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"nodeID":      r.NodeID.Hex(),
		"state":       r.State.String(),
		"currentTerm": r.CurrentTerm,
		"commitIndex": r.CommitIndex,
		"lastApplied": r.LastApplied,
		"logLength":   len(r.Log),
		"peers":       len(r.Peers),
		"leaderID":    r.LeaderID,
		"votedFor":    r.VotedFor,
	}
}

// Network message handlers
func (r *RaftNode) handleRequestVoteFromNetwork(req *RequestVoteRequest) {
	// Send to internal channel for processing
	select {
	case r.RequestVoteChan <- req:
	default:
		fmt.Printf("Request vote channel full, dropping message\n")
	}
}

func (r *RaftNode) handleRequestVoteResponseFromNetwork(resp *RequestVoteResponse) {
	// Send to internal channel for processing
	select {
	case r.RequestVoteRespChan <- resp:
	default:
		fmt.Printf("Request vote response channel full, dropping message\n")
	}
}

func (r *RaftNode) handleAppendEntriesFromNetwork(req *AppendEntriesRequest) {
	// Send to internal channel for processing
	select {
	case r.AppendEntriesChan <- req:
	default:
		fmt.Printf("Append entries channel full, dropping message\n")
	}
}

func (r *RaftNode) handleAppendEntriesResponseFromNetwork(resp *AppendEntriesResponse) {
	// Send to internal channel for processing
	select {
	case r.AppendEntriesRespChan <- resp:
	default:
		fmt.Printf("Append entries response channel full, dropping message\n")
	}
}
