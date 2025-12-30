package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

// Decoder decodes messages from wire format
type Decoder struct {
	validator *Validator
	// Buffer for reading partial messages
	buffer []byte
}

// NewDecoder creates a new message decoder
func NewDecoder() *Decoder {
	return &Decoder{
		validator: NewValidator(),
		buffer:    make([]byte, 0, 4096),
	}
}

// ReadMessage reads a length-prefixed message from the buffer
// Format: [4-byte big-endian length][message data]\n
// Returns the message, bytes consumed, and any error
func (d *Decoder) ReadMessage(data []byte) (Message, int, error) {
	// Append new data to buffer
	d.buffer = append(d.buffer, data...)

	// Need at least 4 bytes for length
	if len(d.buffer) < 4 {
		return nil, 0, nil // Need more data
	}

	// Read length prefix (big-endian)
	length := binary.BigEndian.Uint32(d.buffer[0:4])

	// Validate length
	if length > MaxMessageSize {
		// Clear buffer and return error
		d.buffer = d.buffer[:0]
		return nil, 0, fmt.Errorf("message length %d exceeds maximum %d", length, MaxMessageSize)
	}

	// Need 4 + length bytes total
	totalNeeded := 4 + int(length)
	if len(d.buffer) < totalNeeded {
		return nil, 0, nil // Need more data
	}

	// Extract message data (skip length prefix)
	messageData := d.buffer[4:totalNeeded]

	// Decode the message
	msg, err := d.Decode(string(messageData))
	if err != nil {
		// Clear buffer on error
		d.buffer = d.buffer[:0]
		return nil, 0, fmt.Errorf("failed to decode message: %w", err)
	}

	// Remove consumed bytes from buffer
	d.buffer = d.buffer[totalNeeded:]

	return msg, totalNeeded, nil
}

