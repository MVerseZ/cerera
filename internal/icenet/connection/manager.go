package connection

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/metrics"
)

// Manager manages network connections
type Manager struct {
	mu          sync.RWMutex
	pool        *Pool
	handler     *Handler
	listener    net.Listener
	config      *ConnectionConfig
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
}

// NewManager creates a new connection manager
func NewManager(ctx context.Context, config *ConnectionConfig) *Manager {
	if config == nil {
		config = DefaultConnectionConfig()
	}

	managerCtx, cancel := context.WithCancel(ctx)

	return &Manager{
		pool:    NewPool(config.MaxConnections, config),
		handler: NewHandler(config),
		config:  config,
		ctx:     managerCtx,
		cancel:  cancel,
	}
}

// Start starts the connection manager
func (m *Manager) Start(port string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("connection manager already started")
	}

	addr := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener on %s: %w", addr, err)
	}

	m.listener = listener
	m.started = true

	// Start accepting connections
	go m.acceptConnections()

	return nil
}

// Stop stops the connection manager
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	m.started = false
	m.cancel()

	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
	}

	m.pool.Clear()
	metrics.Get().UpdateActiveConnections(0)
	return nil
}

// acceptConnections accepts incoming connections
func (m *Manager) acceptConnections() {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		m.mu.RLock()
		listener := m.listener
		started := m.started
		m.mu.RUnlock()

		if !started || listener == nil {
			return
		}

		// Set deadline for accept to allow periodic context checks
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			// Check if error is due to timeout (for context checking)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			m.mu.RLock()
			started = m.started
			m.mu.RUnlock()
			if !started {
				return
			}
			// Log error but continue
			continue
		}

		// Create connection object
		connection := &Connection{
			ID:           generateConnectionID(),
			Conn:         conn,
			NetworkAddr:  conn.RemoteAddr().String(),
			State:        StateConnected,
			Type:         TypeIncoming,
			CreatedAt:    time.Now(),
			LastSeen:     time.Now(),
			ReadTimeout:  m.config.ReadTimeout,
			WriteTimeout: m.config.WriteTimeout,
		}

		// Add to pool
		if err := m.pool.Add(connection); err != nil {
			conn.Close()
			metrics.Get().RecordConnectionError("pool_full")
			continue
		}

		// Record metrics
		metrics.Get().RecordConnectionEstablished(TypeIncoming.String(), "connected")
		metrics.Get().UpdateActiveConnections(m.pool.Size())

		// Handle connection in separate goroutine
		go m.handler.HandleConnection(m.ctx, connection, m.pool)
	}
}

// Connect connects to a remote address
func (m *Manager) Connect(addr string, nodeAddr types.Address) (*Connection, error) {
	dialer := &net.Dialer{
		Timeout: m.config.ConnectionTimeout,
	}

	conn, err := dialer.DialContext(m.ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	connection := &Connection{
		ID:           generateConnectionID(),
		Conn:         conn,
		RemoteAddr:   nodeAddr,
		NetworkAddr:  addr,
		State:        StateConnected,
		Type:         TypeOutgoing,
		CreatedAt:    time.Now(),
		LastSeen:     time.Now(),
		ReadTimeout:  m.config.ReadTimeout,
		WriteTimeout: m.config.WriteTimeout,
	}

	if err := m.pool.Add(connection); err != nil {
		conn.Close()
		metrics.Get().RecordConnectionError("pool_full")
		return nil, err
	}

	// Record metrics
	metrics.Get().RecordConnectionEstablished(TypeOutgoing.String(), "connected")
	metrics.Get().UpdateActiveConnections(m.pool.Size())

	// Handle connection in separate goroutine
	go m.handler.HandleConnection(m.ctx, connection, m.pool)

	return connection, nil
}

// ConnectWithRetry connects to a remote address with retry logic
func (m *Manager) ConnectWithRetry(addr string, nodeAddr types.Address, maxRetries int, baseDelay, maxDelay time.Duration) (*Connection, error) {
	var lastErr error
	retries := 0

	for {
		select {
		case <-m.ctx.Done():
			return nil, fmt.Errorf("connection cancelled")
		default:
		}

		conn, err := m.Connect(addr, nodeAddr)
		if err == nil {
			return conn, nil
		}

		lastErr = err
		retries++

		if retries >= maxRetries {
			return nil, fmt.Errorf("failed to connect after %d retries: %w", maxRetries, lastErr)
		}

		// Calculate exponential backoff
		backoffDelay := CalculateBackoff(retries-1, baseDelay, maxDelay)

		select {
		case <-m.ctx.Done():
			return nil, fmt.Errorf("connection cancelled")
		case <-time.After(backoffDelay):
			// Continue retry
		}
	}
}

// CalculateBackoff calculates exponential backoff delay
func CalculateBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// GetConnection retrieves a connection by ID
func (m *Manager) GetConnection(id string) (*Connection, bool) {
	return m.pool.Get(id)
}

// GetConnectionByAddress retrieves a connection by remote address
func (m *Manager) GetConnectionByAddress(addr types.Address) (*Connection, bool) {
	return m.pool.GetByAddress(addr)
}

// RemoveConnection removes a connection
func (m *Manager) RemoveConnection(id string) error {
	return m.pool.Remove(id)
}

// RemoveConnectionByAddress removes a connection by address
func (m *Manager) RemoveConnectionByAddress(addr types.Address) error {
	return m.pool.RemoveByAddress(addr)
}

// GetAllConnections returns all connections
func (m *Manager) GetAllConnections() []*Connection {
	return m.pool.GetAll()
}

// GetConnectedConnections returns all connected connections
func (m *Manager) GetConnectedConnections() []*Connection {
	return m.pool.GetConnected()
}

// GetHandler returns the connection handler
func (m *Manager) GetHandler() *Handler {
	return m.handler
}

// GetPool returns the connection pool
func (m *Manager) GetPool() *Pool {
	return m.pool
}

// IsStarted returns whether the manager is started
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// ConnectionManager interface for managing connections
type ConnectionManager interface {
	Start(port string) error
	Stop() error
	Connect(addr string, nodeAddr types.Address) (*Connection, error)
	ConnectWithRetry(addr string, nodeAddr types.Address, maxRetries int, baseDelay, maxDelay time.Duration) (*Connection, error)
	GetConnection(id string) (*Connection, bool)
	GetConnectionByAddress(addr types.Address) (*Connection, bool)
	RemoveConnection(id string) error
	RemoveConnectionByAddress(addr types.Address) error
	GetAllConnections() []*Connection
	GetConnectedConnections() []*Connection
	GetHandler() *Handler
	IsStarted() bool
}

