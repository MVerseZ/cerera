package icenet

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/cerera/core/types"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/icenet/nat"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

// HostConfig contains configuration for creating a libp2p host
type HostConfig struct {
	Port           string
	PrivateKey     *ecdsa.PrivateKey
	BootstrapNodes []string
	RelayNodes     []string
	EnableRelay    bool
	EnableNAT      bool
	MaxPeers       int
}

type CereraHost interface {
	host.Host
}

// NewHost creates and configures a new libp2p host
func NewHost(ctx context.Context, cfg *config.Config, port string) (CereraHost, error) {
	iceLogger().Infow("Creating libp2p host", "port", port)

	// Convert ECDSA private key to libp2p format
	privKey, err := convertECDSAToLibp2p(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert private key: %w", err)
	}

	// Build listen addresses
	listenAddrs := []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", port),
		fmt.Sprintf("/ip6/::/tcp/%s", port),
	}

	// Build libp2p options
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.Identity(privKey),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.DefaultMuxers,
		libp2p.DefaultPeerstore,
	}

	// Add NAT traversal options
	natOpts := nat.GetNATOptions(cfg)
	opts = append(opts, natOpts...)

	// Create the host
	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Log host information
	hostInfo := peer.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}

	iceLogger().Infow("Host created successfully",
		"peerID", h.ID().String(),
		"addresses", hostInfo.Addrs,
	)

	return h, nil
}

// convertECDSAToLibp2p converts an ECDSA private key to libp2p crypto format
func convertECDSAToLibp2p(cfg *config.Config) (crypto.PrivKey, error) {
	// If no private key in config, generate a new one
	if cfg.NetCfg.PRIV == "" {
		iceLogger().Warnw("No private key in config, generating new identity")
		privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}
		return privKey, nil
	}

	// Decode the ECDSA private key from config
	// The key is stored in PEM format, we need to convert it
	ecdsaKey, err := decodeECDSAPrivateKey(cfg.NetCfg.PRIV)
	if err != nil {
		iceLogger().Warnw("Failed to decode ECDSA key, generating new identity", "error", err)
		privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}
		return privKey, nil
	}

	// Convert ECDSA to libp2p format
	privKey, _, err := crypto.ECDSAKeyPairFromKey(ecdsaKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ECDSA key: %w", err)
	}

	return privKey, nil
}

// decodeECDSAPrivateKey decodes an ECDSA private key from PEM string
func decodeECDSAPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	// Use the existing types.DecodePrivKey function
	key := types.DecodePrivKey(pemStr)
	if key == nil {
		return nil, fmt.Errorf("failed to decode private key")
	}
	return key, nil
}

// GetHostAddresses returns all multiaddresses for the host
func GetHostAddresses(h host.Host) []multiaddr.Multiaddr {
	return h.Addrs()
}

// GetHostPeerID returns the peer ID of the host
func GetHostPeerID(h host.Host) peer.ID {
	return h.ID()
}

// GetFullAddresses returns full multiaddresses including peer ID
func GetFullAddresses(h host.Host) []string {
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.ID().String()))

	addrs := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		fullAddr := addr.Encapsulate(hostAddr)
		addrs = append(addrs, fullAddr.String())
	}
	return addrs
}

// CloseHost gracefully closes the libp2p host
func CloseHost(h host.Host) error {
	if h == nil {
		return nil
	}
	return h.Close()
}
