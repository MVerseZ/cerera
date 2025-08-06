package gigea

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/message"
	"github.com/cerera/internal/cerera/types"
)

// NetworkMessage represents a network message for consensus
type NetworkMessage struct {
	Type      string          `json:"type"`
	From      types.Address   `json:"from"`
	To        types.Address   `json:"to,omitempty"` // Empty for broadcast
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature []byte          `json:"signature,omitempty"`
}

// MessageType represents different types of network messages
type MessageType string

const (
	// PBFT Message Types
	MsgTypePrePrepare MessageType = "pre_prepare"
	MsgTypePrepare    MessageType = "prepare"
	MsgTypeCommit     MessageType = "commit"
	MsgTypeViewChange MessageType = "view_change"
	MsgTypeNewView    MessageType = "new_view"

	// Raft Message Types
	MsgTypeRequestVote       MessageType = "request_vote"
	MsgTypeRequestVoteResp   MessageType = "request_vote_resp"
	MsgTypeAppendEntries     MessageType = "append_entries"
	MsgTypeAppendEntriesResp MessageType = "append_entries_resp"

	// General Message Types
	MsgTypePing      MessageType = "ping"
	MsgTypePong      MessageType = "pong"
	MsgTypeJoin      MessageType = "join"
	MsgTypeLeave     MessageType = "leave"
	MsgTypeHeartbeat MessageType = "heartbeat"
)

// Peer represents a network peer
type Peer struct {
	Address     types.Address
	Conn        net.Conn
	LastSeen    time.Time
	IsConnected bool
	mu          sync.RWMutex
}

// NetworkManager manages network communication for consensus
type NetworkManager struct {
	// Configuration
	NodeID     types.Address
	ListenAddr string
	Port       int

	// Network state
	Listener   net.Listener
	Peers      map[types.Address]*Peer
	PeersMutex sync.RWMutex

	// Message channels
	IncomingMsgChan chan *NetworkMessage
	OutgoingMsgChan chan *NetworkMessage

	// Consensus callbacks
	OnPrePrepare        func(*message.PrePrepare)
	OnPrepare           func(*message.Prepare)
	OnCommit            func(*message.Commit)
	OnViewChange        func(*message.ViewChange)
	OnRequestVote       func(*RequestVoteRequest)
	OnRequestVoteResp   func(*RequestVoteResponse)
	OnAppendEntries     func(*AppendEntriesRequest)
	OnAppendEntriesResp func(*AppendEntriesResponse)

	// State
	IsRunning bool
	mu        sync.RWMutex
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(nodeID types.Address, port int) *NetworkManager {
	fmt.Printf("Creating network manager for node %s on port %d\n", nodeID.Hex(), port)
	return &NetworkManager{
		NodeID:          nodeID,
		ListenAddr:      fmt.Sprintf(":%d", port),
		Port:            port,
		Peers:           make(map[types.Address]*Peer),
		IncomingMsgChan: make(chan *NetworkMessage, 1000),
		OutgoingMsgChan: make(chan *NetworkMessage, 1000),
	}
}

// Start starts the network manager
func (nm *NetworkManager) Start() error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.IsRunning {
		return fmt.Errorf("network manager already running")
	}

	// Start listener
	listener, err := net.Listen("tcp", nm.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	nm.Listener = listener

	nm.IsRunning = true

	// Start goroutines
	go nm.acceptConnections()
	go nm.handleIncomingMessages()
	go nm.handleOutgoingMessages()
	go nm.heartbeatLoop()

	fmt.Printf("Network manager started on %s\n", nm.ListenAddr)
	return nil
}

// Stop stops the network manager
func (nm *NetworkManager) Stop() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.IsRunning {
		return
	}

	nm.IsRunning = false

	// Close listener
	if nm.Listener != nil {
		nm.Listener.Close()
	}

	// Close all peer connections
	nm.PeersMutex.Lock()
	for _, peer := range nm.Peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}
	nm.PeersMutex.Unlock()

	// Close channels
	close(nm.IncomingMsgChan)
	close(nm.OutgoingMsgChan)

	fmt.Println("Network manager stopped")
}

