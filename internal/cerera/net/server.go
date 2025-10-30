package net

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/prometheus/client_golang/prometheus"
)

// reserved for future direct streams
// const protocolID = "/cerera-p2p/1.0.0"

var CereraNode *Node

// p2plog is a dedicated console logger for P2P networking
var p2plog = log.New(os.Stdout, "[p2p] ", log.LstdFlags|log.Lmicroseconds)

var (
	p2pPeerConnectsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_peer_connects_total",
		Help: "Total number of successful peer connections",
	})
	p2pPeerDiscoveryAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_peer_discovery_attempts_total",
		Help: "Total number of peer discovery attempts",
	})
	p2pDirectMessagesSentTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_direct_messages_sent_total",
		Help: "Total number of direct messages sent",
	})
	p2pDirectMessagesReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_direct_messages_received_total",
		Help: "Total number of direct messages received",
	})
	p2pPubsubMessagesReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_pubsub_messages_received_total",
		Help: "Total number of pubsub messages received",
	})
	p2pPublishErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_publish_errors_total",
		Help: "Total number of publish errors",
	})
)

func init() {
	prometheus.MustRegister(
		p2pPeerConnectsTotal,
		p2pPeerDiscoveryAttemptsTotal,
		p2pDirectMessagesSentTotal,
		p2pDirectMessagesReceivedTotal,
		p2pPubsubMessagesReceivedTotal,
		p2pPublishErrorsTotal,
	)
}

type Node struct {
	h       host.Host
	address types.Address
	ch      chan []byte
	sch     chan []byte
	// mu sync.Mutex

	BroadcastHeartBeetTimer *time.Ticker
	FallBackCounter         int
	// connectionPool map[types.Address]bool
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
			p2plog.Println("received channel")
			if err := topic.Publish(ctx, d); err != nil {
				p2plog.Printf("publish error: %v", err)
				p2pPublishErrorsTotal.Inc()
			}
		case d := <-n.sch: // channel cons broadcast
			if err := topic.Publish(ctx, d); err != nil {
				p2plog.Printf("publish error: %v", err)
				p2pPublishErrorsTotal.Inc()
			}
		case <-n.BroadcastHeartBeetTimer.C:
			// // Silence if setup consensus
			// if gigea.E.ConsensusManager != nil && gigea.E.ConsensusManager. {
			// 	var msg = n.address.String() + "_PING"
			// 	if err := topic.Publish(ctx, []byte(msg)); err != nil {
			// 		fmt.Println("### Publish error:", err)
			// 	}
			// }
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

// sendDirectMessage sends a message directly to a specific peer via libp2p stream
func (n *Node) sendDirectMessage(ctx context.Context, peerID peer.ID, message string) error {
	// Define the protocol for direct communication
	protocolID := protocol.ID("/cerera-direct/1.0.0")

	// Open a stream to the peer
	stream, err := n.h.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return fmt.Errorf("failed to open stream to peer %s: %w", peerID, err)
	}
	defer stream.Close()

	// Send the message
	_, err = stream.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message to peer %s: %w", peerID, err)
	}

	p2plog.Printf("direct message sent to peer %s: %s", peerID, message)
	p2pDirectMessagesSentTotal.Inc()
	return nil
}

// handleDirectMessage handles incoming direct messages from peers
func (n *Node) handleDirectMessage(stream network.Stream) {
	defer stream.Close()

	// Read the message
	buf := make([]byte, 4096)
	nBytes, err := stream.Read(buf)
	if err != nil {
		p2plog.Printf("error reading direct message: %v", err)
		return
	}

	message := string(buf[:nBytes])
	peerID := stream.Conn().RemotePeer()

	p2plog.Printf("direct message received from peer %s: %s", peerID, message)
	p2pDirectMessagesReceivedTotal.Inc()

	// Process the message (same logic as pubsub messages)
	if strings.Contains(message, "_CONS_JOIN") {
		p2plog.Printf("received CONS_JOIN from %s via direct message", peerID)
		// Parse CONS_JOIN message: {address}_CONS_JOIN#{network_address}
		if peerAddress, networkAddress, valid := parseConsensusJoinMessage(message); valid {
			p2plog.Printf("peer address: %s, network address: %s", peerAddress, networkAddress)
			// Add peer to consensus voters list and node list
			addr := types.HexToAddress(peerAddress)
			gigea.AddVoter(addr)
			gigea.AddNode(addr, networkAddress)
			p2plog.Printf("added peer %s to consensus voters and node list", peerAddress)

			// Broadcast updated node list to all peers
			go broadcastNodeList()
		} else {
			p2plog.Printf("invalid CONS_JOIN message format: %s", message)
		}
	}

	// Handle NODES message for node list synchronization
	if strings.Contains(message, "_NODES") {
		p2plog.Printf("received NODES from %s via direct message", peerID)
		if senderAddress, nodes, valid := parseNodeListMessage(message); valid {
			p2plog.Printf("received node list from %s: %d nodes", senderAddress, len(nodes))
			// Add all nodes to our known nodes list and make them voters
			for _, nodeInfo := range nodes {
				parts := strings.Split(nodeInfo, "#")
				if len(parts) == 2 {
					nodeAddr := parts[0]
					networkAddr := parts[1]
					addr := types.HexToAddress(nodeAddr)
					gigea.AddNode(addr, networkAddr)
					gigea.AddVoter(addr) // Make all known nodes voters
					p2plog.Printf("added node %s to known nodes and voters", nodeAddr)
				}
			}
		} else {
			p2plog.Printf("invalid NODES message format: %s", message)
		}
	}
}

