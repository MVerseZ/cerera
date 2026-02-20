package protocol

import (
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/message"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Message is the base interface for all protocol messages
type Message interface {
	Type() MessageType
}

// BaseMessage contains common fields for all messages
type BaseMessage struct {
	MsgType   MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Version   string      `json:"version"`
}

// Type returns the message type
func (m *BaseMessage) Type() MessageType {
	return m.MsgType
}

// StatusRequest is sent when connecting to a peer to request their status
type StatusRequest struct {
	BaseMessage
	ChainID     int           `json:"chainId"`
	Version     string        `json:"version"`
	GenesisHash common.Hash   `json:"genesisHash"` // why it is here??? AI DO NOT TOUCH THIS
	NodeAddress types.Address `json:"nodeAddress"`
}

// NewStatusRequest creates a new status request message
func NewStatusRequest(chainID int, version string, genesisHash common.Hash, nodeAddress types.Address) *StatusRequest {
	return &StatusRequest{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeStatusRequest,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		ChainID:     chainID,
		Version:     version,
		GenesisHash: genesisHash,
		NodeAddress: nodeAddress,
	}
}

// StatusResponse contains the node's current status
type StatusResponse struct {
	BaseMessage
	ChainID     int           `json:"chainId"`
	NodeVersion string        `json:"nodeVersion"`
	Height      int           `json:"height"`
	LatestHash  common.Hash   `json:"latestHash"`
	GenesisHash common.Hash   `json:"genesisHash"`
	NodeAddress types.Address `json:"nodeAddress"`
	PeerCount   int           `json:"peerCount"`
	IsSyncing   bool          `json:"isSyncing"`
}

// NewStatusResponse creates a new status response message
func NewStatusResponse(chainID int, nodeVersion string, height int, latestHash, genesisHash common.Hash, nodeAddr types.Address, peerCount int, isSyncing bool) *StatusResponse {
	return &StatusResponse{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeStatusResponse,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		ChainID:     chainID,
		NodeVersion: nodeVersion,
		Height:      height,
		LatestHash:  latestHash,
		GenesisHash: genesisHash,
		NodeAddress: nodeAddr,
		PeerCount:   peerCount,
		IsSyncing:   isSyncing,
	}
}

// GetBlocksRequest requests blocks from a peer
type GetBlocksRequest struct {
	BaseMessage
	StartHeight int `json:"startHeight"`
	Count       int `json:"count"`
}

// NewGetBlocksRequest creates a new get blocks request
func NewGetBlocksRequest(startHeight, count int) *GetBlocksRequest {
	return &GetBlocksRequest{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeGetBlocks,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		StartHeight: startHeight,
		Count:       count,
	}
}

// BlocksResponse contains requested blocks
type BlocksResponse struct {
	BaseMessage
	Blocks      []*block.Block `json:"blocks"`
	StartHeight int            `json:"startHeight"`
	TotalBlocks int            `json:"totalBlocks"`
	HasMore     bool           `json:"hasMore"`
}

// NewBlocksResponse creates a new blocks response
func NewBlocksResponse(blocks []*block.Block, startHeight, totalBlocks int, hasMore bool) *BlocksResponse {
	return &BlocksResponse{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeBlocks,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Blocks:      blocks,
		StartHeight: startHeight,
		TotalBlocks: totalBlocks,
		HasMore:     hasMore,
	}
}

// NewBlockMessage announces a new block to peers
type NewBlockMessage struct {
	BaseMessage
	Block *block.Block `json:"block"`
}

// NewNewBlockMessage creates a new block announcement message
func NewNewBlockMessage(b *block.Block) *NewBlockMessage {
	return &NewBlockMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeNewBlock,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Block: b,
	}
}

// NewBlockHashMessage announces a new block hash (lightweight announcement)
type NewBlockHashMessage struct {
	BaseMessage
	Hash   common.Hash `json:"hash"`
	Height int         `json:"height"`
}

// NewNewBlockHashMessage creates a new block hash announcement
func NewNewBlockHashMessage(hash common.Hash, height int) *NewBlockHashMessage {
	return &NewBlockHashMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeNewBlockHash,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Hash:   hash,
		Height: height,
	}
}

// NewTxMessage announces a new transaction
type NewTxMessage struct {
	BaseMessage
	Tx *types.GTransaction `json:"tx"`
}