// acceptConnections accepts incoming connections
func (nm *NetworkManager) acceptConnections() {
	for nm.IsRunning {
		conn, err := nm.Listener.Accept()
		if err != nil {
			if nm.IsRunning {
				fmt.Printf("Error accepting connection: %v\n", err)
			}
			continue
		}

		go nm.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (nm *NetworkManager) handleConnection(conn net.Conn) {
	defer conn.Close()

	// reader := bufio.NewReader(conn)

	for nm.IsRunning {
		// Read message length
		lengthBytes := make([]byte, 4)
		_, err := conn.Read(lengthBytes)
		if err != nil {
			break
		}

		// Read message
		messageBytes := make([]byte, nm.bytesToInt(lengthBytes))
		_, err = conn.Read(messageBytes)
		if err != nil {
			break
		}

		// Parse message
		var msg NetworkMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			fmt.Printf("Error parsing message: %v\n", err)
			continue
		}

		// Update peer info
		nm.updatePeer(msg.From, conn)

		// Send to incoming message channel
		select {
		case nm.IncomingMsgChan <- &msg:
		default:
			fmt.Printf("Incoming message channel full, dropping message\n")
		}
	}
}

// handleIncomingMessages processes incoming messages
func (nm *NetworkManager) handleIncomingMessages() {
	for msg := range nm.IncomingMsgChan {
		if msg == nil {
			continue
		}

		// Update peer last seen
		nm.updatePeerLastSeen(msg.From)

		// Route message based on type
		nm.routeMessage(msg)
	}
}

// handleOutgoingMessages processes outgoing messages
func (nm *NetworkManager) handleOutgoingMessages() {
	for msg := range nm.OutgoingMsgChan {
		if msg == nil {
			continue
		}

		// Set timestamp and from address
		msg.Timestamp = time.Now().Unix()
		msg.From = nm.NodeID

		// Send message
		nm.sendMessage(msg)
	}
}

// routeMessage routes incoming messages to appropriate handlers
func (nm *NetworkManager) routeMessage(msg *NetworkMessage) {
	switch MessageType(msg.Type) {
	case MsgTypePrePrepare:
		var prePrepare message.PrePrepare
		if err := json.Unmarshal(msg.Payload, &prePrepare); err == nil && nm.OnPrePrepare != nil {
			nm.OnPrePrepare(&prePrepare)
		}

	case MsgTypePrepare:
		var prepare message.Prepare
		if err := json.Unmarshal(msg.Payload, &prepare); err == nil && nm.OnPrepare != nil {
			nm.OnPrepare(&prepare)
		}

	case MsgTypeCommit:
		var commit message.Commit
		if err := json.Unmarshal(msg.Payload, &commit); err == nil && nm.OnCommit != nil {
			nm.OnCommit(&commit)
		}

	case MsgTypeViewChange:
		var viewChange message.ViewChange
		if err := json.Unmarshal(msg.Payload, &viewChange); err == nil && nm.OnViewChange != nil {
			nm.OnViewChange(&viewChange)
		}

	case MsgTypeRequestVote:
		var reqVote RequestVoteRequest
		if err := json.Unmarshal(msg.Payload, &reqVote); err == nil && nm.OnRequestVote != nil {
			nm.OnRequestVote(&reqVote)
		}

	case MsgTypeRequestVoteResp:
		var reqVoteResp RequestVoteResponse
		if err := json.Unmarshal(msg.Payload, &reqVoteResp); err == nil && nm.OnRequestVoteResp != nil {
			nm.OnRequestVoteResp(&reqVoteResp)
		}

	case MsgTypeAppendEntries:
		var appendEntries AppendEntriesRequest
		if err := json.Unmarshal(msg.Payload, &appendEntries); err == nil && nm.OnAppendEntries != nil {
			nm.OnAppendEntries(&appendEntries)
		}

	case MsgTypeAppendEntriesResp:
		var appendEntriesResp AppendEntriesResponse
		if err := json.Unmarshal(msg.Payload, &appendEntriesResp); err == nil && nm.OnAppendEntriesResp != nil {
			nm.OnAppendEntriesResp(&appendEntriesResp)
		}

	case MsgTypePing:
		// Respond with pong
		nm.SendPong(msg.From)

	case MsgTypePong:
		// Update peer last seen
		nm.updatePeerLastSeen(msg.From)

	case MsgTypeJoin:
		// Handle peer join
		fmt.Printf("Peer %s joined the network\n", msg.From.Hex())

	case MsgTypeLeave:
		// Handle peer leave
		fmt.Printf("Peer %s left the network\n", msg.From.Hex())
		nm.RemovePeer(msg.From)

	default:
		fmt.Printf("Unknown message type: %s\n", msg.Type)
	}
}

// sendMessage sends a message to the network
func (nm *NetworkManager) sendMessage(msg *NetworkMessage) {
	// Serialize message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error serializing message: %v\n", err)
		return
	}

	// Add message length
	lengthBytes := nm.intToBytes(len(msgBytes))
	fullMessage := append(lengthBytes, msgBytes...)

	// Send to specific peer or broadcast
	if msg.To != (types.Address{}) {
		// Send to specific peer
		nm.sendToPeer(msg.To, fullMessage)
	} else {
		// Broadcast to all peers
		nm.broadcastToPeers(fullMessage)
	}
}