// Decode decodes a message from wire format (without framing)
// This is used internally after reading the length prefix
func (d *Decoder) Decode(data string) (Message, error) {
	// Sanitize input
	data = d.validator.SanitizeMessage(data)
	data = strings.TrimSpace(data)

	if data == "" {
		return nil, fmt.Errorf("empty message")
	}

	// Parse message type
	msgType := d.parseMessageType(data)

	switch msgType {
	case MsgTypeReadyRequest:
		return d.decodeReadyRequest(data)
	case MsgTypeREQ:
		return d.decodeREQ(data)
	case MsgTypeNodeOk:
		return d.decodeNodeOk(data)
	case MsgTypeWhoIs:
		return d.decodeWhoIs(data)
	case MsgTypeWhoIsResponse:
		return d.decodeWhoIsResponse(data)
	case MsgTypeConsensusStatus:
		return d.decodeConsensusStatus(data)
	case MsgTypeNodes:
		return d.decodeNodes(data)
	case MsgTypeNodesCount:
		return d.decodeNodesCount(data)
	case MsgTypeBlock:
		return d.decodeBlock(data)
	case MsgTypePing:
		return &PingMessage{}, nil
	case MsgTypeKeepAlive:
		return &KeepAliveMessage{}, nil
	case MsgTypeBroadcastNonce:
		return d.decodeBroadcastNonce(data)
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}

func (d *Decoder) parseMessageType(data string) MessageType {
	// Find the first pipe or newline
	idx := strings.IndexAny(data, "|\n")
	if idx > 0 {
		return MessageType(strings.TrimSpace(data[:idx]))
	}
	return MessageType(strings.TrimSpace(data))
}

func (d *Decoder) splitMessage(msg, delimiter string) []string {
	var parts []string
	current := ""
	for _, char := range msg {
		if string(char) == delimiter {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else if char != '\n' && char != '\r' {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func (d *Decoder) trimResponse(s string) string {
	result := ""
	for _, char := range s {
		if char != ' ' && char != '\n' && char != '\r' && char != '\t' {
			result += string(char)
		}
	}
	return result
}

func (d *Decoder) decodeReadyRequest(data string) (*ReadyRequestMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid READY_REQUEST format")
	}

	addrStr := d.trimResponse(parts[1])
	networkAddr := d.trimResponse(parts[2])

	addr, err := d.validator.ValidateHexAddress(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	if err := d.validator.ValidateNetworkAddress(networkAddr); err != nil {
		return nil, fmt.Errorf("invalid network address: %w", err)
	}

	return &ReadyRequestMessage{
		Address:     *addr,
		NetworkAddr: networkAddr,
	}, nil
}

func (d *Decoder) decodeREQ(data string) (*REQMessage, error) {
	// Parse pipe-delimited format: REQ|address|networkAddr|node1Addr#node1Network,node2Addr#node2Network|nonce
	parts := d.splitMessage(data, "|")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid REQ format: expected at least 4 parts, got %d", len(parts))
	}

	req := &REQMessage{
		Nodes: make([]NodeInfo, 0),
	}

	// Part 0: Message type (REQ) - skip
	// Part 1: Address
	addrStr := d.trimResponse(parts[1])
	addr, err := d.validator.ValidateHexAddress(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	req.Address = *addr

	// Part 2: Network address
	req.NetworkAddr = d.trimResponse(parts[2])

	// Part 3: Nodes (optional, comma-separated address#network pairs)
	if len(parts) > 3 && parts[3] != "" {
		nodesStr := parts[3]
		nodePairs := strings.Split(nodesStr, ",")
		for _, nodePair := range nodePairs {
			nodePair = strings.TrimSpace(nodePair)
			if nodePair == "" {
				continue
			}
			nodeParts := strings.Split(nodePair, "#")
			if len(nodeParts) == 2 {
				nodeAddrStr := strings.TrimSpace(nodeParts[0])
				networkAddr := strings.TrimSpace(nodeParts[1])
				
				nodeAddr, err := d.validator.ValidateHexAddress(nodeAddrStr)
				if err != nil {
					continue // skip invalid addresses
				}
				
				req.Nodes = append(req.Nodes, NodeInfo{
					Address:     *nodeAddr,
					NetworkAddr: networkAddr,
				})
			}
		}
	}

	// Part 4: Nonce
	if len(parts) > 4 {
		nonceStr := d.trimResponse(parts[4])
		var nonce uint64
		if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
			return nil, fmt.Errorf("invalid nonce: %w", err)
		}
		req.Nonce = nonce
	}

	return req, nil
}

func (d *Decoder) decodeNodeOk(data string) (*NodeOkMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid NODE_OK format")
	}

	countStr := d.trimResponse(parts[1])
	nonceStr := d.trimResponse(parts[2])

	var count int
	var nonce uint64

	if _, err := fmt.Sscanf(countStr, "%d", &count); err != nil {
		return nil, fmt.Errorf("failed to parse count: %w", err)
	}

	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		return nil, fmt.Errorf("failed to parse nonce: %w", err)
	}

	return &NodeOkMessage{
		Count: count,
		Nonce: nonce,
	}, nil
}

func (d *Decoder) decodeWhoIs(data string) (*WhoIsMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid WHO_IS format")
	}

	addrStr := d.trimResponse(parts[1])
	addr, err := d.validator.ValidateHexAddress(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	return &WhoIsMessage{
		NodeAddress: *addr,
	}, nil
}

func (d *Decoder) decodeWhoIsResponse(data string) (*WhoIsResponseMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid WHO_IS_RESPONSE format")
	}

	addrStr := d.trimResponse(parts[1])
	networkAddr := d.trimResponse(parts[2])

	addr, err := d.validator.ValidateHexAddress(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	if err := d.validator.ValidateNetworkAddress(networkAddr); err != nil {
		return nil, fmt.Errorf("invalid network address: %w", err)
	}

	return &WhoIsResponseMessage{
		NodeAddress: *addr,
		NetworkAddr: networkAddr,
	}, nil
}

func (d *Decoder) decodeConsensusStatus(data string) (*ConsensusStatusMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid CONSENSUS_STATUS format")
	}

	statusStr := d.trimResponse(parts[1])
	var status int
	if _, err := fmt.Sscanf(statusStr, "%d", &status); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	votersStr := d.trimResponse(parts[2])
	var voters []types.Address
	if votersStr != "" {
		voterAddrs := strings.Split(votersStr, ",")
		for _, addrStr := range voterAddrs {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr != "" {
				addr := types.HexToAddress(addrStr)
				if addr != (types.Address{}) {
					voters = append(voters, addr)
				}
			}
		}
	}

	nodesStr := d.trimResponse(parts[3])
	var nodes []types.Address
	if nodesStr != "" {
		nodeAddrs := strings.Split(nodesStr, ",")
		for _, addrStr := range nodeAddrs {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr != "" {
				addr := types.HexToAddress(addrStr)
				if addr != (types.Address{}) {
					nodes = append(nodes, addr)
				}
			}
		}
	}

	nonceStr := d.trimResponse(parts[4])
	var nonce uint64
	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		return nil, fmt.Errorf("failed to parse nonce: %w", err)
	}

	return &ConsensusStatusMessage{
		Status: status,
		Voters: voters,
		Nodes:  nodes,
		Nonce:  nonce,
	}, nil
}

