package icenet

import (
	"context"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/config"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

const (
	// DiscoveryNamespace is the namespace for peer discovery
	DiscoveryNamespace = "cerera-network"
	// DiscoveryInterval is the interval between discovery attempts
	DiscoveryInterval = 30 * time.Second
	// ConnectionTimeout is the timeout for connecting to peers
	ConnectionTimeout = 30 * time.Second
)

// Discovery handles peer discovery via DHT and bootstrap nodes
type Discovery struct {
	host              host.Host
	dht               *dht.IpfsDHT
	mdnsService       mdns.Service
	cfg               *config.Config
	ctx               context.Context
	cancel            context.CancelFunc
	bootstrapPeers    []peer.AddrInfo
	rawBootstrapAddrs []multiaddr.Multiaddr // Addresses without peer ID for raw dialing
	mu                sync.RWMutex
	connected         map[peer.ID]bool
}

// discoveryNotifee handles mDNS discovery notifications
type discoveryNotifee struct {
	d *Discovery
}

// HandlePeerFound is called when mDNS discovers a new peer
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Skip self
	if pi.ID == n.d.host.ID() {
		return
	}

	iceLogger().Debugw("mDNS discovered peer", "peer", pi.ID, "addrs", pi.Addrs)

	// Try to connect
	ctx, cancel := context.WithTimeout(n.d.ctx, ConnectionTimeout)
	defer cancel()

	if err := n.d.host.Connect(ctx, pi); err != nil {
		iceLogger().Warnw("Failed to connect to mDNS peer", "peer", pi.ID, "error", err)
		return
	}

	iceLogger().Debugw("Connected to mDNS peer", "peer", pi.ID)
}

// NewDiscovery creates a new peer discovery service
func NewDiscovery(ctx context.Context, h host.Host, cfg *config.Config) (*Discovery, error) {
	ctx, cancel := context.WithCancel(ctx)

	d := &Discovery{
		host:      h,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		connected: make(map[peer.ID]bool),
	}

	// Parse bootstrap peers from config
	if err := d.parseBootstrapPeers(); err != nil {
		iceLogger().Warnw("Failed to parse some bootstrap peers", "error", err)
	}

	// Initialize DHT
	if err := d.initDHT(); err != nil {
		cancel()
		return nil, err
	}

	return d, nil
}

// parseBootstrapPeers parses bootstrap nodes from config into peer.AddrInfo
func (d *Discovery) parseBootstrapPeers() error {
	d.bootstrapPeers = make([]peer.AddrInfo, 0)
	d.rawBootstrapAddrs = make([]multiaddr.Multiaddr, 0)

	bootstrapNodes := d.cfg.NetCfg.BootstrapNodes
	if len(bootstrapNodes) == 0 {
		bootstrapNodes = d.cfg.NetCfg.SeedNodes
	}

	for _, addrStr := range bootstrapNodes {
		// Skip empty addresses
		if addrStr == "" {
			continue
		}

		// Parse multiaddr
		maddr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			iceLogger().Warnw("Failed to parse bootstrap address", "addr", addrStr, "error", err)
			continue
		}

		// Try to extract peer ID if present
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			// Address doesn't contain peer ID, store for raw dialing
			d.rawBootstrapAddrs = append(d.rawBootstrapAddrs, maddr)
			iceLogger().Debugw("Bootstrap address without peer ID, will try raw dial", "addr", addrStr)
			continue
		}

		d.bootstrapPeers = append(d.bootstrapPeers, *peerInfo)
		iceLogger().Infow("Added bootstrap peer", "peer", peerInfo.ID, "addrs", peerInfo.Addrs)
	}

	return nil
}

// initDHT initializes the Kademlia DHT
func (d *Discovery) initDHT() error {
	var dhtMode dht.ModeOpt

	// Use server mode if configured (for bootstrap nodes)
	if d.cfg.NetCfg.DHTServerMode {
		dhtMode = dht.ModeServer
	} else {
		dhtMode = dht.ModeAutoServer
	}

	kadDHT, err := dht.New(d.ctx, d.host,
		dht.Mode(dhtMode),
		dht.ProtocolPrefix("/cerera"),
	)
	if err != nil {
		return err
	}

	d.dht = kadDHT

	// Bootstrap the DHT
	if err := kadDHT.Bootstrap(d.ctx); err != nil {
		iceLogger().Warnw("Failed to bootstrap DHT", "error", err)
	}

	iceLogger().Infow("DHT initialized", "mode", dhtMode)
	return nil
}

// Start begins the discovery process
func (d *Discovery) Start() error {
	// Start mDNS discovery for local network
	if err := d.startMDNS(); err != nil {
		iceLogger().Warnw("Failed to start mDNS discovery", "error", err)
	}

	// Connect to bootstrap peers
	go d.connectToBootstrapPeers()

	// Start continuous discovery
	go d.discoverPeers()

	// Setup connection notifier
	d.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			d.mu.Lock()
			d.connected[conn.RemotePeer()] = true
			d.mu.Unlock()
			iceLogger().Debugw("Peer connected",
				"peer", conn.RemotePeer(),
				"addr", conn.RemoteMultiaddr(),
				"direction", conn.Stat().Direction,
			)
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			d.mu.Lock()
			delete(d.connected, conn.RemotePeer())
			d.mu.Unlock()
			iceLogger().Infow("Peer disconnected", "peer", conn.RemotePeer())
		},
	})

	return nil
}

