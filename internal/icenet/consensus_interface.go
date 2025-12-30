package icenet

import (
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// ConsensusManager defines the interface for consensus operations
// This allows for dependency injection and easier testing
type ConsensusManager interface {
	// Voter management
	AddVoter(addr types.Address)
	GetVoters() []types.Address

	// Node management
	AddNode(address types.Address, networkAddr string)
	GetNodes() map[string]*gigea.NodeInfo
	UpdateNodeLastSeen(address types.Address)

	// Nonce management
	GetNonce() uint64
	SetNonce(nonce uint64)

	// Status management
	SetStatus(status int)
	GetConsensusInfo() map[string]interface{}
}

// GigeaConsensusManager is an adapter that implements ConsensusManager using gigea package
type GigeaConsensusManager struct{}

// NewGigeaConsensusManager creates a new GigeaConsensusManager instance
func NewGigeaConsensusManager() *GigeaConsensusManager {
	return &GigeaConsensusManager{}
}

// AddVoter adds a voter to the consensus
func (g *GigeaConsensusManager) AddVoter(addr types.Address) {
	gigea.AddVoter(addr)
}

// GetVoters returns a list of all voters
func (g *GigeaConsensusManager) GetVoters() []types.Address {
	return gigea.GetVoters()
}

// AddNode adds or updates a node in the network
func (g *GigeaConsensusManager) AddNode(address types.Address, networkAddr string) {
	gigea.AddNode(address, networkAddr)
}

// GetNodes returns a map of all known nodes
func (g *GigeaConsensusManager) GetNodes() map[string]*gigea.NodeInfo {
	return gigea.GetNodes()
}

// UpdateNodeLastSeen updates the last seen timestamp for a node
func (g *GigeaConsensusManager) UpdateNodeLastSeen(address types.Address) {
	gigea.UpdateNodeLastSeen(address)
}

// GetNonce returns the current consensus nonce
func (g *GigeaConsensusManager) GetNonce() uint64 {
	return gigea.GetNonce()
}

// SetNonce sets the consensus nonce
func (g *GigeaConsensusManager) SetNonce(nonce uint64) {
	gigea.SetNonce(nonce)
}

// SetStatus sets the consensus status
func (g *GigeaConsensusManager) SetStatus(status int) {
	gigea.SetStatus(status)
}

// GetConsensusInfo returns consensus information
func (g *GigeaConsensusManager) GetConsensusInfo() map[string]interface{} {
	return gigea.GetConsensusInfo()
}