func (d *Decoder) decodeNodes(data string) (*NodesMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid NODES format")
	}

	nodeListStr := parts[1]
	nodes := d.splitMessage(nodeListStr, ",")

	var nodeInfos []NodeInfo
	for _, nodeInfo := range nodes {
		nodeParts := d.splitMessage(nodeInfo, "#")
		if len(nodeParts) == 2 {
			nodeAddrStr := nodeParts[0]
			networkAddr := nodeParts[1]

			addr, err := d.validator.ValidateHexAddress(nodeAddrStr)
			if err != nil {
				continue // skip invalid addresses
			}

			nodeInfos = append(nodeInfos, NodeInfo{
				Address:     *addr,
				NetworkAddr: networkAddr,
			})
		}
	}

	return &NodesMessage{
		Nodes: nodeInfos,
	}, nil
}

func (d *Decoder) decodeNodesCount(data string) (*NodesCountMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid NODES_COUNT format")
	}

	countStr := d.trimResponse(parts[1])
	var count int
	if _, err := fmt.Sscanf(countStr, "%d", &count); err != nil {
		return nil, fmt.Errorf("failed to parse count: %w", err)
	}

	return &NodesCountMessage{
		Count: count,
	}, nil
}

func (d *Decoder) decodeBlock(data string) (*BlockMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid BLOCK format: missing JSON data")
	}

	jsonData := parts[1]
	jsonData = strings.TrimSpace(jsonData)

	var b block.Block
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block JSON: %w", err)
	}

	if b.Head == nil {
		return nil, fmt.Errorf("block header is nil")
	}

	return &BlockMessage{
		Block: &b,
	}, nil
}

func (d *Decoder) decodeBroadcastNonce(data string) (*BroadcastNonceMessage, error) {
	parts := d.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid BROADCAST_NONCE format")
	}

	nonceStr := d.trimResponse(parts[1])
	var nonce uint64
	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		return nil, fmt.Errorf("failed to parse nonce: %w", err)
	}

	var nodeList []NodeInfo
	if len(parts) >= 3 && parts[2] != "" {
		nodeListStr := parts[2]
		nodes := d.splitMessage(nodeListStr, ",")
		for _, nodeInfo := range nodes {
			nodeParts := d.splitMessage(nodeInfo, "#")
			if len(nodeParts) == 2 {
				nodeAddrStr := nodeParts[0]
				networkAddr := nodeParts[1]

				addr, err := d.validator.ValidateHexAddress(nodeAddrStr)
				if err != nil {
					continue
				}

				nodeList = append(nodeList, NodeInfo{
					Address:     *addr,
					NetworkAddr: networkAddr,
				})
			}
		}
	}

	return &BroadcastNonceMessage{
		Nonce:    nonce,
		NodeList: nodeList,
	}, nil
}

// MessageDecoder interface for decoding messages
type MessageDecoder interface {
	Decode(data string) (Message, error)
}

