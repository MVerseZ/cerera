package network

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var urlName = "localhost:%d"

type Server struct {
	node *Node
	url  string
}

func nodeIdToPort(nodeId int) int {
	return nodeId + 8080
}

func NewServer(ctx context.Context, cfg *config.Config, nodeId int) *Server {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Ошибка при получении IP-адресов:", err)

	}

	fmt.Println("Ваши IP-адреса:")
	fmt.Printf("%d\r\n", len(addrs))
	// for _, addr := range addrs {
	// 	if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
	// 		if ipNet.IP.To4() != nil {
	// 			fmt.Println(ipNet.IP.String())
	// 		}
	// 	}
	// }

	server := &Server{
		NewNode(nodeId, cfg),
		fmt.Sprintf(urlName, nodeIdToPort(nodeId)),
	}
	pubKnownServer := &GossipServer{
		fmt.Sprintf(urlName, nodeIdToPort(nodeId)+10),
		server,
	}
	go pubKnownServer.Start()
	// fmt.Println(server.node)
	return server

}

func (s *Server) Start() {
	s.node.Start()
	ln, err := net.Listen("tcp", s.url)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	fmt.Printf("node start at %s\n", s.url)
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) addPreKnownNode(conn net.Conn) (string, error) {
	s.node.mutex.Lock()
	defer s.node.mutex.Unlock()

	remoteAddr := conn.RemoteAddr().String()
	// Check if the node is already in the known nodes list
	for _, knownNode := range s.node.knownNodes {
		if knownNode.url == remoteAddr {
			fmt.Printf("Node %s is already known\n", remoteAddr)
			return "", errors.New("node is already known\n")
		}
	}

	// Adding the new node to the list of known nodes
	newNode := &KnownNode{
		nodeID: len(s.node.knownNodes) + 1, // Assuming unique nodeID based on count (you may need a better approach for unique IDs)
		url:    remoteAddr,
		pubkey: nil, // Here you can add logic to get the public key if available
	}
	s.node.knownNodes = append(s.node.knownNodes, newNode)
	fmt.Printf("Added new known node: %s\n", remoteAddr)
	return s.url, nil
}

func (s *Server) handleConnection(conn net.Conn) {
	req, err := io.ReadAll(conn)
	if err != nil {
		panic(err)
	}
	s.node.msgQueue <- req
}

func (s *Server) JoinSwarm(gossipAddr string) error {

	pbk := s.node.keypair.pubkey
	msg, err := types.PublicKeyToString(pbk)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	fmt.Printf("Send key:\r\n%s\r\n", msg)
	conn, err := net.Dial("tcp", gossipAddr)
	if err != nil {
		return fmt.Errorf("%s is not online", gossipAddr)
	}
	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader([]byte(msg)))
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	// swardResp, err := io.ReadAll(conn)
	// swardAddrStr = string(swardResp)

	// req := Request{
	// 	msg,
	// 	hex.EncodeToString(generateDigest(msg)),
	// }
	// reqmsg := &RequestMsg{
	// 	"solve",
	// 	int(time.Now().Unix()),
	// 	s.node.NodeID,
	// 	req,
	// }
	// sig, err := s.node.signMessage(reqmsg)
	// // fmt.Printf("SIG:\r\n %d\r\n", sig)
	// // fmt.Printf("SIG LEN:\r\n %d\r\n", len(sig))
	// if err != nil {
	// 	fmt.Printf("%v\n", err)
	// }
	// logBroadcastMsg(hRequest, reqmsg)
	// if err := send(ComposeMsg(hRequest, reqmsg, sig), swardAddrStr); err != nil {
	// 	fmt.Printf("cannot connect to %s. Reason: %s\r\n", swardAddrStr, err)
	// }
	// // c.request = reqmsg
	return nil
}

func (s *Server) SetUpHttp(ctx context.Context, cfg config.Config) {
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