// startMDNS initializes mDNS discovery for local network peer discovery
func (d *Discovery) startMDNS() error {
	notifee := &discoveryNotifee{d: d}
	service := mdns.NewMdnsService(d.host, DiscoveryNamespace, notifee)
	if err := service.Start(); err != nil {
		return err
	}

	d.mdnsService = service
	iceLogger().Infow("mDNS discovery started", "service", DiscoveryNamespace)
	return nil
}

// connectToBootstrapPeers connects to all configured bootstrap peers
func (d *Discovery) connectToBootstrapPeers() {
	var wg sync.WaitGroup

	for _, peerInfo := range d.bootstrapPeers {
		// Skip self
		if peerInfo.ID == d.host.ID() {
			continue
		}

		wg.Add(1)
		go func(pi peer.AddrInfo) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(d.ctx, ConnectionTimeout)
			defer cancel()

			if err := d.host.Connect(ctx, pi); err != nil {
				iceLogger().Warnw("Failed to connect to bootstrap peer",
					"peer", pi.ID,
					"error", err,
				)
				return
			}

			iceLogger().Infow("Connected to bootstrap peer", "peer", pi.ID)
		}(peerInfo)
	}

	wg.Wait()
	iceLogger().Infow("Bootstrap connection phase complete",
		"connected", len(d.host.Network().Peers()),
	)
}

// discoverPeers continuously discovers new peers via DHT
func (d *Discovery) discoverPeers() {
	routingDiscovery := routing.NewRoutingDiscovery(d.dht)

	// Advertise ourselves
	util.Advertise(d.ctx, routingDiscovery, DiscoveryNamespace)
	iceLogger().Infow("Advertising on DHT", "namespace", DiscoveryNamespace)

	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	// Initial discovery
	d.findPeers(routingDiscovery)

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.findPeers(routingDiscovery)
		}
	}
}

// findPeers searches for peers in the DHT
func (d *Discovery) findPeers(routingDiscovery *routing.RoutingDiscovery) {
	peerChan, err := routingDiscovery.FindPeers(d.ctx, DiscoveryNamespace)
	if err != nil {
		iceLogger().Warnw("Failed to find peers", "error", err)
		return
	}

	for peerInfo := range peerChan {
		// Skip self
		if peerInfo.ID == d.host.ID() {
			continue
		}

		// Skip already connected peers
		if d.host.Network().Connectedness(peerInfo.ID) == network.Connected {
			continue
		}

		// Try to connect
		go func(pi peer.AddrInfo) {
			ctx, cancel := context.WithTimeout(d.ctx, ConnectionTimeout)
			defer cancel()

			if err := d.host.Connect(ctx, pi); err != nil {
				iceLogger().Debugw("Failed to connect to discovered peer",
					"peer", pi.ID,
					"error", err,
				)
				return
			}

			iceLogger().Infow("Connected to discovered peer", "peer", pi.ID)
		}(peerInfo)
	}
}

// GetDHT returns the underlying DHT instance
func (d *Discovery) GetDHT() *dht.IpfsDHT {
	return d.dht
}

// GetConnectedPeers returns the list of connected peer IDs
func (d *Discovery) GetConnectedPeers() []peer.ID {
	return d.host.Network().Peers()
}

// GetConnectedPeerCount returns the number of connected peers
func (d *Discovery) GetConnectedPeerCount() int {
	return len(d.host.Network().Peers())
}

// Stop stops the discovery service
func (d *Discovery) Stop() {
	d.cancel()
	if d.mdnsService != nil {
		if err := d.mdnsService.Close(); err != nil {
			iceLogger().Warnw("Error closing mDNS service", "error", err)
		}
	}
	if d.dht != nil {
		if err := d.dht.Close(); err != nil {
			iceLogger().Warnw("Error closing DHT", "error", err)
		}
	}
	iceLogger().Infow("Discovery service stopped")
}

// ConnectToPeer connects to a specific peer
func (d *Discovery) ConnectToPeer(ctx context.Context, peerInfo peer.AddrInfo) error {
	return d.host.Connect(ctx, peerInfo)
}

// AddBootstrapPeer adds a new bootstrap peer
func (d *Discovery) AddBootstrapPeer(peerInfo peer.AddrInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if already exists
	for _, p := range d.bootstrapPeers {
		if p.ID == peerInfo.ID {
			return
		}
	}

	d.bootstrapPeers = append(d.bootstrapPeers, peerInfo)
	iceLogger().Infow("Added new bootstrap peer", "peer", peerInfo.ID)
}
