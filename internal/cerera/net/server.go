package net

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
)

const (
	protocolID   = "/cerera-dht-p2p/1.0"
	rendezvous   = "cerera-libp2p-dht"
	pingInterval = time.Minute * 1
)

type discoveryNotifee struct {
	h host.Host
}

// interface to be called when new  peer is found
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("Discovered peer: %s\n", pi.ID)
	n.h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
	if err := n.h.Connect(context.Background(), pi); err != nil {
		fmt.Printf("Error connecting to peer %s: %v\n", pi.ID, err)
	}
}

func NewServer() {

	// 14.02.2025 - USE LIBP2P, KISS

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Close()

	dht, err := kaddht.New(
		ctx,
		h,
		dhtopts.Client(false),
		dht.ProtocolPrefix(protocolID),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Bootstrap DHT
	if err = dht.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	bootstrapPeers := []string{
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}

	for _, addr := range bootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			log.Fatal(err)
		}

		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			log.Fatal(err)
		}

		h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
		if err := h.Connect(ctx, *pi); err != nil {
			fmt.Printf("Error connecting to bootstrap node %s: %v\n", pi.ID, err)
		} else {
			fmt.Printf("Connected to bootstrap node %s\n", pi.ID)
		}
	}

	// Настройка mDNS для локального обнаружения
	mdnsService := mdns.NewMdnsService(
		h,
		rendezvous,
		&discoveryNotifee{h: h},
	)
	if err := mdnsService.Start(); err != nil {
		log.Fatal(err)
	}

	h.SetStreamHandler(protocolID, func(s network.Stream) {
		defer s.Close()
		fmt.Printf("New stream from %s\n", s.Conn().RemotePeer())

		buf := make([]byte, 1024)
		n, err := s.Read(buf)
		if err != nil {
			fmt.Printf("Error reading: %v\n", err)
			return
		}

		fmt.Printf("Received: %s\n", string(buf[:n]))
	})

	fmt.Println("Node ID:", h.ID())
	fmt.Println("Addresses:")
	for _, addr := range h.Addrs() {
		fmt.Println(" ", addr)
	}

	// DHT write
	go func() {
		time.Sleep(10 * time.Second) // wait 4 bootstrap
		key := "example-key"
		value := []byte("example-value")

		fmt.Printf("Storing %s => %s in DHT\n", key, value)
		if err := dht.PutValue(ctx, key, value); err != nil {
			fmt.Printf("Failed to store value: %v\n", err)
		}
	}()

	// read DHT
	go func() {
		time.Sleep(20 * time.Second)
		key := "example-key"

		fmt.Printf("Looking up %s in DHT...\n", key)
		value, err := dht.GetValue(ctx, key)
		if err != nil {
			fmt.Printf("Failed to get value: %v\n", err)
			return
		}
		fmt.Printf("Retrieved %s => %s\n", key, string(value))
	}()
}
