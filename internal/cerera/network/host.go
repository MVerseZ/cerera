package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Arceliar/phony"
	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/consensus"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/pallada/pallada"
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

	// Network graph data
	peers []net.Addr
	// connection <-->
	conn net.Conn
}

var cereraHost *Host

// Node interface defines the structure of a Node in the network
type Node interface {
	Context() context.Context
	Host() Host
}

func CheckIPAddressType(ip string) int {
	if net.ParseIP(ip) == nil {
		log.Printf("Invalid IP Address: %s\n", ip)
		return 1
	}
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			log.Printf("Given IP Address %s is IPV4 type\n", ip)
			return 2
		case ':':
			log.Printf("Given IP Address %s is IPV6 type\n", ip)
			return 3
		}
	}
	return 4
}

// InitNetworkHost initializes a new host struct
func InitNetworkHost(ctx context.Context, cfg config.Config) {

	cereraHost = &Host{
		Status:  0x1,
		NetType: 0x1,
		peers:   make([]net.Addr, 0),
	}

	// init rpc requests handling in
	cereraHost.SetUpHttp(ctx, cfg)
	cereraHost.Status = cereraHost.Status << 1

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

	// !!! solution without libp2p
	var networkType = "tcp" // tcp4 tcp6
	listener, err := net.Listen(networkType, localIP+":"+strconv.Itoa(cfg.NetCfg.P2P))
	if err != nil {
		panic(err)
	}
	cereraHost.Status = cereraHost.Status << 1
	fmt.Printf("Start network host at: %s with cerera address: %s\r\n",
		listener.Addr(), cfg.NetCfg.ADDR)
	consensus.Add(listener.Addr(), cfg.NetCfg.ADDR)
	fmt.Printf("Init client...")
	go InitClient(cfg.NetCfg.ADDR)
	fmt.Printf("Consensus status: %f\r\n", consensus.ConsensusStatus())
	fmt.Printf("Status: %x\r\n", cereraHost.Status)
	fmt.Printf("Wait for peers...\r\n")

	for {
		var incomingConnection, err = listener.Accept()
		if err != nil {
			log.Println(err)
			incomingConnection.Close()
			continue
		}
		var remoteAddr = incomingConnection.RemoteAddr()
		fmt.Printf("Client is: %s\r\n", remoteAddr)
		cereraHost.peers = append(cereraHost.peers, remoteAddr)
		go customHandleConnection(incomingConnection)
	}

}

// serviceLoop handles the service loop for the host
func (h *Host) serviceLoop() {
	var errc chan error

	// h.NetHost.

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

func customHandleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			log.Println("error closing connection:", err)
		}
	}()

	// create json encoder/decoder using the io.Conn as
	// io.Writer and io.Reader for streaming IO
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	// command-loop
	for {
		// Next decode the incoming data into Go value
		var req Request
		var resp Response
		if err := dec.Decode(&req); err != nil {
			log.Println("failed to unmarshal request:", err)
			return
		}
		// result
		result := req.Method

		if strings.Contains(result, "consensus") {
			resp.ID = req.ID
			resp.JSONRPC = req.JSONRPC
			params := req.Params
			fmt.Println(result)
			fmt.Println(params...)

			var latestResp Response
			latestResp.ID = req.ID
			latestResp.JSONRPC = req.JSONRPC
			latestResp.Result = pallada.Execute(result, params)
			if err := enc.Encode(&latestResp); err != nil {
				fmt.Println("failed to encode data:", err)
				return
			}

		} else {
			fmt.Println("send default response:", resp)
			// encode result to JSON array
			if err := enc.Encode(&resp); err != nil {
				fmt.Println("failed to encode data:", err)
				return
			}
		}
	}

}

///// CLIENT CODE

type Client struct {
	addr   types.Address
	status byte
}

var client Client
var (
	pollMinutes int = 10
)

func InitClient(cereraAddress types.Address) {
	time.Sleep(5 * time.Second)
	c, err := net.Dial("tcp", "172.23.32.1:6116")

	if err != nil {
		fmt.Println(err)
		return
	}
	defer c.Close()

	client = Client{
		addr:   cereraAddress,
		status: 0x1,
	}

	go customHandleConnectionClient(c)

	for {
		//time.Sleep(pollMinutes * time.Minute)
		time.Sleep(time.Duration(3) * time.Second)
	}
}

func customHandleConnectionClient(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Println("error closing connection:", err)
		}
	}()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var resp Response
	var reqParams = []interface{}{client.addr}
	hReq := Request{
		JSONRPC: "2.0",
		Method:  "cerera.consensus.join",
		Params:  reqParams,
		ID:      5422899109,
	}

	if err := enc.Encode(&hReq); err != nil {
		fmt.Println("failed to encode data:", err)
		return
	}

	for {

		if err := dec.Decode(&resp); err != nil {
			fmt.Println("failed to unmarshal request:", err)
			return
		}
		// result
		result := resp.Result
		fmt.Printf("Current client status: %x\r\n", client.status)
		switch v := result.(type) {
		case map[string]interface{}:

			switch s := client.status; s {
			case 0x1:
				tmpJson, err := json.Marshal(v)
				if err != nil {
					fmt.Println(err)
					continue
				}
				var b block.Block
				if err := json.Unmarshal(tmpJson, &b); err != nil {
					fmt.Println(err)
					return
				}

				// fmt.Println(currentBlock.GetLatestBlock().Hash())
				// fmt.Println(b.Hash())

				var syncParams []interface{}
				fmt.Println("METHOD WITH CHAIN")
				var currentBlock = chain.GetBlockChain().GetLatestBlock()
				if b.Hash().String() != currentBlock.Hash().String() {
					if b.Head.Number.Cmp(currentBlock.Head.Number) > 0 {
						var diff = big.NewInt(0).Sub(b.Head.Number, currentBlock.Head.Number)
						syncParams = []interface{}{diff}
					} else {
						syncParams = []interface{}{0}
					}
				} else {
					syncParams = []interface{}{currentBlock.Head.Number}
				}
				hReq := Request{
					JSONRPC: "2.0",
					Method:  "cerera.consensus.sync",
					Params:  syncParams,
					ID:      5422899110,
				}
				if err := enc.Encode(&hReq); err != nil {
					fmt.Println("failed to encode data:", err)
					return
				}

				// 	}
				// }
				client.status = 0x2
			case 0x2:
				tmpJson, err := json.Marshal(v)
				if err != nil {
					fmt.Println(err)
					continue
				}
				var b block.Block
				if err := json.Unmarshal(tmpJson, &b); err != nil {
					fmt.Println(err)
					return
				}
				fmt.Println("METHOD WITH CHAIN")
				// chain.GetBlockChain().UpdateChain(&b)
				client.status += 1
			default:

			}

		case string:
			fmt.Printf("block_str: %s\r\n", v)
		case float64:
			fmt.Printf("cons stat: %f\r\n", v)
		case map[string]map[string]interface{}:
			fmt.Printf("SWARM BLOCKS\r\n")
		case interface{}:
			fmt.Printf("SWARM BLOCKS ARR\r\n")
			// receive blocks and fullfilled chain

			hReq := Request{
				JSONRPC: "2.0",
				Method:  "cerera.consensus.ready",
				Params:  nil,
				ID:      5422899110,
			}
			if err := enc.Encode(&hReq); err != nil {
				fmt.Println("failed to encode data:", err)
				return
			}
			client.status += 1
		default:
			fmt.Println(v)
			fmt.Println("unknown")
		}
	}
}
