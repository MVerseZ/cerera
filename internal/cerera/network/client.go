package network

import (
	"bufio"
	"context"
	"fmt"
	"os"

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
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var swarmArr []string
	for scanner.Scan() {
		swarmArr = append(swarmArr, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
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
