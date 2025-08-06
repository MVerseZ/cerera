package net

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea/gigea"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	ma "github.com/multiformats/go-multiaddr"
)

const protocolID = "/cerera-p2p/1.0.0"

var CereraNode *Node

type Node struct {
	h       host.Host
	address types.Address
	ch      chan []byte
	sch     chan []byte
	mu      sync.Mutex

	BroadcastHeartBeetTimer *time.Ticker
	FallBackCounter         int
	connectionPool          map[types.Address]bool
}

func (n *Node) streamStateTo(ctx context.Context, topic *pubsub.Topic) {
	// reader := bufio.NewReader(os.Stdin)
	for {
		select {
		// s, err := reader.ReadString('\n')
		// if err != nil {
		// 	panic(err)
		// }
		case d := <-n.ch: // channel from miner with blocks
			fmt.Println("recieved channel")
			if err := topic.Publish(ctx, d); err != nil {
				fmt.Println("### Publish error:", err)
			}
		case d := <-n.sch: // channel cons broadcast
			if err := topic.Publish(ctx, d); err != nil {
				fmt.Println("### Publish error:", err)
			}
		case <-n.BroadcastHeartBeetTimer.C:
			var msg = n.address.String() + "_PING"
			if err := topic.Publish(ctx, []byte(msg)); err != nil {
				fmt.Println("### Publish error:", err)
			}
		}
	}
}

func (n *Node) Alarm(data []byte) {
	n.ch <- data
}

func NewNode(h host.Host, addr types.Address) *Node {
	node := &Node{}
	node.h = h
	node.address = addr
	node.ch = make(chan []byte)
	node.sch = make(chan []byte)
	// node.mu = sync.Mutex{}
	node.BroadcastHeartBeetTimer = time.NewTicker(10 * time.Second)
	node.FallBackCounter = 0
	//node := &Node{h, addr, make(chan []byte), ,*time.NewTicker(47 * time.Second), 0}
	return node
}

var topicName = "Cerera_topic"

func StartNode(addr string, laddr types.Address) *Node {
	fmt.Printf("addr: %s\r\nladdr:%s\r\n", addr, laddr)
	ctx := context.Background()
	// h, err := libp2p.New(
	// 	// Use the keypair we generated
	// 	libp2p.Identity(prvKey),
	// 	libp2p.ListenAddrs(sourceMultiAddr),
	// )
	hAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", addr)
	h, err := libp2p.New(libp2p.ListenAddrStrings(hAddr))
	if err != nil {
		panic(err)
	}
	defer h.Close()
	CereraNode = NewNode(h, laddr)

	fullAddr := getHostAddress(h)
	fmt.Printf("I am %s\n", fullAddr)
	fmt.Printf("Host ID is %s\n", h.ID())

	go discoverPeers(ctx, h, CereraNode)

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join(topicName)
	if err != nil {
		panic(err)
	}

	go CereraNode.streamStateTo(ctx, topic)

	sub, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}
	printMessagesFrom(ctx, sub, h.ID())

	return CereraNode
}
func getHostAddress(ha host.Host) string {
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", ha.ID()))
	addr := ha.Addrs()[0]
	return addr.Encapsulate(hostAddr).String()
}
func initDHT(ctx context.Context, h host.Host) *dht.IpfsDHT {
	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := dht.New(ctx, h)
	if err != nil {
		panic(err)
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, *peerinfo); err != nil {
				fmt.Println("Bootstrap warning:", err)
			}
		}()
	}
	wg.Wait()

	return kademliaDHT
}

func discoverPeers(ctx context.Context, h host.Host, n *Node) {
	kademliaDHT := initDHT(ctx, h)
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, topicName)

	// Look for others who have announced and attempt to connect to them
	anyConnected := false
	for !anyConnected {
		fmt.Println("Searching for peers...")
		fmt.Printf("\tfallback counter: %d\r\n", n.FallBackCounter)
		peerChan, err := routingDiscovery.FindPeers(ctx, topicName)
		if err != nil {
			panic(err)
		}
		for peer := range peerChan {
			if peer.ID == h.ID() {
				continue // No self connection
			}
			err := h.Connect(ctx, peer)
			if err != nil {
				// fmt.Printf("Failed connecting to %s, error: %s\n", peer.ID, err)
				n.FallBackCounter++
				// if n.FallBackCounter > 50 {
				// 	fmt.Printf("Max connections errors reached: %d\r\n", n.FallBackCounter)
				// 	anyConnected = true
				// 	break
				// }
			} else {
				fmt.Println("Connected to:", peer.ID)
				// n.sch <- []byte(fmt.Sprintf("%s_CONS", n.address.String()))
				anyConnected = true
			}
		}
	}
	fmt.Printf("Peer discovery complete: %d fallback attempts\n", n.FallBackCounter)
}

func printMessagesFrom(ctx context.Context, sub *pubsub.Subscription, hostID peer.ID) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			panic(err)
		}
		var msgData = string(m.Message.Data)
		if m.ReceivedFrom != hostID {
			fmt.Printf("Received message from %s: \r\n\t%s,\r\n\t%s\r\n", m.ReceivedFrom, msgData, time.Now().Format("2006-01-02 15:04:05:05.000000000"))
			if strings.Contains(msgData, "_PING") {
				newAddr := strings.Split(msgData, "_PING")[0]
				gigea.E.UpdatePeer(types.HexToAddress(newAddr))
			}
		}
	}
}
