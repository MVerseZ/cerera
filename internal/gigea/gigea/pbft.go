package gigea

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/message"
	"github.com/cerera/internal/cerera/types"
)

// PBFTNode represents a node in the PBFT consensus
type PBFTNode struct {
	NodeID   types.Address
	ViewID   int64
	Sequence int64
	Primary  types.Address
	Replicas []types.Address
	F        int // maximum number of faulty nodes

	// Message logs
	PrePrepareLog map[string]*message.PrePrepare
	PrepareLog    map[string]map[types.Address]*message.Prepare
	CommitLog     map[string]map[types.Address]*message.Commit
	RequestPool   map[string]*message.Request

	// State
	CurrentView   int64
	LastExecuted  int64
	CheckpointSeq int64

	// Channels
	RequestChan    chan *message.Request
	PrePrepareChan chan *message.PrePrepare
	PrepareChan    chan *message.Prepare
	CommitChan     chan *message.Commit
	ViewChangeChan chan *message.ViewChange

	// Mutex for thread safety
	mu sync.RWMutex

	// Engine reference
	engine *Engine
}

// NewPBFTNode creates a new PBFT node
func NewPBFTNode(nodeID types.Address, replicas []types.Address, engine *Engine) *PBFTNode {
	f := (len(replicas) - 1) / 3 // maximum faulty nodes

	return &PBFTNode{
		NodeID:         nodeID,
		ViewID:         0,
		Sequence:       0,
		Primary:        replicas[0], // primary is the first replica
		Replicas:       replicas,
		F:              f,
		PrePrepareLog:  make(map[string]*message.PrePrepare),
		PrepareLog:     make(map[string]map[types.Address]*message.Prepare),
		CommitLog:      make(map[string]map[types.Address]*message.Commit),
		RequestPool:    make(map[string]*message.Request),
		CurrentView:    0,
		LastExecuted:   0,
		CheckpointSeq:  0,
		RequestChan:    make(chan *message.Request, 100),
		PrePrepareChan: make(chan *message.PrePrepare, 100),
		PrepareChan:    make(chan *message.Prepare, 100),
		CommitChan:     make(chan *message.Commit, 100),
		ViewChangeChan: make(chan *message.ViewChange, 100),
		engine:         engine,
	}
}

// Start starts the PBFT consensus
func (p *PBFTNode) Start() {
	go p.handleRequests()
	go p.handlePrePrepare()
	go p.handlePrepare()
	go p.handleCommit()
	go p.handleViewChange()
}

// IsPrimary checks if this node is the primary
func (p *PBFTNode) IsPrimary() bool {
	return p.NodeID == p.Primary
}

// GetPrimary returns the current primary
func (p *PBFTNode) GetPrimary() types.Address {
	return p.Primary
}

// handleRequests handles incoming client requests
func (p *PBFTNode) handleRequests() {
	for req := range p.RequestChan {
		p.mu.Lock()
		p.RequestPool[req.Operation] = req
		p.mu.Unlock()

		if p.IsPrimary() {
			p.handleClientRequest(req)
		}
	}
}

// handleClientRequest handles a client request (primary only)
func (p *PBFTNode) handleClientRequest(req *message.Request) {
	p.Sequence++

	// Create pre-prepare message
	prePrepare := &message.PrePrepare{
		ViewID:     p.ViewID,
		SequenceID: p.Sequence,
		Digest:     p.generateDigest(req),
	}

	// Store in log
	p.mu.Lock()
	p.PrePrepareLog[prePrepare.Digest] = prePrepare
	p.mu.Unlock()

	// Broadcast pre-prepare message
	p.broadcastPrePrepare(prePrepare)

	// Also prepare locally
	p.handlePrePrepareMessage(prePrepare)
}

// handlePrePrepare handles pre-prepare messages
func (p *PBFTNode) handlePrePrepare() {
	for prePrepare := range p.PrePrepareChan {
		p.handlePrePrepareMessage(prePrepare)
	}
}

// handlePrePrepareMessage processes a pre-prepare message
func (p *PBFTNode) handlePrePrepareMessage(prePrepare *message.PrePrepare) {
	// Verify view and sequence
	if prePrepare.ViewID != p.ViewID {
		return
	}

	// Verify primary
	if p.Replicas[prePrepare.ViewID%int64(len(p.Replicas))] != p.NodeID {
		return
	}

	// Store in log
	p.mu.Lock()
	p.PrePrepareLog[prePrepare.Digest] = prePrepare
	p.mu.Unlock()

	// Create and broadcast prepare message
	prepare := &message.Prepare{
		ViewID:     prePrepare.ViewID,
		SequenceID: prePrepare.SequenceID,
		Digest:     prePrepare.Digest,
		NodeID:     int64(p.NodeID[0]), // Simplified node ID
	}

	p.broadcastPrepare(prepare)
	p.handlePrepareMessage(prepare)
}

// handlePrepare handles prepare messages
func (p *PBFTNode) handlePrepare() {
	for prepare := range p.PrepareChan {
		p.handlePrepareMessage(prepare)
	}
}

