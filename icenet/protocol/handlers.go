package protocol

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// StreamTimeout is the timeout for stream operations
	StreamTimeout = 30 * time.Second
	// HandshakeTimeout is the timeout for handshake operations
	HandshakeTimeout = 10 * time.Second
)

func protocolLogger() *zap.SugaredLogger {
	return logger.Named("protocol")
}

// Handler handles protocol messages
type Handler struct {
	host            host.Host
	serviceProvider service.ServiceProvider
	nodeAddr        types.Address
	version         string

	// Callbacks for specific message types
	onNewBlock     func(*block.Block, peer.ID)
	onNewTx        func(*types.GTransaction, peer.ID)
	onConsensusMsg func(*ConsensusMessage, peer.ID)
	onVote         func(*VoteMessage, peer.ID)
}

// NewHandler creates a new protocol handler
func NewHandler(h host.Host, serviceProvider service.ServiceProvider, nodeAddr types.Address, version string) *Handler {
	return &Handler{
		host:            h,
		serviceProvider: serviceProvider,
		nodeAddr:        nodeAddr,
		version:         version,
	}
}

// RegisterHandlers registers all protocol handlers with the host
func (h *Handler) RegisterHandlers() {
	h.host.SetStreamHandler(StatusProtocolID, h.handleStatusStream)
	h.host.SetStreamHandler(SyncProtocolID, h.handleSyncStream)
	h.host.SetStreamHandler(BlockProtocolID, h.handleBlockStream)
	h.host.SetStreamHandler(TxProtocolID, h.handleTxStream)
	h.host.SetStreamHandler(ConsensusProtocolID, h.handleConsensusStream)
	h.host.SetStreamHandler(PingProtocolID, h.handlePingStream)

	protocolLogger().Debugw("Protocol handlers registered",
		"protocols", AllProtocols(),
	)
}

// SetOnNewBlock sets the callback for new block messages
func (h *Handler) SetOnNewBlock(callback func(*block.Block, peer.ID)) {
	h.onNewBlock = callback
}

// SetOnNewTx sets the callback for new transaction messages
func (h *Handler) SetOnNewTx(callback func(*types.GTransaction, peer.ID)) {
	h.onNewTx = callback
}

// SetOnConsensusMsg sets the callback for consensus messages
func (h *Handler) SetOnConsensusMsg(callback func(*ConsensusMessage, peer.ID)) {
	h.onConsensusMsg = callback
}

// SetOnVote sets the callback for vote messages
func (h *Handler) SetOnVote(callback func(*VoteMessage, peer.ID)) {
	h.onVote = callback
}

// handleStatusStream handles incoming status protocol streams
func (h *Handler) handleStatusStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()
	protocolLogger().Debugw("Status stream opened", "peer", remotePeer)

	// Set deadline
	if err := s.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	// Read request
	decoder := NewDecoder(s)
	msg, err := decoder.Decode()
	if err != nil {
		protocolLogger().Warnw("Failed to decode status request", "peer", remotePeer, "error", err)
		return
	}

	statusReq, ok := msg.(*StatusRequest)
	if !ok {
		protocolLogger().Warnw("Unexpected message type", "peer", remotePeer, "type", msg.Type())
		return
	}

	// Create and send response

	// Check chain compatibility
	// if statusReq.GenesisHash != genesisHash && genesisHash != (common.Hash{}) {
	// 	protocolLogger().Warnw("Incompatible chain",
	// 		"peer", remotePeer,
	// 		"peerGenesis", statusReq.GenesisHash,
	// 	)
	// }

	height := 0
	chainID := statusReq.ChainID

	if h.serviceProvider != nil {
		height = h.serviceProvider.GetCurrentHeight()
		chainID = h.serviceProvider.GetChainID()
	}

	status, err := GetStatus(h.serviceProvider)
	if err != nil {
		protocolLogger().Warnw("Failed to get status", "error", err)
		return
	}

	response := NewStatusResponse(
		chainID,
		h.version,
		height,
		status,
		h.nodeAddr,
		len(h.host.Network().Peers()),
	)

	encoder := NewEncoder(s)
	if err := encoder.Encode(response); err != nil {
		protocolLogger().Warnw("Failed to encode status response", "peer", remotePeer, "error", err)
		return
	}

	protocolLogger().Debugw("Status exchange complete",
		"peer", remotePeer,
		"peerChainID", statusReq.ChainID,
		"ourHeight", height,
	)
}

