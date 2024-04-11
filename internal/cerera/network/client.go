package network

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

type Client struct {
}

func InitClient(h *Host) network.Stream {
	var swarmCfg = "swarm.ddd"
	f, err := os.Open(swarmCfg)
	if err != nil {
		panic(err)
	}
	b1 := make([]byte, 80)
	_, err = f.Read(b1)
	if err != nil {
		panic(err)
	}
	swarmArr := strings.Split(string(b1), "\r\n")
	fmt.Printf("Swarm is:%s\r\n", swarmArr[0])
	fmt.Printf("Joining\r\n")

	maddr, err := ma.NewMultiaddr(swarmArr[0])
	if err != nil {
		panic(err)
	}
	remoteHost, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		panic(err)
	}
	if h.NetHost.Addrs()[0] == maddr {
		return nil
	}
	h.NetHost.Peerstore().AddAddrs(remoteHost.ID, remoteHost.Addrs, peerstore.PermanentAddrTTL)
	s, err := h.NetHost.NewStream(context.Background(), remoteHost.ID, DiscoveryServiceTag)
	if err != nil {
		panic(err)
	}
	h.Status = 0x2
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	go h.ClientProtocol(rw)
	h.Stream = s
	return h.Stream
}