// handlePrepareMessage processes a prepare message
func (p *PBFTNode) handlePrepareMessage(prepare *message.Prepare) {
	// Verify view and sequence
	if prepare.ViewID != p.ViewID {
		return
	}

	// Store in log
	p.mu.Lock()
	if p.PrepareLog[prepare.Digest] == nil {
		p.PrepareLog[prepare.Digest] = make(map[types.Address]*message.Prepare)
	}
	p.PrepareLog[prepare.Digest][p.NodeID] = prepare
	p.mu.Unlock()

	// Check if we have enough prepare messages
	if p.hasEnoughPrepares(prepare.Digest) {
		// Create and broadcast commit message
		commit := &message.Commit{
			ViewID:     prepare.ViewID,
			SequenceID: prepare.SequenceID,
			Digest:     prepare.Digest,
			NodeID:     int64(p.NodeID[0]), // Simplified node ID
		}

		p.broadcastCommit(commit)
		p.handleCommitMessage(commit)
	}
}

// handleCommit handles commit messages
func (p *PBFTNode) handleCommit() {
	for commit := range p.CommitChan {
		p.handleCommitMessage(commit)
	}
}

// handleCommitMessage processes a commit message
func (p *PBFTNode) handleCommitMessage(commit *message.Commit) {
	// Verify view and sequence
	if commit.ViewID != p.ViewID {
		return
	}

	// Store in log
	p.mu.Lock()
	if p.CommitLog[commit.Digest] == nil {
		p.CommitLog[commit.Digest] = make(map[types.Address]*message.Commit)
	}
	p.CommitLog[commit.Digest][p.NodeID] = commit
	p.mu.Unlock()

	// Check if we have enough commit messages
	if p.hasEnoughCommits(commit.Digest) {
		p.executeRequest(commit.Digest)
	}
}

// handleViewChange handles view change messages
func (p *PBFTNode) handleViewChange() {
	for viewChange := range p.ViewChangeChan {
		// Handle view change logic
		fmt.Printf("View change request to view %d\n", viewChange.NewViewID)
	}
}

// hasEnoughPrepares checks if we have enough prepare messages
func (p *PBFTNode) hasEnoughPrepares(digest string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := len(p.PrepareLog[digest])
	return count >= 2*p.F+1
}

// hasEnoughCommits checks if we have enough commit messages
func (p *PBFTNode) hasEnoughCommits(digest string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := len(p.CommitLog[digest])
	return count >= 2*p.F+1
}

// executeRequest executes a committed request
func (p *PBFTNode) executeRequest(digest string) {
	p.mu.Lock()
	req, exists := p.RequestPool[digest]
	p.mu.Unlock()

	if !exists {
		return
	}

	// Execute the request (in this case, create a block)
	fmt.Printf("Executing request: %s\n", req.Operation)

	// Create a new block with the transaction
	// This is a simplified implementation
	header := &block.Header{
		Height:     int(p.LastExecuted + 1),
		Index:      uint64(p.Sequence),
		Node:       p.NodeID,
		Ctx:        17,
		Difficulty: 11111111111,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		GasLimit:   250000,
		GasUsed:    1,
		ChainId:    11,
		Size:       0,
		Timestamp:  uint64(time.Now().Unix()),
		V:          [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x1, 0x0},
		Nonce:      uint64(p.Sequence),
	}

	newBlock := block.NewBlock(header)

	// Send to engine
	if p.engine != nil {
		p.engine.BlockFunnel <- newBlock
	}

	p.LastExecuted++
}

// generateDigest generates a digest for a request
func (p *PBFTNode) generateDigest(req *message.Request) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// broadcastPrePrepare broadcasts a pre-prepare message
func (p *PBFTNode) broadcastPrePrepare(prePrepare *message.PrePrepare) {
	message.CreateConMsg(message.MTPrePrepare, prePrepare)
	// TODO: Implement actual broadcasting
	fmt.Printf("Broadcasting pre-prepare: %+v\n", prePrepare)
}

// broadcastPrepare broadcasts a prepare message
func (p *PBFTNode) broadcastPrepare(prepare *message.Prepare) {
	message.CreateConMsg(message.MTPrepare, prepare)
	// TODO: Implement actual broadcasting
	fmt.Printf("Broadcasting prepare: %+v\n", prepare)
}

// broadcastCommit broadcasts a commit message
func (p *PBFTNode) broadcastCommit(commit *message.Commit) {
	message.CreateConMsg(message.MTCommit, commit)
	// TODO: Implement actual broadcasting
	fmt.Printf("Broadcasting commit: %+v\n", commit)
}

// SubmitRequest submits a client request to the consensus
func (p *PBFTNode) SubmitRequest(operation string) {
	req := &message.Request{
		SeqID:     p.Sequence + 1,
		TimeStamp: time.Now().Unix(),
		ClientID:  p.NodeID.Hex(),
		Operation: operation,
	}

	p.RequestChan <- req
}

// GetConsensusState returns the current consensus state
func (p *PBFTNode) GetConsensusState() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"nodeID":        p.NodeID.Hex(),
		"viewID":        p.ViewID,
		"sequence":      p.Sequence,
		"primary":       p.Primary.Hex(),
		"isPrimary":     p.IsPrimary(),
		"lastExecuted":  p.LastExecuted,
		"replicas":      len(p.Replicas),
		"faultyNodes":   p.F,
		"requestPool":   len(p.RequestPool),
		"prePrepareLog": len(p.PrePrepareLog),
	}
}
