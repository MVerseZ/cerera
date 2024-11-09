package network

import (
	"fmt"
	"io"
	"net"
)

type GossipServer struct {
	url string
	srv *Server
}

func (s *GossipServer) Start() {
	ln, err := net.Listen("tcp", s.url)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	fmt.Printf("gossip node start at %s\n", s.url)
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go s.handleConnection(conn)
	}
}

func (s *GossipServer) handleConnection(conn net.Conn) {
	req, err := io.ReadAll(conn)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Gossip data get: %s\r\n", req)
	gossipData, err := s.srv.addPreKnownNode(conn)
	if err != nil {
		conn.Write([]byte{0xe, 0xf, 0xf, 0x0, 0xf})
	}
	conn.Write([]byte(gossipData))
	// s.node.msgQueue <- req
}
