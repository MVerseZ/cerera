package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/Arceliar/phony"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DiscoveryServiceTag = "/vavilov/1.0.0"

type Host struct {
	phony.Inbox
	Addr    types.Address
	NetHost host.Host
	K       []byte

	DataChannel chan []byte

	c context.Context

	// Overlay Swarm
	Status  byte
	Stream  network.Stream
	NetType byte
}

// Node interface defines the structure of a Node in the network
type Node interface {
	Context() context.Context
	Host() Host
}

func CheckIPAddressType(ip string) int {
	if net.ParseIP(ip) == nil {
		fmt.Printf("Invalid IP Address: %s\n", ip)
		return 1
	}
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			fmt.Printf("Given IP Address %s is IPV4 type\n", ip)
			return 2
		case ':':
			fmt.Printf("Given IP Address %s is IPV6 type\n", ip)
			return 3
		}
	}
	return 4
}

// InitP2PHost initializes a new P2P host
func InitP2PHost(ctx context.Context, cfg config.Config) *Host {
	// Open log file
	f, err := os.OpenFile("logfile", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

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

	// Create a new libp2p Host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/" + localIP + "/tcp/" + strconv.Itoa(cfg.NetCfg.P2P)),
	)
	if err != nil {
		panic(err)
	}

	// Initialize the Host struct
	p := types.DecodePrivKey(cfg.NetCfg.PRIV)
	b := types.EncodePrivateKeyToByte(p)
	dHost := &Host{
		Addr:    cfg.NetCfg.ADDR,
		NetHost: h,
		K:       b,
		c:       ctx,
	}
	log.Println("Create host with cerera addr:", dHost.Addr)
	log.Println("Create host with net addrs:", dHost.NetHost.Addrs())

	// Connect to Swarm
	// ConnectToSwarm(dHost)
	go dHost.serviceLoop()

	return dHost
}

// ConnectToSwarm connects the host to a swarm
func ConnectToSwarm(h *Host) {
	const swarmCfg = "swarm.ddd"
	var s network.Stream

	if _, err := os.Stat(swarmCfg); err == nil {
		h.NetType = 0x2
		s = InitClient(h, h.Addr.String())
	} else {
		h.NetType = 0x1
		s = InitServer(h)
	}
	h.Stream = s
}

// isOwnAddress checks if the given address matches any of the host's addresses
func isOwnAddress(addr string) bool {
	host, err := os.Hostname()
	if err != nil {
		log.Printf("Unable to get hostname: %v", err)
		return false
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		log.Printf("Unable to lookup host: %v", err)
		return false
	}
	for _, a := range addrs {
		if a == addr {
			return true
		}
	}
	return false
}

// serviceLoop handles the service loop for the host
func (h *Host) serviceLoop() {
	var errc chan error

	if errc == nil {
		select {}
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

func (h *Host) SetUpProtocol() {

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

// WriteSwarmData writes the swarm data to a file
func WriteSwarmData(chainAddress types.Address, mAddress string) {
	const swarmCfg = "swarm.ddd"
	maddr, errAddr := multiaddr.NewMultiaddr(mAddress)
	if errAddr != nil {
		panic(errAddr)
	}
	swarm := make(map[types.Address]multiaddr.Multiaddr)
	swarm[chainAddress] = maddr
	err := os.WriteFile(swarmCfg, []byte(fmt.Sprintf("%s:%s", chainAddress, mAddress)), 0644)
	if err != nil {
		panic(err)
	}
}