// sendToPeer sends a message to a specific peer
func (nm *NetworkManager) sendToPeer(peerAddr types.Address, message []byte) {
	nm.PeersMutex.RLock()
	peer, exists := nm.Peers[peerAddr]
	nm.PeersMutex.RUnlock()

	if !exists || !peer.IsConnected {
		// Try to connect to peer
		if err := nm.connectToPeer(peerAddr); err != nil {
			fmt.Printf("Failed to connect to peer %s: %v\n", peerAddr.Hex(), err)
			return
		}

		nm.PeersMutex.RLock()
		peer = nm.Peers[peerAddr]
		nm.PeersMutex.RUnlock()
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()

	if peer.Conn != nil {
		_, err := peer.Conn.Write(message)
		if err != nil {
			fmt.Printf("Error sending message to peer %s: %v\n", peerAddr.Hex(), err)
			peer.IsConnected = false
		}
	}
}

// broadcastToPeers broadcasts a message to all peers
func (nm *NetworkManager) broadcastToPeers(message []byte) {
	nm.PeersMutex.RLock()
	peers := make([]*Peer, 0, len(nm.Peers))
	for _, peer := range nm.Peers {
		peers = append(peers, peer)
	}
	nm.PeersMutex.RUnlock()

	for _, peer := range peers {
		peer.mu.Lock()
		if peer.Conn != nil && peer.IsConnected {
			_, err := peer.Conn.Write(message)
			if err != nil {
				fmt.Printf("Error broadcasting to peer %s: %v\n", peer.Address.Hex(), err)
				peer.IsConnected = false
			}
		}
		peer.mu.Unlock()
	}
}

// connectToPeer connects to a peer
func (nm *NetworkManager) connectToPeer(peerAddr types.Address) error {
	// For now, assume peers are on localhost with different ports
	// In a real implementation, you'd have peer discovery
	peerPort := nm.Port + int(peerAddr[0]) // Simple port calculation
	peerAddrStr := fmt.Sprintf("localhost:%d", peerPort)

	conn, err := net.DialTimeout("tcp", peerAddrStr, 5*time.Second)
	if err != nil {
		return err
	}

	peer := &Peer{
		Address:     peerAddr,
		Conn:        conn,
		LastSeen:    time.Now(),
		IsConnected: true,
	}

	nm.PeersMutex.Lock()
	nm.Peers[peerAddr] = peer
	nm.PeersMutex.Unlock()

	// Send join message
	nm.SendJoin(peerAddr)

	return nil
}

// updatePeer updates peer information
func (nm *NetworkManager) updatePeer(addr types.Address, conn net.Conn) {
	nm.PeersMutex.Lock()
	defer nm.PeersMutex.Unlock()

	peer, exists := nm.Peers[addr]
	if !exists {
		peer = &Peer{
			Address:     addr,
			LastSeen:    time.Now(),
			IsConnected: true,
		}
		nm.Peers[addr] = peer
	}

	peer.Conn = conn
	peer.LastSeen = time.Now()
	peer.IsConnected = true
}

// updatePeerLastSeen updates peer last seen time
func (nm *NetworkManager) updatePeerLastSeen(addr types.Address) {
	nm.PeersMutex.RLock()
	peer, exists := nm.Peers[addr]
	nm.PeersMutex.RUnlock()

	if exists {
		peer.mu.Lock()
		peer.LastSeen = time.Now()
		peer.mu.Unlock()
	}
}

// heartbeatLoop sends periodic heartbeats
func (nm *NetworkManager) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for nm.IsRunning {
		select {
		case <-ticker.C:
			nm.broadcastHeartbeat()
		}
	}
}

// broadcastHeartbeat broadcasts heartbeat to all peers
func (nm *NetworkManager) broadcastHeartbeat() {
	msg := &NetworkMessage{
		Type:      string(MsgTypeHeartbeat),
		Timestamp: time.Now().Unix(),
	}

	select {
	case nm.OutgoingMsgChan <- msg:
	default:
		fmt.Printf("Outgoing message channel full, dropping heartbeat\n")
	}
}

