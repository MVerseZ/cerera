package protocol

import "time"

const (
	MsgTypeSystemPeerList MessageType = 200
	MsgTypeSystemPeerListResponse MessageType = 201
)

// SystemPeerListRequest asks for the node's confirmed peer registry.
type SystemPeerListRequest struct {
	BaseMessage
}

func NewSystemPeerListRequest() *SystemPeerListRequest {
	return &SystemPeerListRequest{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeSystemPeerList,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
	}
}

// PeerRecordDTO is a wire-friendly peer registry entry.
type PeerRecordDTO struct {
	CereraAddress string   `json:"cereraAddress"`
	PeerID        string   `json:"peerId"`
	ListenAddrs   []string `json:"listenAddrs,omitempty"`
	Height        int      `json:"height"`
	IsReady       bool     `json:"isReady"`
}

// SystemPeerListResponse returns known Cerera↔libp2p mappings.
type SystemPeerListResponse struct {
	BaseMessage
	Peers []PeerRecordDTO `json:"peers"`
}

func NewSystemPeerListResponse(peers []PeerRecordDTO) *SystemPeerListResponse {
	return &SystemPeerListResponse{
		BaseMessage: BaseMessage{
			MsgType:   MsgTypeSystemPeerListResponse,
			Timestamp: time.Now().UnixNano(),
			Version:   ProtocolVersion,
		},
		Peers: peers,
	}
}
