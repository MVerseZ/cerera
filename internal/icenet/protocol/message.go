package protocol

import (
	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// ProtocolVersion represents the protocol version
const ProtocolVersion = "1.0"

// MessageType represents a protocol message type
type MessageType string

const (
	// Message type constants
	MsgTypeReadyRequest    MessageType = "READY_REQUEST"
	MsgTypeREQ            MessageType = "REQ"
	MsgTypeNodeOk         MessageType = "NODE_OK"
	MsgTypeWhoIs          MessageType = "WHO_IS"
	MsgTypeWhoIsResponse  MessageType = "WHO_IS_RESPONSE"
	MsgTypeConsensusStatus MessageType = "CONSENSUS_STATUS"
	MsgTypeNodes          MessageType = "NODES"
	MsgTypeNodesCount     MessageType = "NODES_COUNT"
	MsgTypeBlock          MessageType = "BLOCK"
	MsgTypePing           MessageType = "PING"
	MsgTypeKeepAlive      MessageType = "KEEPALIVE"
	MsgTypeStartConsensus MessageType = "START_CONSENSUS"
	MsgTypeConsensusStart MessageType = "CONSENSUS_START"
	MsgTypeBeginConsensus MessageType = "BEGIN_CONSENSUS"
	MsgTypeConsensusBegin MessageType = "CONSENSUS_BEGIN"
	MsgTypeBroadcastNonce MessageType = "BROADCAST_NONCE"
)

// IsValid checks if the message type is valid
func (mt MessageType) IsValid() bool {
	validTypes := []MessageType{
		MsgTypeReadyRequest,
		MsgTypeREQ,
		MsgTypeNodeOk,
		MsgTypeWhoIs,
		MsgTypeWhoIsResponse,
		MsgTypeConsensusStatus,
		MsgTypeNodes,
		MsgTypeNodesCount,
		MsgTypeBlock,
		MsgTypePing,
		MsgTypeKeepAlive,
		MsgTypeStartConsensus,
		MsgTypeConsensusStart,
		MsgTypeBeginConsensus,
		MsgTypeConsensusBegin,
		MsgTypeBroadcastNonce,
	}
	for _, vt := range validTypes {
		if mt == vt {
			return true
		}
	}
	return false
}

// Message represents a protocol message
type Message interface {
	Type() MessageType
	Version() string
}

// ReadyRequestMessage represents a READY_REQUEST message
type ReadyRequestMessage struct {
	Address     types.Address
	NetworkAddr string
}

func (m *ReadyRequestMessage) Type() MessageType {
	return MsgTypeReadyRequest
}

func (m *ReadyRequestMessage) Version() string {
	return ProtocolVersion
}

// REQMessage represents a REQ message with structured data
type REQMessage struct {
	Address     types.Address
	NetworkAddr string
	Nodes       []NodeInfo
	Nonce       uint64
}

func (m *REQMessage) Type() MessageType {
	return MsgTypeREQ
}

func (m *REQMessage) Version() string {
	return ProtocolVersion
}

// NodeInfo represents information about a node
type NodeInfo struct {
	Address     types.Address
	NetworkAddr string
}

// NodeOkMessage represents a NODE_OK message
type NodeOkMessage struct {
	Count int
	Nonce uint64
}

func (m *NodeOkMessage) Type() MessageType {
	return MsgTypeNodeOk
}

func (m *NodeOkMessage) Version() string {
	return ProtocolVersion
}

// WhoIsMessage represents a WHO_IS message
type WhoIsMessage struct {
	NodeAddress types.Address
}

func (m *WhoIsMessage) Type() MessageType {
	return MsgTypeWhoIs
}

func (m *WhoIsMessage) Version() string {
	return ProtocolVersion
}

// WhoIsResponseMessage represents a WHO_IS_RESPONSE message
type WhoIsResponseMessage struct {
	NodeAddress types.Address
	NetworkAddr string
}

func (m *WhoIsResponseMessage) Type() MessageType {
	return MsgTypeWhoIsResponse
}

func (m *WhoIsResponseMessage) Version() string {
	return ProtocolVersion
}

// ConsensusStatusMessage represents a CONSENSUS_STATUS message
type ConsensusStatusMessage struct {
	Status      int
	Voters      []types.Address
	Nodes       []types.Address
	Nonce       uint64
}

func (m *ConsensusStatusMessage) Type() MessageType {
	return MsgTypeConsensusStatus
}

func (m *ConsensusStatusMessage) Version() string {
	return ProtocolVersion
}

// NodesMessage represents a NODES message
type NodesMessage struct {
	Nodes []NodeInfo
}

func (m *NodesMessage) Type() MessageType {
	return MsgTypeNodes
}

func (m *NodesMessage) Version() string {
	return ProtocolVersion
}

// NodesCountMessage represents a NODES_COUNT message
type NodesCountMessage struct {
	Count int
}

func (m *NodesCountMessage) Type() MessageType {
	return MsgTypeNodesCount
}

func (m *NodesCountMessage) Version() string {
	return ProtocolVersion
}

// BlockMessage represents a BLOCK message
type BlockMessage struct {
	Block *block.Block
}

func (m *BlockMessage) Type() MessageType {
	return MsgTypeBlock
}

func (m *BlockMessage) Version() string {
	return ProtocolVersion
}

// PingMessage represents a PING message
type PingMessage struct{}

func (m *PingMessage) Type() MessageType {
	return MsgTypePing
}

func (m *PingMessage) Version() string {
	return ProtocolVersion
}

// KeepAliveMessage represents a KEEPALIVE message
type KeepAliveMessage struct{}

func (m *KeepAliveMessage) Type() MessageType {
	return MsgTypeKeepAlive
}

func (m *KeepAliveMessage) Version() string {
	return ProtocolVersion
}

// BroadcastNonceMessage represents a BROADCAST_NONCE message
type BroadcastNonceMessage struct {
	Nonce    uint64
	NodeList []NodeInfo
}

func (m *BroadcastNonceMessage) Type() MessageType {
	return MsgTypeBroadcastNonce
}

func (m *BroadcastNonceMessage) Version() string {
	return ProtocolVersion
}

