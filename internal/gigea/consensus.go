package gigea

import (
	"fmt"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

type Voter struct {
	V uint64
}

type ConsensusState uint32

const (
	Follower ConsensusState = iota
	Candidate
	Leader
	Validator
	Miner
	Shutdown
)

func (s ConsensusState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Validator:
		return "Validator"
	case Miner:
		return "Miner"
	case Leader:
		return "Leader"
	case Shutdown:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

type CSP struct {
	address *types.Address
	state   ConsensusState
}

type NodeInfo struct {
	Address     types.Address
	NetworkAddr string
	LastSeen    int64
	IsConnected bool
}

type Consensus struct {
	Nonce  uint64
	Status int
	Voters []types.Address
	Nodes  map[string]*NodeInfo // Map of address -> NodeInfo

	Chain chan *block.Block
	term  uint64
}

var (
	C      Consensus
	G      CSP
	Status []byte
	mu     sync.RWMutex // Mutex for protecting global variables
)

func Init(lAddr types.Address) error {
	C = Consensus{
		Nonce:  1337,
		Chain:  make(chan *block.Block),
		Voters: make([]types.Address, 0),
		Nodes:  make(map[string]*NodeInfo),
		term:   0,
	}
	G = CSP{state: Follower, address: &lAddr}
	C.Voters = append(C.Voters, lAddr)
	E = Engine{Port: 32000}
	E.Start(lAddr)
	Status = []byte{0x0, 0x0, 0x0, 0x0, 0x0}
	return nil
}

func SetStatus(s int) {
	SetConsensusStatus(s)
}

func (c *Consensus) Notify(b *block.Block) { // TODO may be better solution for delegate
	mu.RLock()
	status := c.Status
	state := G.state
	mu.RUnlock()

	fmt.Printf("Consensus status:\r\n\t%d, %s\r\n", status, state.String())
	if state == Leader || state == Miner {
		mu.RLock()
		C.Chain <- b
		mu.RUnlock()
	}
	// net.CereraNode.Alarm(b.ToBytes())
}

// Safe methods for accessing global variables

// GetConsensusState safely returns the current consensus state
func GetConsensusState() ConsensusState {
	mu.RLock()
	defer mu.RUnlock()
	return G.state
}

// SetConsensusState safely sets the consensus state
func SetConsensusState(state ConsensusState) {
	mu.Lock()
	defer mu.Unlock()
	G.state = state
}

// GetConsensusStatus safely returns the current consensus status
func GetConsensusStatus() int {
	mu.RLock()
	defer mu.RUnlock()
	return C.Status
}

// SetConsensusStatus safely sets the consensus status
func SetConsensusStatus(status int) {
	mu.Lock()
	defer mu.Unlock()
	C.Status = status
}

// AddVoter safely adds a voter to the consensus
func AddVoter(addr types.Address) {
	mu.Lock()
	defer mu.Unlock()
	// Check if the voter already exists in the list
	for _, voter := range C.Voters {
		if voter == addr {
			// Voter already exists, do not add again
			return
		}
	}
	C.Voters = append(C.Voters, addr)
	for _, voter := range C.Voters {
		fmt.Printf("\t\tVoter: %s\r\n", voter.String())
	}
	fmt.Printf("\tVoters: %d\r\n", len(C.Voters))
}

// GetVoters safely returns a copy of the voters list
func GetVoters() []types.Address {
	mu.RLock()
	defer mu.RUnlock()
	voters := make([]types.Address, len(C.Voters))
	copy(voters, C.Voters)
	return voters
}

// GetConsensusInfo safely returns consensus information
func GetConsensusInfo() map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()
	return map[string]interface{}{
		"status":  C.Status,
		"address": G.address.String(),
		"voters":  len(C.Voters),
		"nodes":   len(C.Nodes),
		"nonce":   C.Nonce,
	}
}

// Node management methods

// AddNode safely adds or updates a node in the network
func AddNode(address types.Address, networkAddr string) {
	mu.Lock()
	defer mu.Unlock()
	addrStr := address.String()
	C.Nodes[addrStr] = &NodeInfo{
		Address:     address,
		NetworkAddr: networkAddr,
		LastSeen:    time.Now().Unix(),
		IsConnected: true,
	}
}

// GetNodes safely returns a copy of all known nodes
func GetNodes() map[string]*NodeInfo {
	mu.RLock()
	defer mu.RUnlock()
	nodes := make(map[string]*NodeInfo)
	for addr, node := range C.Nodes {
		nodes[addr] = &NodeInfo{
			Address:     node.Address,
			NetworkAddr: node.NetworkAddr,
			LastSeen:    node.LastSeen,
			IsConnected: node.IsConnected,
		}
	}
	return nodes
}

// UpdateNodeLastSeen updates the last seen timestamp for a node
func UpdateNodeLastSeen(address types.Address) {
	mu.Lock()
	defer mu.Unlock()
	addrStr := address.String()
	if node, exists := C.Nodes[addrStr]; exists {
		node.LastSeen = time.Now().Unix()
		node.IsConnected = true
	}
}

// RemoveNode safely removes a node from the network
func RemoveNode(address types.Address) {
	mu.Lock()
	defer mu.Unlock()
	addrStr := address.String()
	if node, exists := C.Nodes[addrStr]; exists {
		node.IsConnected = false
	}
}
