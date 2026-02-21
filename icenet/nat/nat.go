package nat

import (
	"github.com/cerera/config"
	"github.com/libp2p/go-libp2p"
)

// GetNATOptions returns libp2p options for NAT traversal
func GetNATOptions(cfg *config.Config) []libp2p.Option {
	opts := []libp2p.Option{}

	// Enable NAT port mapping (UPnP/NAT-PMP)
	opts = append(opts, libp2p.NATPortMap())

	// Enable AutoNAT service for NAT detection
	opts = append(opts, libp2p.EnableNATService())

	// Enable hole punching for direct connections through NAT
	opts = append(opts, libp2p.EnableHolePunching())

	// Enable relay service - allows this node to be a relay for others
	opts = append(opts, libp2p.EnableRelayService())

	// Enable relay client - allows this node to connect through relays
	opts = append(opts, libp2p.EnableRelay())

	// Configure relay nodes if specified
	if len(cfg.NetCfg.RelayNodes) > 0 {
		// Static relay configuration will be handled in discovery
		// when we connect to the relay nodes
	}

	return opts
}

// NATConfig holds NAT traversal configuration
type NATConfig struct {
	// EnableUPnP enables UPnP port mapping
	EnableUPnP bool
	// EnableHolePunching enables NAT hole punching
	EnableHolePunching bool
	// EnableRelay enables circuit relay
	EnableRelay bool
	// RelayNodes is a list of relay node multiaddresses
	RelayNodes []string
	// ForceRelay forces the use of relay even if direct connection is possible
	ForceRelay bool
}

// DefaultNATConfig returns the default NAT configuration
func DefaultNATConfig() *NATConfig {
	return &NATConfig{
		EnableUPnP:         true,
		EnableHolePunching: true,
		EnableRelay:        true,
		RelayNodes:         []string{},
		ForceRelay:         false,
	}
}

// GetNATConfigFromConfig extracts NAT configuration from the main config
func GetNATConfigFromConfig(cfg *config.Config) *NATConfig {
	natCfg := DefaultNATConfig()
	
	if len(cfg.NetCfg.RelayNodes) > 0 {
		natCfg.RelayNodes = cfg.NetCfg.RelayNodes
	}
	
	return natCfg
}
