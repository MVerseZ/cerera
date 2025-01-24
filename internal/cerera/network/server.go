package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea/gigea"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var urlName = "0.0.0.0:%d"
var transportServer *Server

func GetTransport() *Server {
	return transportServer
}

type Server struct {
	node        *Node
	url         string
	wsListeners []*websocket.Conn
}

func nodeIdToPort(nodeId int) int {
	return nodeId + 8090
}

func NewServer(ctx context.Context, cfg *config.Config, nodeId int) *Server {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Ошибка при получении IP-адресов: %s", err)
		// fmt.Println("Ошибка при получении IP-адресов:", err)

	}

	log.Println("Ваши IP-адреса:")
	log.Printf("%d\r\n", len(addrs))
	// for _, addr := range addrs {
	// 	if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
	// 		if ipNet.IP.To4() != nil {
	// 			fmt.Println(ipNet.IP.String())
	// 		}
	// 	}
	// }

	transportServer = &Server{
		NewNode(nodeId, cfg),
		fmt.Sprintf(urlName, nodeIdToPort(nodeId)),
		make([]*websocket.Conn, 0),
	}

	// err = transportServer.JoinSwarm(cfg.Gossip)
	// if err != nil {
	// 	// panic(err)
	// 	fmt.Printf("gigea.network.error! %s\r\n", err)
	// 	gigea.SetStatus(1)
	// 	fmt.Println("Running in local mode")
	// } else {
	// 	gigea.SetStatus(2)
	// 	fmt.Println("Running in full mode")
	// }

	// fmt.Println(server.node)
	return transportServer

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
		s.addKnownNode(conn)
		go s.handleConnection(conn)

	}
}

func (s *Server) addPreKnownNode(conn net.Conn) (string, error) {
	s.node.mutex.Lock()
	defer s.node.mutex.Unlock()

	remoteAddr := conn.RemoteAddr().String()
	// Check if the node is already in the known nodes list
	for _, preKnownNode := range s.node.preKnownNodes {
		if preKnownNode.url == remoteAddr {
			fmt.Printf("Node %s is already known\n", remoteAddr)
			return "", errors.New("node is already known\n")
		}
	}

	// Adding the new node to the list of known nodes
	newNode := &PreKnownNode{
		nodeID: -1, // Assuming unique nodeID based on count (you may need a better approach for unique IDs)
		url:    remoteAddr,
		pubkey: nil, // Here you can add logic to get the public key if available
	}
	s.node.preKnownNodes = append(s.node.preKnownNodes, newNode)
	fmt.Printf("Added new pre known node: %s\n", remoteAddr)
	gigea.SetStatus(2)
	fmt.Println("Running in full mode")
	return s.url, nil
}

func (s *Server) addKnownNode(conn net.Conn) (string, error) {
	s.node.mutex.Lock()
	defer s.node.mutex.Unlock()
	defer conn.Close()

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
		nodeID:     -1, // Assuming unique nodeID based on count (you may need a better approach for unique IDs)
		url:        remoteAddr,
		pubkey:     nil, // Here you can add logic to get the public key if available
		connection: conn,
	}
	s.node.knownNodes = append(s.node.knownNodes, newNode)
	fmt.Printf("Added new known node: %s\n", remoteAddr)
	gigea.SetStatus(2)
	return s.url, nil
}

func (s *Server) removeKnownNode(conn net.Conn) (string, error) {
	s.node.mutex.Lock()
	defer s.node.mutex.Unlock()
	conn.Close()
	return "", nil
}

func (s *Server) handleConnection(conn net.Conn) {
	req, err := io.ReadAll(conn)
	fmt.Printf("connectiong %s to current %s\r\n", conn.RemoteAddr(), conn.LocalAddr())
	if err != nil {
		// s.removeKnownNode(conn)
		fmt.Errorf("Connection closed: %s", err)
	}
	s.node.msgQueue <- req
}

func (s *Server) JoinSwarm(gossipAddr string) error {
	if len(gossipAddr) > 0 {
		pbk := s.node.keypair.pubkey
		msg, err := types.PublicKeyToString(pbk)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		fmt.Printf("Send key: %s\r\n", msg)

		req := Request{
			msg,
			hex.EncodeToString(generateDigest(msg)),
		}
		reqmsg := &JoinMsg{
			"join",
			int(time.Now().Unix()),
			s.node.NodeID,
			req,
		}
		sig, err := signMessage(reqmsg, s.node.keypair.privkey)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		// Dial("ip4:1", "192.0.2.1")
		fmt.Printf("dial gossip: %s\r\n", gossipAddr)

		laddr, err := net.ResolveTCPAddr("tcp", s.url)
		if err != nil {
			return fmt.Errorf("error while parse address %s", s.url)
		}
		raddr, err := net.ResolveTCPAddr("tcp", gossipAddr)
		if err != nil {
			return fmt.Errorf("error while parse address  %s", gossipAddr)
		}
		conn, err := net.DialTCP("tcp", laddr, raddr)
		// conn, err := net.Dial("tcp", gossipAddr)
		if err != nil {
			return fmt.Errorf("%s is not online", gossipAddr)
		}
		defer conn.Close()

		_, err = io.Copy(conn, bytes.NewReader(ComposeMsg(hJoin, reqmsg, sig)))
		if err != nil {
			return fmt.Errorf("%v", err)
		}
		return nil
	} else {
		return fmt.Errorf("%s", "empty/wrong gossip address!")
	}
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

	// fmt.Printf("Starting http server at port %d\r\n", cfg.NetCfg.RPC)
	log.Printf("Starting http server at port %d\r\n", cfg.NetCfg.RPC)
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))
}

func AddWsListener(conn *websocket.Conn) {

}
