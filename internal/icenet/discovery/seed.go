package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/protocol"
)

// SeedDiscovery manages peer discovery from seed nodes
type SeedDiscovery struct {
	mu          sync.RWMutex
	ctx         context.Context
	seedNodes   []string
	connManager *connection.Manager
	encoder     *protocol.Encoder
	decoder     *protocol.Decoder
	connected   map[string]bool // отслеживание подключенных seed nodes
}

// NewSeedDiscovery creates a new seed nodes discovery
func NewSeedDiscovery(ctx context.Context, seedNodes []string, connManager *connection.Manager) *SeedDiscovery {
	return &SeedDiscovery{
		ctx:         ctx,
		seedNodes:   seedNodes,
		connManager: connManager,
		encoder:     protocol.NewEncoder(),
		decoder:     protocol.NewDecoder(),
		connected:   make(map[string]bool),
	}
}

// ConnectToSeedNodes connects to seed nodes and discovers peers
func (sd *SeedDiscovery) ConnectToSeedNodes() error {
	if len(sd.seedNodes) == 0 {
		return nil // нет seed nodes для подключения
	}

	var errors []error
	connectedCount := 0

	for _, seedAddr := range sd.seedNodes {
		seedAddr = strings.TrimSpace(seedAddr)
		if seedAddr == "" {
			continue
		}

		// Проверяем, не подключены ли мы уже к этому seed node
		sd.mu.RLock()
		if sd.connected[seedAddr] {
			sd.mu.RUnlock()
			continue
		}
		sd.mu.RUnlock()

		// Подключаемся к seed node
		conn, err := sd.connManager.Connect(seedAddr, types.Address{})
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to connect to seed node %s: %w", seedAddr, err))
			continue
		}

		// Отмечаем как подключенный
		sd.mu.Lock()
		sd.connected[seedAddr] = true
		sd.mu.Unlock()

		connectedCount++

		// Отправляем READY_REQUEST для получения списка пиров
		go sd.requestPeersFromSeed(conn, seedAddr)
	}

	if len(errors) > 0 && connectedCount == 0 {
		return fmt.Errorf("failed to connect to any seed nodes: %d errors", len(errors))
	}

	return nil
}

// requestPeersFromSeed requests peer list from a seed node
func (sd *SeedDiscovery) requestPeersFromSeed(conn *connection.Connection, seedAddr string) {
	defer func() {
		sd.mu.Lock()
		delete(sd.connected, seedAddr)
		sd.mu.Unlock()
	}()

	// Отправляем READY_REQUEST
	readyMsg := &protocol.ReadyRequestMessage{
		Address:     types.Address{}, // будет заполнено позже
		NetworkAddr: "",              // будет заполнено позже
	}

	handler := sd.connManager.GetHandler()
	if err := handler.WriteMessage(conn, readyMsg); err != nil {
		return
	}

	// Читаем ответы (REQ, NODES) через обработчик сообщений
	// Это будет обработано через processConnectionMessages в ice.go
}

// GetSeedNodes returns the list of seed nodes
func (sd *SeedDiscovery) GetSeedNodes() []string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return sd.seedNodes
}

// AddSeedNode adds a seed node to the list
func (sd *SeedDiscovery) AddSeedNode(seedAddr string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Проверяем, нет ли уже этого seed node в списке
	for _, existing := range sd.seedNodes {
		if existing == seedAddr {
			return
		}
	}

	sd.seedNodes = append(sd.seedNodes, seedAddr)
}