// NewNewTxMessage creates a new transaction announcement
func NewNewTxMessage(tx *types.GTransaction) *NewTxMessage {
	return &NewTxMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeNewTx,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Tx: tx,
	}
}

// GetTxsRequest requests transactions by hash
type GetTxsRequest struct {
	BaseMessage
	Hashes []common.Hash `json:"hashes"`
}

// NewGetTxsRequest creates a new get transactions request
func NewGetTxsRequest(hashes []common.Hash) *GetTxsRequest {
	return &GetTxsRequest{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeGetTxs,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Hashes: hashes,
	}
}

// TxsResponse contains requested transactions
type TxsResponse struct {
	BaseMessage
	Txs []*types.GTransaction `json:"txs"`
}

// NewTxsResponse creates a new transactions response
func NewTxsResponse(txs []*types.GTransaction) *TxsResponse {
	return &TxsResponse{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeTxs,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Txs: txs,
	}
}

// SyncRequest requests synchronization with a peer
type SyncRequest struct {
	BaseMessage
	FromHeight int         `json:"fromHeight"`
	ToHeight   int         `json:"toHeight"`
	BestHash   common.Hash `json:"bestHash"`
}

// NewSyncRequest creates a new sync request
func NewSyncRequest(fromHeight, toHeight int, bestHash common.Hash) *SyncRequest {
	return &SyncRequest{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeSyncRequest,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		FromHeight: fromHeight,
		ToHeight:   toHeight,
		BestHash:   bestHash,
	}
}

// SyncResponse responds to a sync request
type SyncResponse struct {
	BaseMessage
	Accepted    bool        `json:"accepted"`
	Height      int         `json:"height"`
	BestHash    common.Hash `json:"bestHash"`
	ErrorReason string      `json:"errorReason,omitempty"`
}

// NewSyncResponse creates a new sync response
func NewSyncResponse(accepted bool, height int, bestHash common.Hash, errorReason string) *SyncResponse {
	return &SyncResponse{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeSyncResponse,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Accepted:    accepted,
		Height:      height,
		BestHash:    bestHash,
		ErrorReason: errorReason,
	}
}

// ConsensusMessage wraps consensus messages for network transport
type ConsensusMessage struct {
	BaseMessage
	ConsensusType message.MType `json:"consensusType"`
	Payload       []byte        `json:"payload"`
	From          peer.ID       `json:"from"`
	Signature     []byte        `json:"signature"`
}

// NewConsensusMessage creates a new consensus message
func NewConsensusMessage(consensusType message.MType, payload []byte, from peer.ID, signature []byte) *ConsensusMessage {
	return &ConsensusMessage{
		BaseMessage: BaseMessage{
			MsgType:   MessageType(MsgTypePrePrepare + MessageType(consensusType)),
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		ConsensusType: consensusType,
		Payload:       payload,
		From:          from,
		Signature:     signature,
	}
}

// VoteMessage is used for block voting in hybrid consensus
type VoteMessage struct {
	BaseMessage
	BlockHash   common.Hash   `json:"blockHash"`
	BlockHeight int           `json:"blockHeight"`
	Vote        bool          `json:"vote"`
	VoterAddr   types.Address `json:"voterAddr"`
	Signature   []byte        `json:"signature"`
}

// NewVoteMessage creates a new vote message
func NewVoteMessage(blockHash common.Hash, blockHeight int, vote bool, voterAddr types.Address, signature []byte) *VoteMessage {
	return &VoteMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeVote,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
		Vote:        vote,
		VoterAddr:   voterAddr,
		Signature:   signature,
	}
}

// PingMessage is used to check peer liveness
type PingMessage struct {
	BaseMessage
	Nonce uint64 `json:"nonce"`
}

// NewPingMessage creates a new ping message
func NewPingMessage(nonce uint64) *PingMessage {
	return &PingMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypePing,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Nonce: nonce,
	}
}

// PongMessage responds to a ping
type PongMessage struct {
	BaseMessage
	Nonce uint64 `json:"nonce"`
}

// NewPongMessage creates a new pong message
func NewPongMessage(nonce uint64) *PongMessage {
	return &PongMessage{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypePong,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Nonce: nonce,
	}
}
