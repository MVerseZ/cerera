package mesh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/protocol"
)

// Gossip manages gossip protocol for block and message propagation
type Gossip struct {
	mu          sync.RWMutex
	ctx         context.Context
	peerStore   *PeerStore
	connManager *connection.Manager
	encoder     *protocol.Encoder
	started     bool
	
	// Gossip state
	seenBlocks   map[string]time.Time
	seenMessages map[string]time.Time
	ttl          time.Duration
}

// NewGossip creates a new gossip instance
func NewGossip(ctx context.Context, peerStore *PeerStore, connManager *connection.Manager) *Gossip {
	return &Gossip{
		ctx:         ctx,
		peerStore:   peerStore,
		connManager: connManager,
		encoder:     protocol.NewEncoder(),
		seenBlocks:  make(map[string]time.Time),
		seenMessages: make(map[string]time.Time),
		ttl:         10 * time.Minute,
	}
}

// Start starts the gossip protocol
func (g *Gossip) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.started {
		return fmt.Errorf("gossip already started")
	}
	
	// Start cleanup goroutine
	go g.cleanupSeen()
	
	g.started = true
	return nil
}

// Stop stops the gossip protocol
func (g *Gossip) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.started = false
}

// cleanupSeen periodically cleans up old seen items
func (g *Gossip) cleanupSeen() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.mu.Lock()
			now := time.Now()
			for id, seenTime := range g.seenBlocks {
				if now.Sub(seenTime) > g.ttl {
					delete(g.seenBlocks, id)
				}
			}
			for id, seenTime := range g.seenMessages {
				if now.Sub(seenTime) > g.ttl {
					delete(g.seenMessages, id)
				}
			}
			g.mu.Unlock()
		}
	}
}

// BroadcastBlock broadcasts a block to connected peers using gossip
func (g *Gossip) BroadcastBlock(b *block.Block) error {
	if b == nil {
		return fmt.Errorf("block cannot be nil")
	}
	
	blockID := b.Hash.Hex()
	
	// Check if we've already seen this block
	g.mu.RLock()
	if _, seen := g.seenBlocks[blockID]; seen {
		g.mu.RUnlock()
		return nil // Already propagated
	}
	g.mu.RUnlock()
	
	// Mark as seen
	g.mu.Lock()
	g.seenBlocks[blockID] = time.Now()
	g.mu.Unlock()
	
	// Record metrics
	metrics.Get().BlocksBroadcastTotal.WithLabelValues("gossip").Inc()
	
	// Create block message
	msg := &protocol.BlockMessage{
		Block: b,
	}
	
	// Broadcast to connected peers
	return g.broadcastToPeers(msg)
}

// BroadcastMessage broadcasts a protocol message to connected peers
func (g *Gossip) BroadcastMessage(msg protocol.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}
	
	msgID := fmt.Sprintf("%s_%d", msg.Type(), time.Now().Unix())
	
	// Check if we've already seen this message
	g.mu.RLock()
	if _, seen := g.seenMessages[msgID]; seen {
		g.mu.RUnlock()
		return nil
	}
	g.mu.RUnlock()
	
	// Mark as seen
	g.mu.Lock()
	g.seenMessages[msgID] = time.Now()
	g.mu.Unlock()
	
	return g.broadcastToPeers(msg)
}

// broadcastToPeers broadcasts a message to all connected peers
func (g *Gossip) broadcastToPeers(msg protocol.Message) error {
	peers := g.peerStore.GetConnected()
	
	errors := make([]error, 0)
	for _, peer := range peers {
		conn, ok := g.connManager.GetConnectionByAddress(peer.Address)
		if !ok {
			continue
		}
		
		handler := g.connManager.GetHandler()
		if err := handler.WriteMessage(conn, msg); err != nil {
			errors = append(errors, fmt.Errorf("failed to send to %s: %w", peer.Address.Hex(), err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("some broadcasts failed: %d errors", len(errors))
	}
	
	return nil
}

// HandleReceivedBlock handles a received block (for deduplication)
func (g *Gossip) HandleReceivedBlock(blockID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if _, seen := g.seenBlocks[blockID]; seen {
		return false // Already seen
	}
	
	g.seenBlocks[blockID] = time.Now()
	return true // New block
}

// HandleReceivedMessage handles a received message (for deduplication)
func (g *Gossip) HandleReceivedMessage(msgID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if _, seen := g.seenMessages[msgID]; seen {
		return false // Already seen
	}
	
	g.seenMessages[msgID] = time.Now()
	return true // New message
}

