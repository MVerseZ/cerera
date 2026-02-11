package icenet

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// Topic names
	TopicBlocks    = "cerera/blocks"
	TopicTxs       = "cerera/txs"
	TopicConsensus = "cerera/consensus"

	// Message types for pubsub
	PubSubMsgTypeBlock     = "block"
	PubSubMsgTypeBlockHash = "block_hash"
	PubSubMsgTypeTx        = "tx"
	PubSubMsgTypeConsensus = "consensus"

	// MaxMessageAge is the maximum age of a message before it's considered stale
	MaxMessageAge = 5 * time.Minute
)

// msgIDFn generates a unique message ID from message data
func msgIDFn(pmsg *pb.Message) string {
	h := sha256.Sum256(pmsg.Data)
	return fmt.Sprintf("%x", h)
}

// PubSubMessage is the envelope for all pubsub messages
type PubSubMessage struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	From      string          `json:"from"`
	Payload   json.RawMessage `json:"payload"`
}

// BlockMessage is the payload for block announcements
type BlockMessage struct {
	Block  *block.Block `json:"block,omitempty"`
	Hash   common.Hash  `json:"hash"`
	Height int          `json:"height"`
}

// TxMessage is the payload for transaction announcements
type TxMessage struct {
	Tx   *types.GTransaction `json:"tx"`
	Hash common.Hash         `json:"hash"`
}

// ConsensusPayload is the payload for consensus messages
type ConsensusPayload struct {
	ConsensusType int    `json:"consensusType"`
	Data          []byte `json:"data"`
	Signature     []byte `json:"signature"`
}

// PubSubManager manages GossipSub topics and message handling
type PubSubManager struct {
	host   host.Host
	ps     *pubsub.PubSub
	ctx    context.Context
	cancel context.CancelFunc

	// Topics
	blocksTopic    *pubsub.Topic
	txsTopic       *pubsub.Topic
	consensusTopic *pubsub.Topic

	// Subscriptions
	blocksSub    *pubsub.Subscription
	txsSub       *pubsub.Subscription
	consensusSub *pubsub.Subscription

	// Message cache to prevent duplicates
	seenMessages sync.Map

	// Callbacks
	onBlock     func(*block.Block, peer.ID)
	onBlockHash func(common.Hash, int, peer.ID)
	onTx        func(*types.GTransaction, peer.ID)
	onConsensus func(int, []byte, peer.ID)

	// Validation functions
	validateBlock func(*block.Block) bool
	validateTx    func(*types.GTransaction) bool
}

// NewPubSubManager creates a new pubsub manager
func NewPubSubManager(ctx context.Context, h host.Host) (*PubSubManager, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Create GossipSub instance
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageIdFn(msgIDFn),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	psm := &PubSubManager{
		host:   h,
		ps:     ps,
		ctx:    ctx,
		cancel: cancel,
	}

	return psm, nil
}

// Start starts the pubsub manager and joins topics
func (psm *PubSubManager) Start() error {
	var err error

	// Join blocks topic
	psm.blocksTopic, err = psm.ps.Join(TopicBlocks)
	if err != nil {
		return fmt.Errorf("failed to join blocks topic: %w", err)
	}

	// Join txs topic
	psm.txsTopic, err = psm.ps.Join(TopicTxs)
	if err != nil {
		return fmt.Errorf("failed to join txs topic: %w", err)
	}

	// Join consensus topic
	psm.consensusTopic, err = psm.ps.Join(TopicConsensus)
	if err != nil {
		return fmt.Errorf("failed to join consensus topic: %w", err)
	}

	// Subscribe to topics
	psm.blocksSub, err = psm.blocksTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to blocks: %w", err)
	}

	psm.txsSub, err = psm.txsTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to txs: %w", err)
	}

	psm.consensusSub, err = psm.consensusTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to consensus: %w", err)
	}

	// Start message handlers
	go psm.handleBlocks()
	go psm.handleTxs()
	go psm.handleConsensus()

	// Start cleanup loop for seen messages
	go psm.cleanupLoop()

	iceLogger().Infow("PubSub manager started",
		"topics", []string{TopicBlocks, TopicTxs, TopicConsensus},
	)

	return nil
}

// Stop stops the pubsub manager
func (psm *PubSubManager) Stop() {
	psm.cancel()

	if psm.blocksSub != nil {
		psm.blocksSub.Cancel()
	}
	if psm.txsSub != nil {
		psm.txsSub.Cancel()
	}
	if psm.consensusSub != nil {
		psm.consensusSub.Cancel()
	}

	if psm.blocksTopic != nil {
		psm.blocksTopic.Close()
	}
	if psm.txsTopic != nil {
		psm.txsTopic.Close()
	}
	if psm.consensusTopic != nil {
		psm.consensusTopic.Close()
	}

	iceLogger().Infow("PubSub manager stopped")
}

