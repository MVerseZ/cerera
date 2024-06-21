package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/Arceliar/phony"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
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

type Node interface {
	Context() context.Context
	Host() Host
}

func InitP2PHost(ctx context.Context, cfg config.Config) *Host {
	f, err := os.OpenFile("logfile", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			panic(err)
		}
	}(f)

	log.SetOutput(f)
	// create a new libp2p Host that listens on a TCP port
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	fmt.Println("Found local IPv4 addresses:", len(addrs))
	var localIP string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// if ipnet.IP.To4() != nil {
			localIP = ipnet.IP.String()
			// fmt.Println(Local IPv4 address found:", localIP) // Print the found local IP address
			// break
			// }
		}
	}
	if localIP == "" {
		panic("No local IP address found")
	}
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/" + "127.0.0.1" + "/tcp/" + fmt.Sprintf("%d", cfg.NetCfg.P2P)),
	)
	if err != nil {
		panic(err)
	}

	var p = types.DecodePrivKey(cfg.NetCfg.PRIV)
	var b = types.EncodePrivateKeyToByte(p)
	dHost := &Host{
		Addr:    cfg.NetCfg.ADDR,
		NetHost: h,
		K:       b,
		c:       ctx,
		// Overlay: Flush(currentNodeAddress),
	}
	ConnectToSwarm(dHost)
	go dHost.serviceLoop()

	return dHost
}

func ConnectToSwarm(h *Host) {
	var swarmCfg = "swarm.ddd"
	var s network.Stream
	{ //connect to swarm
		if _, err := os.Stat(swarmCfg); err == nil {
			h.NetType = 0x2
			// check address inside this funaction call
			s = InitClient(h, h.Addr.String())
		} else {
			h.NetType = 0x1
			s = InitServer(h)
		}
	}
	h.Stream = s
}

// isOwnAddress checks if the given address matches any of the host's addresses.
// You might need to adjust the logic based on how types.Address is defined and used.
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

func (h *Host) serviceLoop() {
	var errc chan error

	if errc == nil {
		select {}
	}
}
func (h *Host) HandShake() {
	var p = &Packet{
		T:    0xa,
		Data: []byte("OP_I"),
		EF:   0xa,
	}
	var data, _ = json.Marshal(p)
	var n, _ = h.Stream.Write(data)
	fmt.Printf("Writed data: %d\r\n", data)
	fmt.Printf("Writed len: %d\r\n", n)
	_, err := h.Stream.Write([]byte("\r"))
	if err != nil {
		panic(err)
	}
}
func (h *Host) SetUpHttp(ctx context.Context, cfg config.Config) {
	var rpcRequestMetric = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rpc_requests_hits",
			Help: "Count http rpc requests",
		},
	)
	prometheus.MustRegister(rpcRequestMetric)

	go func() {
		// prometheus metrics
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
func (h *Host) Stop() error {
	var err error
	phony.Block(h, func() {
		err = h._stop()
	})
	return err
}
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
func WriteSwarmData(chainAddress types.Address, mAddress string) {
	var swarmCfg = "swarm.ddd"
	maddr, errAddr := ma.NewMultiaddr(mAddress)
	if errAddr != nil {
		panic(errAddr)
	}
	swarm := make(map[types.Address]ma.Multiaddr)
	swarm[chainAddress] = maddr
	err := os.WriteFile(swarmCfg, []byte(fmt.Sprintf("%s:%s", chainAddress, mAddress)), 0644)
	if err != nil {
		panic(err)
	}
}

// func (h *Host) StreamHandler(stream network.Stream) {
// 	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
// 	for {
// 		time.Sleep(time.Second * 2)
// 		str, _ := rw.ReadString('\n')
// 		if str == "" {
// 			return
// 		}
// 		if str != "\n" {
// 			fmt.Printf("RECEIVED (h): %s\r", str)
// 			if strings.Contains(str, "OP_SYNC") {
// 				// send to swarm
// 				fmt.Printf("SENDED (h): %s\r\n", "OP_SYNC_CLI")
// 				rw.WriteString("OP_SYNC_CLI\n")
// 				rw.Flush()
// 			}
// 		}
// 	}
// }
