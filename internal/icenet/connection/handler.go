package connection

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/protocol"
)

// Handler handles individual connections
type Handler struct {
	decoder     *protocol.Decoder
	encoder     *protocol.Encoder
	validator   *protocol.Validator
	config      *ConnectionConfig
	messageChan chan *MessageEvent
	rateLimiter *RateLimiter
}

// MessageEvent represents a message received from a connection
type MessageEvent struct {
	Connection *Connection
	Message    protocol.Message
	Error      error
}

// NewHandler creates a new connection handler
func NewHandler(config *ConnectionConfig) *Handler {
	if config == nil {
		config = DefaultConnectionConfig()
	}
	return &Handler{
		decoder:     protocol.NewDecoder(),
		encoder:     protocol.NewEncoder(),
		validator:   protocol.NewValidator(),
		config:      config,
		messageChan: make(chan *MessageEvent, 100),
		rateLimiter: NewRateLimiter(config.RateLimitMessagesPerSecond, config.RateLimitBurstSize),
	}
}

// HandleConnection handles a connection in a separate goroutine
func (h *Handler) HandleConnection(ctx context.Context, conn *Connection, pool *Pool) {
	if conn == nil || conn.Conn == nil {
		return
	}

	// Ensure connection is cleaned up on exit
	defer func() {
		if conn.Conn != nil {
			conn.Conn.Close()
		}
		// Remove from rate limiter
		if h.rateLimiter != nil {
			h.rateLimiter.Remove(conn.ID)
		}
		// Remove from pool if provided
		if pool != nil {
			pool.Remove(conn.ID)
		}
	}()

	// Set up context cancellation
	ctxDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(ctxDone)
		if conn.Conn != nil {
			conn.Conn.Close()
		}
	}()

	// Read loop with message framing
	buffer := make([]byte, h.config.ReadBufferSize)
	for {
		// Check context cancellation
		select {
		case <-ctxDone:
			return
		default:
		}

		// Set read deadline
		conn.Conn.SetReadDeadline(time.Now().Add(h.config.ReadTimeout))

		n, err := conn.Conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				// Connection closed by peer
				return
			}
			// Check if error is due to context cancellation
			select {
			case <-ctxDone:
				return
			default:
			}
			// Other read error
			return
		}

		if n > 0 {
			// Process all complete messages in the buffer
			readData := buffer[:n]
			for len(readData) > 0 {
				// Record message received metrics
				startTime := time.Now()

				// Read message with framing
				msg, consumed, err := h.decoder.ReadMessage(readData)
				if err != nil {
					// Record error metric
					metrics.Get().RecordMessageError(protocol.MsgTypeReadyRequest, "decode_error")
					// Send error event
					select {
					case h.messageChan <- &MessageEvent{
						Connection: conn,
						Error:      fmt.Errorf("failed to decode message: %w", err),
					}:
						utilization := float64(len(h.messageChan)) / float64(cap(h.messageChan))
						metrics.Get().UpdateChannelBufferUtilization(utilization)
					default:
						metrics.Get().RecordMessageDropped("unknown", "channel_full")
					}
					// On decode error, close connection (defer will clean up)
					return
				}

				// If no message decoded yet, need more data
				if msg == nil {
					break
				}

				// Check rate limit
				if !h.rateLimiter.Allow(conn.ID) {
					metrics.Get().RecordMessageDropped(string(msg.Type()), "rate_limit_exceeded")
					// Skip this message but continue processing
					readData = readData[consumed:]
					continue
				}

				// Record message received
				metrics.Get().RecordMessageReceived(msg.Type(), consumed)

				// Validate message
				if err := h.validator.ValidateMessage(msg); err != nil {
					metrics.Get().RecordMessageError(msg.Type(), "validation_error")
					select {
					case h.messageChan <- &MessageEvent{
						Connection: conn,
						Error:      fmt.Errorf("message validation failed: %w", err),
					}:
						utilization := float64(len(h.messageChan)) / float64(cap(h.messageChan))
						metrics.Get().UpdateChannelBufferUtilization(utilization)
					default:
						metrics.Get().RecordMessageDropped(string(msg.Type()), "channel_full")
					}
					// Continue to next message
					readData = readData[consumed:]
					continue
				}

				// Record processing time
				processingTime := time.Since(startTime)
				metrics.Get().RecordMessageProcessingTime(msg.Type(), processingTime)

				// Update last seen
				conn.LastSeen = time.Now()

				// Send message event with backpressure handling
				select {
				case h.messageChan <- &MessageEvent{
					Connection: conn,
					Message:    msg,
				}:
					// Update channel buffer utilization
					utilization := float64(len(h.messageChan)) / float64(cap(h.messageChan))
					metrics.Get().UpdateChannelBufferUtilization(utilization)
				default:
					// Channel full, record dropped message
					metrics.Get().RecordMessageDropped(string(msg.Type()), "channel_full")
					// Optionally log or handle the dropped message
				}

				// Move to next message
				readData = readData[consumed:]
			}
		}
	}
}

// MessageChannel returns the channel for receiving messages
func (h *Handler) MessageChannel() <-chan *MessageEvent {
	return h.messageChan
}

// WriteMessage writes a message to a connection
func (h *Handler) WriteMessage(conn *Connection, msg protocol.Message) error {
	if conn == nil || conn.Conn == nil {
		return fmt.Errorf("connection is nil or closed")
	}

	data, err := h.encoder.Encode(msg)
	if err != nil {
		metrics.Get().RecordMessageError(msg.Type(), "encode_error")
		return fmt.Errorf("failed to encode message: %w", err)
	}

	conn.Conn.SetWriteDeadline(time.Now().Add(h.config.WriteTimeout))
	_, err = conn.Conn.Write(data)
	if err != nil {
		metrics.Get().RecordConnectionError("write_error")
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Record message sent metrics
	metrics.Get().RecordMessageSent(msg.Type(), len(data))
	metrics.Get().RecordNetworkThroughput("out", int64(len(data)))

	conn.LastSeen = time.Now()
	return nil
}

// WriteData writes raw data to a connection
func (h *Handler) WriteData(conn *Connection, data []byte) error {
	if conn == nil || conn.Conn == nil {
		return fmt.Errorf("connection is nil or closed")
	}

	conn.Conn.SetWriteDeadline(time.Now().Add(h.config.WriteTimeout))
	_, err := conn.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	conn.LastSeen = time.Now()
	return nil
}
