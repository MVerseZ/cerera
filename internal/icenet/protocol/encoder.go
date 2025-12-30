package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Encoder encodes messages to wire format
type Encoder struct{}

// NewEncoder creates a new message encoder
func NewEncoder() *Encoder {
	return &Encoder{}
}

// Encode encodes a message to its wire format with length prefix
// Format: [4-byte big-endian length][message data]\n
func (e *Encoder) Encode(msg Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	var data []byte
	var err error

	switch m := msg.(type) {
	case *ReadyRequestMessage:
		data = e.encodeReadyRequest(m)
	case *REQMessage:
		data = e.encodeREQ(m)
	case *NodeOkMessage:
		data = e.encodeNodeOk(m)
	case *WhoIsMessage:
		data = e.encodeWhoIs(m)
	case *WhoIsResponseMessage:
		data = e.encodeWhoIsResponse(m)
	case *ConsensusStatusMessage:
		data = e.encodeConsensusStatus(m)
	case *NodesMessage:
		data = e.encodeNodes(m)
	case *NodesCountMessage:
		data = e.encodeNodesCount(m)
	case *BlockMessage:
		data, err = e.encodeBlock(m)
		if err != nil {
			return nil, err
		}
	case *PingMessage:
		data = e.encodePing(m)
	case *KeepAliveMessage:
		data = e.encodeKeepAlive(m)
	case *BroadcastNonceMessage:
		data = e.encodeBroadcastNonce(m)
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}

	// Add newline if not present
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	// Prepend 4-byte big-endian length
	length := uint32(len(data))
	framed := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(framed[0:4], length)
	copy(framed[4:], data)

	return framed, nil
}

func (e *Encoder) encodeReadyRequest(msg *ReadyRequestMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeReadyRequest))
	buf.WriteByte('|')
	buf.WriteString(msg.Address.Hex())
	buf.WriteByte('|')
	buf.WriteString(msg.NetworkAddr)
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeREQ(msg *REQMessage) []byte {
	// Standardize REQ to pipe-delimited format: REQ|address|networkAddr|node1Addr#node1Network,node2Addr#node2Network|nonce
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeREQ))
	buf.WriteByte('|')
	buf.WriteString(msg.Address.Hex())
	buf.WriteByte('|')
	buf.WriteString(msg.NetworkAddr)

	// Encode nodes as comma-separated address#network pairs
	if len(msg.Nodes) > 0 {
		buf.WriteByte('|')
		for i, node := range msg.Nodes {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(node.Address.Hex())
			buf.WriteByte('#')
			buf.WriteString(node.NetworkAddr)
		}
	}

	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Nonce))

	return buf.Bytes()
}

func (e *Encoder) encodeNodeOk(msg *NodeOkMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeNodeOk))
	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Count))
	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Nonce))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeWhoIs(msg *WhoIsMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeWhoIs))
	buf.WriteByte('|')
	buf.WriteString(msg.NodeAddress.Hex())
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeWhoIsResponse(msg *WhoIsResponseMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeWhoIsResponse))
	buf.WriteByte('|')
	buf.WriteString(msg.NodeAddress.Hex())
	buf.WriteByte('|')
	buf.WriteString(msg.NetworkAddr)
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeConsensusStatus(msg *ConsensusStatusMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeConsensusStatus))
	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Status))
	buf.WriteByte('|')

	// Write voters
	for i, v := range msg.Voters {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(v.Hex())
	}

	buf.WriteByte('|')

	// Write nodes
	for i, n := range msg.Nodes {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(n.Hex())
	}

	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Nonce))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeNodes(msg *NodesMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeNodes))
	buf.WriteByte('|')

	for i, node := range msg.Nodes {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(node.Address.Hex())
		buf.WriteByte('#')
		buf.WriteString(node.NetworkAddr)
	}

	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeNodesCount(msg *NodesCountMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeNodesCount))
	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Count))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeBlock(msg *BlockMessage) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeBlock))
	buf.WriteByte('|')

	// Use json.Encoder for better performance
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(msg.Block); err != nil {
		return nil, fmt.Errorf("failed to marshal block: %w", err)
	}

	// json.Encoder adds newline, but we want to ensure it's there
	if buf.Bytes()[buf.Len()-1] != '\n' {
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func (e *Encoder) encodePing(msg *PingMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypePing))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeKeepAlive(msg *KeepAliveMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeKeepAlive))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (e *Encoder) encodeBroadcastNonce(msg *BroadcastNonceMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(MsgTypeBroadcastNonce))
	buf.WriteByte('|')
	buf.WriteString(fmt.Sprintf("%d", msg.Nonce))

	if len(msg.NodeList) > 0 {
		buf.WriteByte('|')
		for i, node := range msg.NodeList {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(node.Address.Hex())
			buf.WriteByte('#')
			buf.WriteString(node.NetworkAddr)
		}
	}

	buf.WriteByte('\n')
	return buf.Bytes()
}

// MessageEncoder interface for encoding messages
type MessageEncoder interface {
	Encode(msg Message) ([]byte, error)
}