// handleSyncStream handles incoming sync protocol streams
func (h *Handler) handleSyncStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()
	protocolLogger().Debugw("Sync stream opened", "peer", remotePeer)

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	decoder := NewDecoder(s)
	encoder := NewEncoder(s)

	for {
		msg, err := decoder.Decode()
		if err != nil {
			if err != io.EOF {
				protocolLogger().Debugw("Sync stream ended", "peer", remotePeer, "error", err)
			}
			return
		}

		switch m := msg.(type) {
		case *SyncRequest:
			h.handleSyncRequest(m, encoder, remotePeer)
		case *GetBlocksRequest:
			h.handleGetBlocksRequest(m, encoder, remotePeer)
		default:
			protocolLogger().Warnw("Unexpected message in sync stream", "type", msg.Type())
		}
	}
}

// handleSyncRequest handles a sync request
func (h *Handler) handleSyncRequest(req *SyncRequest, encoder *Encoder, peer peer.ID) {
	height := 0
	latestHash := common.Hash{}

	if h.serviceProvider != nil {
		height = h.serviceProvider.GetCurrentHeight()
		latestHash = h.serviceProvider.GetLatestHash()
	}

	response := NewSyncResponse(true, height, latestHash, "")
	if err := encoder.Encode(response); err != nil {
		protocolLogger().Warnw("Failed to send sync response", "peer", peer, "error", err)
	}
}

// handleGetBlocksRequest handles a get blocks request
func (h *Handler) handleGetBlocksRequest(req *GetBlocksRequest, encoder *Encoder, peer peer.ID) {
	if h.serviceProvider == nil {
		response := NewBlocksResponse(nil, req.StartHeight, 0, false)
		encoder.Encode(response)
		return
	}

	blocks := make([]*block.Block, 0, req.Count)
	currentHeight := h.serviceProvider.GetCurrentHeight()

	for i := 0; i < req.Count && req.StartHeight+i <= currentHeight; i++ {
		b := h.serviceProvider.GetBlockByHeight(req.StartHeight + i)
		if b != nil {
			blocks = append(blocks, b)
		}
	}

	hasMore := req.StartHeight+req.Count <= currentHeight

	response := NewBlocksResponse(blocks, req.StartHeight, len(blocks), hasMore)
	if err := encoder.Encode(response); err != nil {
		protocolLogger().Warnw("Failed to send blocks response", "peer", peer, "error", err)
	}

	protocolLogger().Debugw("Sent blocks",
		"peer", peer,
		"startHeight", req.StartHeight,
		"count", len(blocks),
		"hasMore", hasMore,
	)
}

// handleBlockStream handles incoming block protocol streams
func (h *Handler) handleBlockStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	decoder := NewDecoder(s)

	for {
		msg, err := decoder.Decode()
		if err != nil {
			if err != io.EOF {
				protocolLogger().Debugw("Block stream ended", "peer", remotePeer, "error", err)
			}
			return
		}

		switch m := msg.(type) {
		case *NewBlockMessage:
			if h.onNewBlock != nil && m.Block != nil {
				h.onNewBlock(m.Block, remotePeer)
			}
		case *NewBlockHashMessage:
			// TODO: request full block if we don't have it
			protocolLogger().Debugw("Received block hash", "peer", remotePeer, "hash", m.Hash, "height", m.Height)
		}
	}
}

// handleTxStream handles incoming transaction protocol streams
func (h *Handler) handleTxStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	decoder := NewDecoder(s)

	for {
		msg, err := decoder.Decode()
		if err != nil {
			if err != io.EOF {
				protocolLogger().Debugw("Tx stream ended", "peer", remotePeer, "error", err)
			}
			return
		}

		switch m := msg.(type) {
		case *NewTxMessage:
			if h.onNewTx != nil && m.Tx != nil {
				h.onNewTx(m.Tx, remotePeer)
			}
		}
	}
}

// handleConsensusStream handles incoming consensus protocol streams
func (h *Handler) handleConsensusStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	decoder := NewDecoder(s)

	for {
		msg, err := decoder.Decode()
		if err != nil {
			if err != io.EOF {
				protocolLogger().Debugw("Consensus stream ended", "peer", remotePeer, "error", err)
			}
			return
		}

		switch m := msg.(type) {
		case *ConsensusMessage:
			if h.onConsensusMsg != nil {
				h.onConsensusMsg(m, remotePeer)
			}
		case *VoteMessage:
			if h.onVote != nil {
				h.onVote(m, remotePeer)
			}
		}
	}
}