var topicName = "Cerera_topic"

// parseConsensusJoinMessage parses CONS_JOIN message format: {address}_CONS_JOIN#{network_address}
func parseConsensusJoinMessage(message string) (peerAddress, networkAddress string, valid bool) {
	parts := strings.Split(message, "_CONS_JOIN#")
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}

// broadcastNodeList broadcasts the list of known nodes to all peers
func broadcastNodeList() {
	if CereraNode == nil {
		return
	}

	nodes := gigea.GetNodes()
	if len(nodes) == 0 {
		return
	}

	// Create a simple node list message
	var nodeList []string
	for addr, node := range nodes {
		if node.IsConnected {
			nodeList = append(nodeList, fmt.Sprintf("%s#%s", addr, node.NetworkAddr))
		}
	}

	if len(nodeList) > 0 {
		msg := fmt.Sprintf("%s_NODES#%s", CereraNode.address.String(), strings.Join(nodeList, ","))
		CereraNode.sch <- []byte(msg)
		p2plog.Printf("broadcasting node list: %d nodes", len(nodeList))
	}
}

// parseNodeListMessage parses NODES message format: {sender}_NODES#{node1#addr1,node2#addr2,...}
func parseNodeListMessage(message string) (senderAddress string, nodes []string, valid bool) {
	parts := strings.Split(message, "_NODES#")
	if len(parts) == 2 {
		senderAddress = parts[0]
		nodesStr := parts[1]
		if nodesStr != "" {
			nodes = strings.Split(nodesStr, ",")
		}
		return senderAddress, nodes, true
	}
	return "", nil, false
}

