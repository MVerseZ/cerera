package protocol

import (
	"github.com/libp2p/go-libp2p/core/protocol"
)

const (
	// ProtocolPrefix is the prefix for all Cerera protocols
	ProtocolPrefix = "/cerera"

	// ProtocolVersion is the current protocol version
	ProtocolVersion = "1.0.0"
)

// Protocol IDs for different message types
var (
	// StatusProtocolID is used for exchanging node status information
	StatusProtocolID = protocol.ID(ProtocolPrefix + "/status/" + ProtocolVersion)

	// SyncProtocolID is used for block synchronization
	SyncProtocolID = protocol.ID(ProtocolPrefix + "/sync/" + ProtocolVersion)

	// BlockProtocolID is used for block requests and responses
	BlockProtocolID = protocol.ID(ProtocolPrefix + "/block/" + ProtocolVersion)

	// TxProtocolID is used for transaction propagation
	TxProtocolID = protocol.ID(ProtocolPrefix + "/tx/" + ProtocolVersion)

	// ConsensusProtocolID is used for consensus messages
	ConsensusProtocolID = protocol.ID(ProtocolPrefix + "/consensus/" + ProtocolVersion)

	// PingProtocolID is used for ping/pong messages
	PingProtocolID = protocol.ID(ProtocolPrefix + "/ping/" + ProtocolVersion)
)

// AllProtocols returns all supported protocol IDs
func AllProtocols() []protocol.ID {
	return []protocol.ID{
		StatusProtocolID,
		SyncProtocolID,
		BlockProtocolID,
		TxProtocolID,
		ConsensusProtocolID,
		PingProtocolID,
	}
}

// MessageType represents the type of a protocol message
type MessageType uint8

const (
	// Status messages
	MsgTypeStatusRequest MessageType = iota
	MsgTypeStatusResponse

	// Block messages
	MsgTypeGetBlocks
	MsgTypeBlocks
	MsgTypeGetBlockHeaders
	MsgTypeBlockHeaders
	MsgTypeNewBlock
	MsgTypeNewBlockHash

	// Transaction messages
	MsgTypeNewTx
	MsgTypeGetTxs
	MsgTypeTxs

	// Sync messages
	MsgTypeSyncRequest
	MsgTypeSyncResponse
	MsgTypeSyncComplete

	// Consensus messages
	MsgTypePrePrepare
	MsgTypePrepare
	MsgTypeCommit
	MsgTypeViewChange
	MsgTypeNewView
	MsgTypeVote
	MsgTypeVoteResponse

	// Ping/Pong messages
	MsgTypePing
	MsgTypePong
)

// String returns the string representation of the message type
func (mt MessageType) String() string {
	switch mt {
	case MsgTypeStatusRequest:
		return "StatusRequest"
	case MsgTypeStatusResponse:
		return "StatusResponse"
	case MsgTypeGetBlocks:
		return "GetBlocks"
	case MsgTypeBlocks:
		return "Blocks"
	case MsgTypeGetBlockHeaders:
		return "GetBlockHeaders"
	case MsgTypeBlockHeaders:
		return "BlockHeaders"
	case MsgTypeNewBlock:
		return "NewBlock"
	case MsgTypeNewBlockHash:
		return "NewBlockHash"
	case MsgTypeNewTx:
		return "NewTx"
	case MsgTypeGetTxs:
		return "GetTxs"
	case MsgTypeTxs:
		return "Txs"
	case MsgTypeSyncRequest:
		return "SyncRequest"
	case MsgTypeSyncResponse:
		return "SyncResponse"
	case MsgTypeSyncComplete:
		return "SyncComplete"
	case MsgTypePrePrepare:
		return "PrePrepare"
	case MsgTypePrepare:
		return "Prepare"
	case MsgTypeCommit:
		return "Commit"
	case MsgTypeViewChange:
		return "ViewChange"
	case MsgTypeNewView:
		return "NewView"
	case MsgTypeVote:
		return "Vote"
	case MsgTypeVoteResponse:
		return "VoteResponse"
	case MsgTypePing:
		return "Ping"
	case MsgTypePong:
		return "Pong"
	default:
		return "Unknown"
	}
}
