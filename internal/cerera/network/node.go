package network

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

type P2PHost struct {
}

func InitServer(h *Host) network.Stream {
	fmt.Printf("Swarm is empty now. Init nearest...\r\n")

	// // Let's get the actual TCP port from our listen multiaddr, in case we're using 0 (default; random available port).
	var port string
	for _, la := range h.NetHost.Network().ListenAddresses() {
		if p, err := la.ValueForProtocol(ma.P_TCP); err == nil {
			port = p
			break
		}
	}
	if port == "" {
		panic("was not able to find actual local port")
	}
	var addr = h.NetHost.Addrs()[0]
	fmt.Printf("My address is: %s\r\n", fmt.Sprintf("%s/p2p/%s", addr, h.NetHost.ID()))
	var endPointAddress = fmt.Sprintf("%s/p2p/%s", addr, h.NetHost.ID())
	WriteSwarmData(h.Addr, endPointAddress)
	h.NetHost.SetStreamHandler("/vavilov/1.0.0", h.ServerProtocol)
	h.Status = 0x2
	return h.Stream
}
