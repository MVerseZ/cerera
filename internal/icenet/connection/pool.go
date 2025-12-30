package connection

import (
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/metrics"
)

// Pool manages a pool of connections
type Pool struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	maxSize     int
	config      *ConnectionConfig
}

// NewPool creates a new connection pool
func NewPool(maxSize int, config *ConnectionConfig) *Pool {
	if config == nil {
		config = DefaultConnectionConfig()
	}
	return &Pool{
		connections: make(map[string]*Connection),
		maxSize:     maxSize,
		config:      config,
	}
}

// Add adds a connection to the pool
func (p *Pool) Add(conn *Connection) error {
	if conn == nil {
		return fmt.Errorf("connection cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.connections) >= p.maxSize {
		return fmt.Errorf("connection pool is full (max: %d)", p.maxSize)
	}

	if conn.ID == "" {
		conn.ID = generateConnectionID()
	}

	p.connections[conn.ID] = conn
	return nil
}

// Get retrieves a connection by ID
func (p *Pool) Get(id string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, ok := p.connections[id]
	return conn, ok
}

// GetByAddress retrieves a connection by remote address
func (p *Pool) GetByAddress(addr types.Address) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, conn := range p.connections {
		if conn.RemoteAddr == addr {
			return conn, true
		}
	}
	return nil, false
}

// Remove removes a connection from the pool
func (p *Pool) Remove(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, exists := p.connections[id]
	if !exists {
		return fmt.Errorf("connection not found: %s", id)
	}

	connType := conn.Type.String()
	if conn.Conn != nil {
		conn.Conn.Close()
	}

	delete(p.connections, id)
	
	// Record metrics
	metrics.Get().RecordConnectionClosed(connType)
	metrics.Get().UpdateActiveConnections(len(p.connections))
	
	return nil
}

// RemoveByAddress removes a connection by remote address
func (p *Pool) RemoveByAddress(addr types.Address) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, conn := range p.connections {
		if conn.RemoteAddr == addr {
			connType := conn.Type.String()
			if conn.Conn != nil {
				conn.Conn.Close()
			}
			delete(p.connections, id)
			
			// Record metrics
			metrics.Get().RecordConnectionClosed(connType)
			metrics.Get().UpdateActiveConnections(len(p.connections))
			
			return nil
		}
	}
	return fmt.Errorf("connection not found for address: %s", addr.Hex())
}

// GetAll returns all connections
func (p *Pool) GetAll() []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	return connections
}

// GetConnected returns all connected connections
func (p *Pool) GetConnected() []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	connections := make([]*Connection, 0)
	for _, conn := range p.connections {
		if conn.IsConnected() {
			connections = append(connections, conn)
		}
	}
	return connections
}

// Size returns the current pool size
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// Clear removes all connections from the pool
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		if conn.Conn != nil {
			conn.Conn.Close()
		}
	}
	p.connections = make(map[string]*Connection)
}

// Cleanup removes disconnected connections
func (p *Pool) Cleanup() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	removed := 0
	for id, conn := range p.connections {
		if !conn.IsConnected() {
			if conn.Conn != nil {
				conn.Conn.Close()
			}
			delete(p.connections, id)
			removed++
		}
	}
	return removed
}

func generateConnectionID() string {
	return fmt.Sprintf("conn_%d", time.Now().UnixNano())
}

