package net

import (
	"context"
	"encoding/json"
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

// reserved for future direct streams
// const protocolID = "/cerera-p2p/1.0.0"

var CereraNode *Node

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
			fmt.Println("recieved channel")
			if err := topic.Publish(ctx, d); err != nil {
				fmt.Println("### Publish error:", err)
			}
		case d := <-n.sch: // channel cons broadcast
			if err := topic.Publish(ctx, d); err != nil {
				fmt.Println("### Publish error:", err)
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

var topicName = "Cerera_topic"

// BroadcastConsensusRequest publishes a consensus request over pubsub so other nodes
// can forward it into their consensus engine. Format: "<addr>_CONS_REQ:<operation>".
func BroadcastConsensusRequest(operation string) {
	if CereraNode == nil {
		return
	}
	msg := fmt.Sprintf("%s_CONS_REQ:%s", CereraNode.address.String(), operation)
	CereraNode.sch <- []byte(msg)
}

// BroadcastConsensusResponse publishes a consensus response/ack over pubsub.
// Format: "<addr>_CONS_RESP:<payload>".
func BroadcastConsensusResponse(payload string) {
	if CereraNode == nil {
		return
	}
	msg := fmt.Sprintf("%s_CONS_RESP:%s", CereraNode.address.String(), payload)
	CereraNode.sch <- []byte(msg)
}

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

	// Inject consensus publisher into gigea so consensus can use pubsub directly
	gigea.SetConsensusPublisher(func(msg string) {
		if CereraNode != nil {
			CereraNode.sch <- []byte(msg)
		}
	})

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

			// Bridge consensus messages from pubsub into the consensus engine
			if strings.Contains(msgData, "_CONS_REQ:") {
				parts := strings.SplitN(msgData, "_CONS_REQ:", 2)
				if len(parts) == 2 && gigea.E.ConsensusManager != nil {
					op := strings.TrimSpace(parts[1])
					gigea.E.ConsensusManager.SubmitRequest(op)
				}
			}
			if strings.Contains(msgData, "_CONS_HB:") {
				// Heartbeat message from leader with JSON payload
				parts := strings.SplitN(msgData, "_CONS_HB:", 2)
				if len(parts) == 2 && gigea.E.ConsensusManager != nil {
					payload := strings.TrimSpace(parts[1])
					var hb struct {
						Term   int64  `json:"term"`
						Leader string `json:"leader"`
					}
					if err := json.Unmarshal([]byte(payload), &hb); err == nil {
						leaderAddr := types.HexToAddress(hb.Leader)
						gigea.NotifyHeartbeat(hb.Term, leaderAddr)
					}
				}
			}
			if strings.Contains(msgData, "_CONS_RESP:") {
				// Currently no-op; reserved for future consensus response handling
			}

			// Voting: handle vote requests
			if strings.Contains(msgData, "_CONS_VOTE_REQ:") {
				parts := strings.SplitN(msgData, "_CONS_VOTE_REQ:", 2)
				if len(parts) == 2 && gigea.E.ConsensusManager != nil {
					payload := strings.TrimSpace(parts[1])
					var vr struct {
						Term      int64  `json:"term"`
						Candidate string `json:"candidate"`
					}
					if err := json.Unmarshal([]byte(payload), &vr); err == nil {
						gigea.NotifyVoteRequest(vr.Term, types.HexToAddress(vr.Candidate))
					}
				}
			}

			// Voting: handle vote responses
			if strings.Contains(msgData, "_CONS_VOTE_RESP:") {
				parts := strings.SplitN(msgData, "_CONS_VOTE_RESP:", 2)
				if len(parts) == 2 && gigea.E.ConsensusManager != nil {
					payload := strings.TrimSpace(parts[1])
					var vrs struct {
						Term    int64  `json:"term"`
						Voter   string `json:"voter"`
						Granted bool   `json:"granted"`
					}
					if err := json.Unmarshal([]byte(payload), &vrs); err == nil {
						gigea.NotifyVoteResponse(vrs.Term, types.HexToAddress(vrs.Voter), vrs.Granted)
					}
				}
			}

			// Leader announcement
			if strings.Contains(msgData, "_CONS_LEADER:") {
				parts := strings.SplitN(msgData, "_CONS_LEADER:", 2)
				if len(parts) == 2 && gigea.E.ConsensusManager != nil {
					payload := strings.TrimSpace(parts[1])
					var la struct {
						Term   int64  `json:"term"`
						Leader string `json:"leader"`
					}
					if err := json.Unmarshal([]byte(payload), &la); err == nil {
						gigea.NotifyLeaderAnnouncement(la.Term, types.HexToAddress(la.Leader))
					}
				}
			}
			// Topology change
			if strings.Contains(msgData, "_CONS_TOPOLOGY:") {
				fmt.Printf("Topology change detected: \tinfo:\n\t%v\n\tstate:%v\n", gigea.E.ConsensusManager.GetConsensusInfo(), gigea.E.ConsensusManager.GetConsensusState())
			}
		}
	}
}