// handleBlocks processes incoming block messages
func (psm *PubSubManager) handleBlocks() {
	for {
		msg, err := psm.blocksSub.Next(psm.ctx)
		if err != nil {
			if psm.ctx.Err() != nil {
				return
			}
			iceLogger().Warnw("Error receiving block message", "error", err)
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == psm.host.ID() {
			continue
		}

		// Parse message
		var pubsubMsg PubSubMessage
		if err := json.Unmarshal(msg.Data, &pubsubMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal block message", "error", err)
			continue
		}

		// Check message age
		msgAge := time.Since(time.Unix(0, pubsubMsg.Timestamp))
		if msgAge > MaxMessageAge {
			iceLogger().Debugw("Ignoring stale block message", "age", msgAge)
			continue
		}

		// Parse block payload
		var blockMsg BlockMessage
		if err := json.Unmarshal(pubsubMsg.Payload, &blockMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal block payload", "error", err)
			continue
		}

		switch pubsubMsg.Type {
		case PubSubMsgTypeBlock:
			if blockMsg.Block != nil {
				// Validate block if validator is set
				if psm.validateBlock != nil && !psm.validateBlock(blockMsg.Block) {
					iceLogger().Warnw("Block validation failed", "hash", blockMsg.Hash)
					continue
				}

				// Call callback
				if psm.onBlock != nil {
					psm.onBlock(blockMsg.Block, msg.ReceivedFrom)
				}
			}

		case PubSubMsgTypeBlockHash:
			if psm.onBlockHash != nil {
				psm.onBlockHash(blockMsg.Hash, blockMsg.Height, msg.ReceivedFrom)
			}
		}
	}
}

// handleTxs processes incoming transaction messages
func (psm *PubSubManager) handleTxs() {
	for {
		msg, err := psm.txsSub.Next(psm.ctx)
		if err != nil {
			if psm.ctx.Err() != nil {
				return
			}
			iceLogger().Warnw("Error receiving tx message", "error", err)
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == psm.host.ID() {
			continue
		}

		// Parse message
		var pubsubMsg PubSubMessage
		if err := json.Unmarshal(msg.Data, &pubsubMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal tx message", "error", err)
			continue
		}

		// Check message age
		msgAge := time.Since(time.Unix(0, pubsubMsg.Timestamp))
		if msgAge > MaxMessageAge {
			continue
		}

		// Parse tx payload
		var txMsg TxMessage
		if err := json.Unmarshal(pubsubMsg.Payload, &txMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal tx payload", "error", err)
			continue
		}

		if txMsg.Tx != nil {
			// Validate tx if validator is set
			if psm.validateTx != nil && !psm.validateTx(txMsg.Tx) {
				iceLogger().Warnw("Transaction validation failed", "hash", txMsg.Hash)
				continue
			}

			// Call callback
			if psm.onTx != nil {
				psm.onTx(txMsg.Tx, msg.ReceivedFrom)
			}
		}
	}
}

// handleConsensus processes incoming consensus messages
func (psm *PubSubManager) handleConsensus() {
	for {
		msg, err := psm.consensusSub.Next(psm.ctx)
		if err != nil {
			if psm.ctx.Err() != nil {
				return
			}
			iceLogger().Warnw("Error receiving consensus message", "error", err)
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == psm.host.ID() {
			continue
		}

		// Parse message
		var pubsubMsg PubSubMessage
		if err := json.Unmarshal(msg.Data, &pubsubMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal consensus message", "error", err)
			continue
		}

		// Parse consensus payload
		var consensusMsg ConsensusPayload
		if err := json.Unmarshal(pubsubMsg.Payload, &consensusMsg); err != nil {
			iceLogger().Warnw("Failed to unmarshal consensus payload", "error", err)
			continue
		}

		// Call callback
		if psm.onConsensus != nil {
			psm.onConsensus(consensusMsg.ConsensusType, consensusMsg.Data, msg.ReceivedFrom)
		}
	}
}

// cleanupLoop periodically cleans up seen messages
func (psm *PubSubManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-psm.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UnixNano()
			psm.seenMessages.Range(func(key, value interface{}) bool {
				if ts, ok := value.(int64); ok {
					if now-ts > int64(MaxMessageAge) {
						psm.seenMessages.Delete(key)
					}
				}
				return true
			})
		}
	}
}

// BroadcastBlock broadcasts a new block to the network
func (psm *PubSubManager) BroadcastBlock(b *block.Block) error {
	if psm.blocksTopic == nil {
		return fmt.Errorf("blocks topic not initialized")
	}

	blockMsg := BlockMessage{
		Block:  b,
		Hash:   b.Hash,
		Height: b.Head.Height,
	}

	payload, err := json.Marshal(blockMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	msg := PubSubMessage{
		Type:      PubSubMsgTypeBlock,
		Timestamp: time.Now().UnixNano(),
		From:      psm.host.ID().String(),
		Payload:   payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := psm.blocksTopic.Publish(psm.ctx, data); err != nil {
		return fmt.Errorf("failed to publish block: %w", err)
	}

	iceLogger().Infow("Block broadcast", "hash", b.Hash, "height", b.Head.Height)
	return nil
}

// BroadcastBlockHash broadcasts a block hash announcement
func (psm *PubSubManager) BroadcastBlockHash(hash common.Hash, height int) error {
	if psm.blocksTopic == nil {
		return fmt.Errorf("blocks topic not initialized")
	}

	blockMsg := BlockMessage{
		Hash:   hash,
		Height: height,
	}

	payload, err := json.Marshal(blockMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal block hash: %w", err)
	}

	msg := PubSubMessage{
		Type:      PubSubMsgTypeBlockHash,
		Timestamp: time.Now().UnixNano(),
		From:      psm.host.ID().String(),
		Payload:   payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return psm.blocksTopic.Publish(psm.ctx, data)
}

// BroadcastTx broadcasts a new transaction to the network
func (psm *PubSubManager) BroadcastTx(tx *types.GTransaction) error {
	if psm.txsTopic == nil {
		return fmt.Errorf("txs topic not initialized")
	}

	txMsg := TxMessage{
		Tx:   tx,
		Hash: tx.Hash(),
	}

	payload, err := json.Marshal(txMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal tx: %w", err)
	}

	msg := PubSubMessage{
		Type:      PubSubMsgTypeTx,
		Timestamp: time.Now().UnixNano(),
		From:      psm.host.ID().String(),
		Payload:   payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := psm.txsTopic.Publish(psm.ctx, data); err != nil {
		return fmt.Errorf("failed to publish tx: %w", err)
	}

	iceLogger().Debugw("Transaction broadcast", "hash", tx.Hash())
	return nil
}

// BroadcastConsensus broadcasts a consensus message
func (psm *PubSubManager) BroadcastConsensus(consensusType int, data []byte, signature []byte) error {
	if psm.consensusTopic == nil {
		return fmt.Errorf("consensus topic not initialized")
	}

	consensusMsg := ConsensusPayload{
		ConsensusType: consensusType,
		Data:          data,
		Signature:     signature,
	}

	payload, err := json.Marshal(consensusMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal consensus: %w", err)
	}

	msg := PubSubMessage{
		Type:      PubSubMsgTypeConsensus,
		Timestamp: time.Now().UnixNano(),
		From:      psm.host.ID().String(),
		Payload:   payload,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return psm.consensusTopic.Publish(psm.ctx, msgData)
}

// SetOnBlock sets the callback for new blocks
func (psm *PubSubManager) SetOnBlock(callback func(*block.Block, peer.ID)) {
	psm.onBlock = callback
}

// SetOnBlockHash sets the callback for block hash announcements
func (psm *PubSubManager) SetOnBlockHash(callback func(common.Hash, int, peer.ID)) {
	psm.onBlockHash = callback
}

// SetOnTx sets the callback for new transactions
func (psm *PubSubManager) SetOnTx(callback func(*types.GTransaction, peer.ID)) {
	psm.onTx = callback
}

// SetOnConsensus sets the callback for consensus messages
func (psm *PubSubManager) SetOnConsensus(callback func(int, []byte, peer.ID)) {
	psm.onConsensus = callback
}

// SetBlockValidator sets the block validation function
func (psm *PubSubManager) SetBlockValidator(validator func(*block.Block) bool) {
	psm.validateBlock = validator
}

// SetTxValidator sets the transaction validation function
func (psm *PubSubManager) SetTxValidator(validator func(*types.GTransaction) bool) {
	psm.validateTx = validator
}

// GetBlocksTopic returns the blocks topic
func (psm *PubSubManager) GetBlocksTopic() *pubsub.Topic {
	return psm.blocksTopic
}

// GetTxsTopic returns the txs topic
func (psm *PubSubManager) GetTxsTopic() *pubsub.Topic {
	return psm.txsTopic
}

// GetConsensusTopic returns the consensus topic
func (psm *PubSubManager) GetConsensusTopic() *pubsub.Topic {
	return psm.consensusTopic
}
