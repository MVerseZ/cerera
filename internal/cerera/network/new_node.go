package network

// import (
//     "context"
//     "fmt"
//     "sync"

//     "github.com/libp2p/go-libp2p"
//     "github.com/libp2p/go-libp2p/core/host"
//     "github.com/libp2p/go-libp2p/core/network"
//     "github.com/libp2p/go-libp2p/core/peer"
//     "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
//     dht "github.com/libp2p/go-libp2p-kad-dht"
//     "github.com/libp2p/go-libp2p/p2p/discovery/routing"
//     "github.com/libp2p/go-libp2p/p2p/transport/upnp"
//     rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
//     "github.com/multiformats/go-multiaddr"
// )

// // DiscoveryServiceTag for mDNS
// const DiscoveryServiceTag = "p2p-example"

// // Default bootstrap nodes
// var defaultBootstrapAddrs = []string{
//     "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
//     "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqbiS7Y5P",
//     "/dnsaddr/bootstrap.libp2p.io/p2p/QmZa1sAx2BNrL5k4u136VgmafMZD5p8f4fM9nR9KteyW3r",
// }

// // discoveryNotifee for mDNS
// type discoveryNotifee struct {
//     PeerChan chan peer.AddrInfo
// }

// func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
//     n.PeerChan <- pi
// }

// func main() {
//     ctx := context.Background()

//     // Configure resource manager with more permissive limits
//     limits := rcmgr.DefaultLimits.AutoScale()
//     rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits))
//     if err != nil {
//         panic(err)
//     }

//     // Create a new libp2p Host with NAT traversal
//     h, err := libp2p.New(
//         libp2p.ListenAddrStrings(
//             "/ip4/0.0.0.0/tcp/0",
//             "/ip6/::/tcp/0",
//             "/ip4/0.0.0.0/udp/0/quic",
//         ),
//         libp2p.NATPortMap(),            // Enable UPnP/NAT-PMP
//         libp2p.EnableRelay(),           // Enable circuit relay
//         libp2p.EnableHolePunching(),    // Enable NAT hole punching
//         libp2p.ResourceManager(rm),     // Custom resource manager
//         libp2p.Transport(upnp.New()),   // UPnP transport
//     )
//     if err != nil {
//         panic(err)
//     }
//     defer h.Close()

//     // Set up stream handler
//     h.SetStreamHandler("/chat/1.0.0", func(s network.Stream) {
//         defer s.Close()
//         buf := make([]byte, 1024)
//         n, err := s.Read(buf)
//         if err != nil {
//             fmt.Printf("Error reading from stream: %v\n", err)
//             return
//         }
//         fmt.Printf("Received message from %s: %s\n", s.Conn().RemotePeer().String(), string(buf[:n]))
//     })

//     // Create peer channel
//     peerChan := make(chan peer.AddrInfo)

//     // Set up DHT
//     kademliaDHT, err := dht.New(ctx, h)
//     if err != nil {
//         panic(err)
//     }

//     // Bootstrap the DHT
//     if err = kademliaDHT.Bootstrap(ctx); err != nil {
//         panic(err)
//     }

//     // Connect to bootstrap nodes
//     var wg sync.WaitGroup
//     for _, addrStr := range defaultBootstrapAddrs {
//         addr, err := multiaddr.NewMultiaddr(addrStr)
//         if err != nil {
//             fmt.Printf("Error parsing bootstrap addr: %v\n", err)
//             continue
//         }
//         peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
//         if err != nil {
//             fmt.Printf("Error getting peer info: %v\n", err)
//             continue
//         }
//         wg.Add(1)
//         go func(pi peer.AddrInfo) {
//             defer wg.Done()
//             if err := h.Connect(ctx, pi); err != nil {
//                 fmt.Printf("Failed to connect to bootstrap peer: %v\n", err)
//             } else {
//                 fmt.Printf("Connected to bootstrap peer: %s\n", pi.ID.String())
//             }
//         }(*peerInfo)
//     }

//     // Set up routing discovery
//     routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

//     // Advertise ourselves
//     _, err = routingDiscovery.Advertise(ctx, "/p2p-example/1.0.0")
//     if err != nil {
//         panic(err)
//     }

//     // Set up mDNS for local discovery
//     mdnsService, err := mdns.NewMdnsService(ctx, h, DiscoveryServiceTag, &discoveryNotifee{peerChan})
//     if err != nil {
//         fmt.Printf("mDNS setup failed: %v\n", err)
//     } else {
//         defer mdnsService.Close()
//     }

//     // Handle discovered peers
//     wg.Add(1)
//     go func() {
//         defer wg.Done()
//         for {
//             select {
//             case pi := <-peerChan:
//                 handlePeer(ctx, h, pi)
//             case <-ctx.Done():
//                 return
//             }
//         }
//     }()

//     // Search for other peers using DHT
//     go func() {
//         peerChanDHT, err := routingDiscovery.FindPeers(ctx, "/p2p-example/1.0.0")
//         if err != nil {
//             fmt.Printf("DHT discovery failed: %v\n", err)
//             return
//         }
//         for pi := range peerChanDHT {
//             if pi.ID == h.ID() {
//                 continue
//             }
//             handlePeer(ctx, h, pi)
//         }
//     }()

//     fmt.Printf("Host ID: %s\n", h.ID().String())
//     fmt.Printf("Listening on: %v\n", h.Addrs())
//     fmt.Println("Started P2P node with NAT traversal and relay support")

//     // Keep the program running
//     select {}
// }

// func handlePeer(ctx context.Context, h host.Host, pi peer.AddrInfo) {
//     if err := h.Connect(ctx, pi); err != nil {
//         fmt.Printf("Failed to connect to peer %s: %v\n", pi.ID.String(), err)
//         return
//     }

//     s, err := h.NewStream(ctx, pi.ID, "/chat/1.0.0")
//     if err != nil {
//         fmt.Printf("Failed to create stream to %s: %v\n", pi.ID.String(), err)
//         return
//     }
//     defer s.Close()

//     msg := fmt.Sprintf("Hello from %s!", h.ID().String())
//     _, err = s.Write([]byte(msg))
//     if err != nil {
//         fmt.Printf("Failed to send message to %s: %v\n", pi.ID.String(), err)
//     }
//     fmt.Printf("Sent message to peer: %s\n", pi.ID.String())
// }
