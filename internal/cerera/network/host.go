package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Arceliar/phony"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DiscoveryInterval = time.Hour
const DiscoveryServiceTag = "/vavilov/1.0.0"

type Host struct {
	phony.Inbox
	Addr    types.Address
	NetHost host.Host
	K       []byte

	DataChannel chan []byte

	c  context.Context
	cr *ChatRoom

	// Overlay Swarm
	Status  byte
	Stream  network.Stream
	NetType byte

	// Network graph data
	peers []net.Addr
}

var cereraHost *Host

// Node interface defines the structure of a Node in the network
type NodeI interface {
	Context() context.Context
	Host() Host
}

// InitNetworkHost initializes a new host struct
func InitNetworkHost(ctx context.Context, cfg config.Config) {

	// init rpc requests handling in
	// cereraHost.SetUpHttp(ctx, cfg)

	// Find local IP addresses
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	log.Println("Found local IPv4 addresses:", len(addrs))
	var localIP string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && CheckIPAddressType(ipnet.IP.String()) == 2 {
			localIP = ipnet.IP.String()

		}
	}
	if localIP == "" {
		panic("No local IP address found")
	}
	// BASED ON
	// https://github.com/libp2p/go-libp2p/blob/master/examples/pubsub
	var networkType = "tcp"
	h, err := libp2p.New(libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/%s/%s", networkType, strconv.Itoa(cfg.NetCfg.P2P))))
	if err != nil {
		panic(err)
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

	if err := setupDiscovery(h); err != nil {
		panic(err)
	}

	crm, err := JoinChatRoom(ctx, ps, h.ID(), cfg.NetCfg.ADDR, "room")
	if err != nil {
		panic(err)
	}
	cereraHost = &Host{
		Status:  0x1,
		NetType: 0x1,
		peers:   make([]net.Addr, 0),
		cr:      crm,
	}
}

// HandShake performs a handshake over the network stream
func (h *Host) HandShake() {
	p := &Packet{
		T:    0xa,
		Data: []byte("OP_I"),
		EF:   0xa,
	}
	data, _ := json.Marshal(p)
	n, _ := h.Stream.Write(data)
	fmt.Printf("Writed data: %d\r\n", data)
	fmt.Printf("Writed len: %d\r\n", n)
	_, err := h.Stream.Write([]byte("\r"))
	if err != nil {
		panic(err)
	}
}

// SetUpHttp sets up the HTTP server
func (h *Host) SetUpHttp(ctx context.Context, cfg config.Config) {
	rpcRequestMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rpc_requests_hits",
			Help: "Count http rpc requests",
		},
	)
	prometheus.MustRegister(rpcRequestMetric)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if cfg.SEC.HTTP.TLS {
			err := http.ListenAndServeTLS(fmt.Sprintf(":%d", cfg.NetCfg.RPC), "./server.crt", "./server.key", nil)
			if err != nil {
				fmt.Println("ListenAndServe: ", err)
			}
		} else {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.NetCfg.RPC), nil); err != nil {
				fmt.Println("Error starting server:", err)
			}
		}
	}()

	fmt.Printf("Starting http server at port %d\r\n", cfg.NetCfg.RPC)
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))
}

// Stop stops the host
func (h *Host) Stop() error {
	var err error
	phony.Block(h, func() {
		err = h._stop()
	})
	return err
}

// _stop is the internal stop function for the host
func (h *Host) _stop() error {
	h.Status = 0xf
	if h.NetHost != nil {
		err := h.NetHost.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("discovered new peer %s\n", pi.ID)
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID, err)
	}
}

func setupDiscovery(h host.Host) error {
	// setup mDNS discovery to find local peers
	s := mdns.NewMdnsService(h, DiscoveryServiceTag, &discoveryNotifee{h: h})
	return s.Start()
}