// AddPeer adds a peer to the network
func (nm *NetworkManager) AddPeer(addr types.Address) {
	nm.PeersMutex.Lock()
	defer nm.PeersMutex.Unlock()

	if _, exists := nm.Peers[addr]; !exists {
		nm.Peers[addr] = &Peer{
			Address:     addr,
			LastSeen:    time.Now(),
			IsConnected: false,
		}
	}
}

// RemovePeer removes a peer from the network
func (nm *NetworkManager) RemovePeer(addr types.Address) {
	nm.PeersMutex.Lock()
	defer nm.PeersMutex.Unlock()

	if peer, exists := nm.Peers[addr]; exists {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
		delete(nm.Peers, addr)
	}
}

// GetPeers returns all peers
func (nm *NetworkManager) GetPeers() []types.Address {
	nm.PeersMutex.RLock()
	defer nm.PeersMutex.RUnlock()

	peers := make([]types.Address, 0, len(nm.Peers))
	for _, peer := range nm.Peers {
		peers = append(peers, peer.Address)
	}

	return peers
}

// GetConnectedPeers returns connected peers
func (nm *NetworkManager) GetConnectedPeers() []types.Address {
	nm.PeersMutex.RLock()
	defer nm.PeersMutex.RUnlock()

	peers := make([]types.Address, 0)
	for _, peer := range nm.Peers {
		if peer.IsConnected {
			peers = append(peers, peer.Address)
		}
	}

	return peers
}

// SendMessage sends a message to the network
func (nm *NetworkManager) SendMessage(msgType MessageType, payload interface{}, to types.Address) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling payload: %v\n", err)
		return
	}

	msg := &NetworkMessage{
		Type:    string(msgType),
		To:      to,
		Payload: payloadBytes,
	}

	select {
	case nm.OutgoingMsgChan <- msg:
	default:
		fmt.Printf("Outgoing message channel full, dropping message\n")
	}
}

// BroadcastMessage broadcasts a message to all peers
func (nm *NetworkManager) BroadcastMessage(msgType MessageType, payload interface{}) {
	nm.SendMessage(msgType, payload, types.Address{})
}

// PBFT Message Senders
func (nm *NetworkManager) SendPrePrepare(prePrepare *message.PrePrepare) {
	nm.BroadcastMessage(MsgTypePrePrepare, prePrepare)
}

func (nm *NetworkManager) SendPrepare(prepare *message.Prepare) {
	nm.BroadcastMessage(MsgTypePrepare, prepare)
}

func (nm *NetworkManager) SendCommit(commit *message.Commit) {
	nm.BroadcastMessage(MsgTypeCommit, commit)
}

func (nm *NetworkManager) SendViewChange(viewChange *message.ViewChange) {
	nm.BroadcastMessage(MsgTypeViewChange, viewChange)
}

// Raft Message Senders
func (nm *NetworkManager) SendRequestVote(req *RequestVoteRequest, to types.Address) {
	nm.SendMessage(MsgTypeRequestVote, req, to)
}

func (nm *NetworkManager) SendRequestVoteResponse(resp *RequestVoteResponse, to types.Address) {
	nm.SendMessage(MsgTypeRequestVoteResp, resp, to)
}

func (nm *NetworkManager) SendAppendEntries(req *AppendEntriesRequest, to types.Address) {
	nm.SendMessage(MsgTypeAppendEntries, req, to)
}

func (nm *NetworkManager) SendAppendEntriesResponse(resp *AppendEntriesResponse, to types.Address) {
	nm.SendMessage(MsgTypeAppendEntriesResp, resp, to)
}

// General Message Senders
func (nm *NetworkManager) SendPing(to types.Address) {
	nm.SendMessage(MsgTypePing, nil, to)
}

func (nm *NetworkManager) SendPong(to types.Address) {
	nm.SendMessage(MsgTypePong, nil, to)
}

func (nm *NetworkManager) SendJoin(to types.Address) {
	nm.SendMessage(MsgTypeJoin, nil, to)
}

func (nm *NetworkManager) SendLeave(to types.Address) {
	nm.SendMessage(MsgTypeLeave, nil, to)
}

// Utility functions
func (nm *NetworkManager) intToBytes(n int) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(n >> 24)
	bytes[1] = byte(n >> 16)
	bytes[2] = byte(n >> 8)
	bytes[3] = byte(n)
	return bytes
}

func (nm *NetworkManager) bytesToInt(bytes []byte) int {
	return int(bytes[0])<<24 | int(bytes[1])<<16 | int(bytes[2])<<8 | int(bytes[3])
}