// printNetworkStatus prints current network status including voters and nodes
func printNetworkStatus() {
	if CereraNode == nil {
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Printf("🌐 NETWORK STATUS REPORT - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 80))

	// Get consensus info
	consensusInfo := gigea.GetConsensusInfo()
	fmt.Printf("📊 Consensus Info:\n")
	fmt.Printf("   Status: %d\n", consensusInfo["status"])
	fmt.Printf("   Address: %s\n", consensusInfo["address"])
	fmt.Printf("   Voters: %d\n", consensusInfo["voters"])
	fmt.Printf("   Nodes: %d\n", consensusInfo["nodes"])
	fmt.Printf("   Nonce: %d\n", consensusInfo["nonce"])

	// Print voters list (these are the participating nodes)
	voters := gigea.GetVoters()
	fmt.Printf("\n🗳️  Voters/Participating Nodes (%d):\n", len(voters))
	for i, voter := range voters {
		fmt.Printf("   %d. %s\n", i+1, voter.String())
	}

	// Print nodes list (all nodes in the network)
	nodes := gigea.GetNodes()
	fmt.Printf("\n🖥️  All Known Nodes (%d):\n", len(nodes))
	nodeCount := 1
	for addr, node := range nodes {
		status := "❌"
		if node.IsConnected {
			status = "✅"
		}
		lastSeen := time.Unix(node.LastSeen, 0).Format("15:04:05")
		fmt.Printf("   %d. %s %s (Last seen: %s)\n", nodeCount, status, addr, lastSeen)
		fmt.Printf("      Network: %s\n", node.NetworkAddr)
		nodeCount++
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}

// BroadcastConsensusRequest publishes a consensus request over pubsub so other nodes
// can forward it into their consensus engine. Format: "<addr>_CONS_REQ:<operation>".
func BroadcastConsensusRequest(operation string) {
	if CereraNode == nil {
		return
	}
	msg := fmt.Sprintf("%s_CONS_REQ:%s", CereraNode.address.String(), operation)
	CereraNode.sch <- []byte(msg)
}

func StartNode(addr string, laddr types.Address) *Node {
	p2plog.Printf("startup params: addr=%s, laddr=%s", addr, laddr)
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
	p2plog.Printf("node multiaddr: %s", fullAddr)
	p2plog.Printf("host id: %s", h.ID())

	// Print initial status
	p2plog.Println("starting Cerera node...")
	printNetworkStatus()

	// Set up direct message handler
	h.SetStreamHandler(protocol.ID("/cerera-direct/1.0.0"), CereraNode.handleDirectMessage)

	// Start periodic node list synchronization
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				broadcastNodeList()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start periodic status reporting
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				printNetworkStatus()
			case <-ctx.Done():
				return
			}
		}
	}()

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

	// Find the first non-localhost address
	for _, addr := range ha.Addrs() {
		addrStr := addr.String()
		// Skip localhost addresses
		if !strings.Contains(addrStr, "127.0.0.1") && !strings.Contains(addrStr, "::1") {
			return addr.Encapsulate(hostAddr).String()
		}
	}

	// Fallback to first address if no external address found
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
				p2plog.Printf("bootstrap warning: %v", err)
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
		p2plog.Println("searching for peers...")
		p2pPeerDiscoveryAttemptsTotal.Inc()
		p2plog.Printf("fallback counter: %d", n.FallBackCounter)
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
				p2plog.Printf("connected to peer: %s", peer.ID)
				p2pPeerConnectsTotal.Inc()
				networkAddr := getHostAddress(n.h)

				// Send direct message to the connected peer
				if err := n.sendDirectMessage(ctx, peer.ID, fmt.Sprintf("%s_CONS_JOIN#%s", n.address.String(), networkAddr)); err != nil {
					p2plog.Printf("failed to send direct message to peer %s: %v", peer.ID, err)
				}

				// Also broadcast through pubsub for other nodes
				n.sch <- []byte(fmt.Sprintf("%s_CONS_JOIN#%s", n.address.String(), networkAddr))
				anyConnected = true
			}
		}
	}
	p2plog.Printf("peer discovery complete: %d fallback attempts", n.FallBackCounter)
}

func printMessagesFrom(ctx context.Context, sub *pubsub.Subscription, hostID peer.ID) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			panic(err)
		}
		var msgData = string(m.Message.Data)
		if m.ReceivedFrom != hostID {
			p2pPubsubMessagesReceivedTotal.Inc()
			// fmt.Printf("Received message from %s: \r\n\t%s,\r\n\t%s\r\n", m.ReceivedFrom, msgData, time.Now().Format("2006-01-02 15:04:05:05.000000000"))
			if strings.Contains(msgData, "_PING") {
				newAddr := strings.Split(msgData, "_PING")[0]
				p2plog.Printf("received PING from %s", newAddr)
			}
			if strings.Contains(msgData, "_CONS_JOIN") {
				// fmt.Printf("Received CONS_JOIN from %s\r\n", m.ReceivedFrom)
				// Parse CONS_JOIN message: {address}_CONS_JOIN#{network_address}
				if peerAddress, networkAddress, valid := parseConsensusJoinMessage(msgData); valid {
					// fmt.Printf("Peer address: %s, Network address: %s\r\n", peerAddress, networkAddress)
					// Add peer to consensus voters list and node list
					addr := types.HexToAddress(peerAddress)
					gigea.AddVoter(addr)
					gigea.AddNode(addr, networkAddress)
					// fmt.Printf("Added peer %s to consensus voters and node list\r\n", peerAddress)

					// Broadcast updated node list to all peers
					go broadcastNodeList()
				} else {
					p2plog.Printf("invalid CONS_JOIN message format: %s", msgData)
				}
			}
			if strings.Contains(msgData, "_NODES") {
				// fmt.Printf("Received NODES from %s\r\n", m.ReceivedFrom)
				if _, nodes, valid := parseNodeListMessage(msgData); valid {
					// fmt.Printf("Received node list from %s: %d nodes\r\n", senderAddress, len(nodes))
					// Add all nodes to our known nodes list and make them voters
					for _, nodeInfo := range nodes {
						parts := strings.Split(nodeInfo, "#")
						if len(parts) == 2 {
							nodeAddr := parts[0]
							networkAddr := parts[1]
							addr := types.HexToAddress(nodeAddr)
							gigea.AddNode(addr, networkAddr)
							gigea.AddVoter(addr) // Make all known nodes voters
							// p2plog.Printf("added node %s to known nodes and voters", nodeAddr)
						}
					}
				} else {
					p2plog.Printf("invalid NODES message format: %s", msgData)
				}
			}
		}
	}
}
