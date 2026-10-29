package protocol

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// MaxMessageSize is the maximum size of a protocol message (16MB)
	MaxMessageSize = 16 * 1024 * 1024

	// MessageHeaderSize is the size of the message header (4 bytes for length + 1 byte for type)
	MessageHeaderSize = 5
)

// Encoder handles encoding and writing protocol messages
type Encoder struct {
	writer *bufio.Writer
}

// NewEncoder creates a new message encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		writer: bufio.NewWriter(w),
	}
}

// Encode encodes and writes a message to the underlying writer
func (e *Encoder) Encode(msg Message) error {
	// Serialize the message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Write message length (4 bytes, big endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)+1)) // +1 for message type
	if _, err := e.writer.Write(lenBuf); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message type (1 byte)
	if err := e.writer.WriteByte(byte(msg.Type())); err != nil {
		return fmt.Errorf("failed to write message type: %w", err)
	}

	// Write message data
	if _, err := e.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return e.writer.Flush()
}

// Decoder handles reading and decoding protocol messages
type Decoder struct {
	reader *bufio.Reader
}

// NewDecoder creates a new message decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		reader: bufio.NewReader(r),
	}
}

// Decode reads and decodes a message from the underlying reader
func (d *Decoder) Decode() (Message, error) {
	// Read message length (4 bytes)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(d.reader, lenBuf); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", msgLen, MaxMessageSize)
	}

	if msgLen < 1 {
		return nil, fmt.Errorf("message too small: %d bytes", msgLen)
	}

	// Read message type (1 byte)
	msgType, err := d.reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Read message data
	data := make([]byte, msgLen-1)
	if _, err := io.ReadFull(d.reader, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Decode based on message type
	return decodeMessage(MessageType(msgType), data)
}

// decodeMessage decodes a message based on its type
func decodeMessage(msgType MessageType, data []byte) (Message, error) {
	var msg Message

	switch msgType {
	case MsgTypeStatusRequest:
		msg = &StatusRequest{}
	case MsgTypeStatusResponse:
		msg = &StatusResponse{}
	case MsgTypeGetBlocks:
		msg = &GetBlocksRequest{}
	case MsgTypeBlocks:
		msg = &BlocksResponse{}
	case MsgTypeNewBlock:
		msg = &NewBlockMessage{}
	case MsgTypeNewBlockHash:
		msg = &NewBlockHashMessage{}
	case MsgTypeNewTx:
		msg = &NewTxMessage{}
	case MsgTypeGetTxs:
		msg = &GetTxsRequest{}
	case MsgTypeTxs:
		msg = &TxsResponse{}
	case MsgTypeSyncRequest:
		msg = &SyncRequest{}
	case MsgTypeSyncResponse:
		msg = &SyncResponse{}
	case MsgTypePrePrepare, MsgTypePrepare, MsgTypeCommit, MsgTypeViewChange, MsgTypeNewView:
		msg = &ConsensusMessage{}
	case MsgTypeVote:
		msg = &VoteMessage{}
	case MsgTypePing:
		msg = &PingMessage{}
	case MsgTypePong:
		msg = &PongMessage{}
	default:
		return nil, fmt.Errorf("unknown message type: %d", msgType)
	}

	if err := json.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, nil
}

// EncodeToBytes encodes a message to bytes
func EncodeToBytes(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Allocate buffer for header + data
	buf := make([]byte, MessageHeaderSize+len(data))

	// Write message length
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)+1))

	// Write message type
	buf[4] = byte(msg.Type())

	// Copy data
	copy(buf[5:], data)

	return buf, nil
}

// DecodeFromBytes decodes a message from bytes
func DecodeFromBytes(data []byte) (Message, error) {
	if len(data) < MessageHeaderSize {
		return nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	msgLen := binary.BigEndian.Uint32(data[:4])
	if int(msgLen) > len(data)-4 {
		return nil, fmt.Errorf("invalid message length: %d (data len: %d)", msgLen, len(data))
	}

	msgType := MessageType(data[4])
	msgData := data[5 : 4+msgLen]

	return decodeMessage(msgType, msgData)
}
