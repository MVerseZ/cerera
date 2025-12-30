package connection

import (
	"net"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// ConnectionState represents the state of a connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosing
)

// ConnectionType represents the type of connection
type ConnectionType int

const (
	TypeIncoming ConnectionType = iota
	TypeOutgoing
	TypeBootstrap
)

// String returns the string representation of ConnectionType
func (ct ConnectionType) String() string {
	switch ct {
	case TypeIncoming:
		return "incoming"
	case TypeOutgoing:
		return "outgoing"
	case TypeBootstrap:
		return "bootstrap"
	default:
		return "unknown"
	}
}

// Connection represents a network connection with metadata
type Connection struct {
	ID          string
	Conn        net.Conn
	RemoteAddr  types.Address
	NetworkAddr string
	State       ConnectionState
	Type        ConnectionType
	CreatedAt   time.Time
	LastSeen    time.Time
	ReadTimeout time.Duration
	WriteTimeout time.Duration
}

// IsConnected returns true if the connection is in connected state
func (c *Connection) IsConnected() bool {
	return c.State == StateConnected && c.Conn != nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

// ConnectionConfig holds configuration for connections
type ConnectionConfig struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	ConnectionTimeout time.Duration
	MaxConnections    int
	KeepAliveInterval time.Duration
	PingInterval      time.Duration
	PingTimeout       time.Duration
	ReadBufferSize    int
	RateLimitMessagesPerSecond int
	RateLimitBurstSize int
}

// DefaultConnectionConfig returns default connection configuration
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Second,
		ConnectionTimeout: 10 * time.Second,
		MaxConnections:    100,
		KeepAliveInterval: 30 * time.Second,
		PingInterval:      10 * time.Second,
		PingTimeout:       1 * time.Second,
		ReadBufferSize:    4096,
		RateLimitMessagesPerSecond: 100,
		RateLimitBurstSize:         200,
	}
}

