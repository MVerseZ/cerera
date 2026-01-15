package icenet

import (
	"context"
	"fmt"

	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/consensus"
	"github.com/cerera/internal/icenet/mesh"
	"github.com/cerera/internal/icenet/metrics"
	"github.com/cerera/internal/icenet/protocol"
)

// Coordinator coordinates all icenet components
type Coordinator struct {
	ctx         context.Context
	cfg         *config.Config
	address     types.Address
	networkAddr string
	isBootstrap bool
	meshEnabled bool

	// Components
	protocol       *protocol.Encoder
	decoder        *protocol.Decoder
	validator      *protocol.Validator
	connManager    *connection.Manager
	consensusCoord *consensus.Coordinator
	meshNetwork    *mesh.Network

	// State
	started bool
}

// NewCoordinator creates a new coordinator
func NewCoordinator(ctx context.Context, cfg *config.Config, address types.Address, networkAddr string, isBootstrap bool, meshEnabled bool) (*Coordinator, error) {
	// Initialize metrics
	metrics.Init()

	// Initialize protocol components
	encoder := protocol.NewEncoder()
	decoder := protocol.NewDecoder()
	validator := protocol.NewValidator()

	// Initialize connection manager
	connConfig := connection.DefaultConnectionConfig()
	connManager := connection.NewManager(ctx, connConfig)

	// Initialize consensus coordinator
	consensusManager := consensus.NewGigeaManager()
	consensusCoord := consensus.NewCoordinator(consensusManager, isBootstrap)

	coord := &Coordinator{
		ctx:            ctx,
		cfg:            cfg,
		address:        address,
		networkAddr:    networkAddr,
		isBootstrap:    isBootstrap,
		meshEnabled:    meshEnabled,
		protocol:       encoder,
		decoder:        decoder,
		validator:      validator,
		connManager:    connManager,
		consensusCoord: consensusCoord,
		started:        false,
	}

	// Initialize mesh network if enabled
	if meshEnabled {
		peerStore := mesh.NewPeerStore()
		seedNodes := []string{}
		if cfg != nil && len(cfg.NetCfg.SeedNodes) > 0 {
			seedNodes = cfg.NetCfg.SeedNodes
		}
		discovery := mesh.NewDiscovery(ctx, peerStore, seedNodes, connManager)
		meshConfig := mesh.DefaultNetworkConfig()
		coord.meshNetwork = mesh.NewNetwork(ctx, meshConfig, peerStore, discovery, connManager)
	}

	return coord, nil
}

// Start starts the coordinator
func (c *Coordinator) Start(port string) error {
	if c.started {
		return fmt.Errorf("coordinator already started")
	}

	// Start connection manager
	if err := c.connManager.Start(port); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	// Start consensus coordinator
	c.consensusCoord.Start()

	// Start mesh network if enabled
	if c.meshEnabled && c.meshNetwork != nil {
		if err := c.meshNetwork.Start(); err != nil {
			return fmt.Errorf("failed to start mesh network: %w", err)
		}
	}

	c.started = true
	return nil
}

// Stop stops the coordinator
func (c *Coordinator) Stop() error {
	if !c.started {
		return nil
	}

	// Stop mesh network if enabled
	if c.meshEnabled && c.meshNetwork != nil {
		if err := c.meshNetwork.Stop(); err != nil {
			return fmt.Errorf("failed to stop mesh network: %w", err)
		}
	}

	// Stop consensus coordinator
	c.consensusCoord.Stop()

	// Stop connection manager
	if err := c.connManager.Stop(); err != nil {
		return fmt.Errorf("failed to stop connection manager: %w", err)
	}

	c.started = false
	return nil
}

// GetConnectionManager returns the connection manager
func (c *Coordinator) GetConnectionManager() *connection.Manager {
	return c.connManager
}

// GetConsensusCoordinator returns the consensus coordinator
func (c *Coordinator) GetConsensusCoordinator() *consensus.Coordinator {
	return c.consensusCoord
}

// GetProtocolEncoder returns the protocol encoder
func (c *Coordinator) GetProtocolEncoder() *protocol.Encoder {
	return c.protocol
}

// GetProtocolDecoder returns the protocol decoder
func (c *Coordinator) GetProtocolDecoder() *protocol.Decoder {
	return c.decoder
}

// GetProtocolValidator returns the protocol validator
func (c *Coordinator) GetProtocolValidator() *protocol.Validator {
	return c.validator
}

// IsStarted returns whether the coordinator is started
func (c *Coordinator) IsStarted() bool {
	return c.started
}

// IsBootstrap returns whether this is a bootstrap node
func (c *Coordinator) IsBootstrap() bool {
	return c.isBootstrap
}

// IsMeshEnabled returns whether mesh network is enabled
func (c *Coordinator) IsMeshEnabled() bool {
	return c.meshEnabled
}

// GetMeshNetwork returns the mesh network (if enabled)
func (c *Coordinator) GetMeshNetwork() *mesh.Network {
	return c.meshNetwork
}