// handlePingStream handles incoming ping protocol streams
func (h *Handler) handlePingStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()

	if err := s.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		protocolLogger().Warnw("Failed to set stream deadline", "error", err)
		return
	}

	decoder := NewDecoder(s)
	encoder := NewEncoder(s)

	msg, err := decoder.Decode()
	if err != nil {
		protocolLogger().Debugw("Failed to decode ping", "peer", remotePeer, "error", err)
		return
	}

	ping, ok := msg.(*PingMessage)
	if !ok {
		return
	}

	pong := NewPongMessage(ping.Nonce)
	if err := encoder.Encode(pong); err != nil {
		protocolLogger().Debugw("Failed to send pong", "peer", remotePeer, "error", err)
	}
}

// RequestStatus requests status from a peer
func (h *Handler) RequestStatus(ctx context.Context, peerID peer.ID) (*StatusResponse, error) {
	s, err := h.host.NewStream(ctx, peerID, StatusProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open status stream: %w", err)
	}
	defer s.Close()
	protocolLogger().Debugw("Requesting status from peer", "peer", peerID)

	if err := s.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	genesisHash := h.serviceProvider.GetBlockByHeight(0).Hash
	chainID := h.serviceProvider.GetChainID()

	request := NewStatusRequest(chainID, h.version, genesisHash, h.nodeAddr)
	protocolLogger().Debugw("Sending status request", "request", request)

	encoder := NewEncoder(s)
	if err := encoder.Encode(request); err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}

	decoder := NewDecoder(s)
	msg, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to receive status response: %w", err)
	}

	response, ok := msg.(*StatusResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", msg)
	}

	protocolLogger().Debugw("Received status response", "response", response)

	return response, nil
}

// RequestBlocks requests blocks from a peer
func (h *Handler) RequestBlocks(ctx context.Context, peerID peer.ID, startHeight, count int) ([]*block.Block, error) {
	s, err := h.host.NewStream(ctx, peerID, SyncProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open sync stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	request := NewGetBlocksRequest(startHeight, count)

	encoder := NewEncoder(s)
	if err := encoder.Encode(request); err != nil {
		return nil, fmt.Errorf("failed to send blocks request: %w", err)
	}

	decoder := NewDecoder(s)
	msg, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to receive blocks response: %w", err)
	}

	response, ok := msg.(*BlocksResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", msg)
	}

	return response.Blocks, nil
}

// BroadcastNewBlock sends a new block to a peer
func (h *Handler) BroadcastNewBlock(ctx context.Context, peerID peer.ID, b *block.Block) error {
	s, err := h.host.NewStream(ctx, peerID, BlockProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open block stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	msg := NewNewBlockMessage(b)
	encoder := NewEncoder(s)
	return encoder.Encode(msg)
}

// BroadcastNewTx sends a new transaction to a peer
func (h *Handler) BroadcastNewTx(ctx context.Context, peerID peer.ID, tx *types.GTransaction) error {
	s, err := h.host.NewStream(ctx, peerID, TxProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open tx stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	msg := NewNewTxMessage(tx)
	encoder := NewEncoder(s)
	return encoder.Encode(msg)
}

// SendConsensusMessage sends a consensus message to a peer
func (h *Handler) SendConsensusMessage(ctx context.Context, peerID peer.ID, msg *ConsensusMessage) error {
	s, err := h.host.NewStream(ctx, peerID, ConsensusProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open consensus stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	encoder := NewEncoder(s)
	return encoder.Encode(msg)
}

// Ping sends a ping to a peer and waits for pong
func (h *Handler) Ping(ctx context.Context, peerID peer.ID) (time.Duration, error) {
	start := time.Now()

	s, err := h.host.NewStream(ctx, peerID, PingProtocolID)
	if err != nil {
		return 0, fmt.Errorf("failed to open ping stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(HandshakeTimeout)); err != nil {
		return 0, fmt.Errorf("failed to set deadline: %w", err)
	}

	nonce := uint64(time.Now().UnixNano())
	ping := NewPingMessage(nonce)

	encoder := NewEncoder(s)
	if err := encoder.Encode(ping); err != nil {
		return 0, fmt.Errorf("failed to send ping: %w", err)
	}

	decoder := NewDecoder(s)
	msg, err := decoder.Decode()
	if err != nil {
		return 0, fmt.Errorf("failed to receive pong: %w", err)
	}

	pong, ok := msg.(*PongMessage)
	if !ok {
		return 0, fmt.Errorf("unexpected response type: %T", msg)
	}

	if pong.Nonce != nonce {
		return 0, fmt.Errorf("nonce mismatch: expected %d, got %d", nonce, pong.Nonce)
	}

	return time.Since(start), nil
}
