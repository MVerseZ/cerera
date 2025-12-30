package consensus

import (
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// Manager defines the interface for consensus operations
type Manager interface {
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

// GigeaManager is an adapter that implements Manager using gigea package
type GigeaManager struct{}

// NewGigeaManager creates a new GigeaManager instance
func NewGigeaManager() *GigeaManager {
	return &GigeaManager{}
}

// AddVoter adds a voter to the consensus
func (g *GigeaManager) AddVoter(addr types.Address) {
	gigea.AddVoter(addr)
}

// GetVoters returns a list of all voters
func (g *GigeaManager) GetVoters() []types.Address {
	return gigea.GetVoters()
}

// AddNode adds or updates a node in the network
func (g *GigeaManager) AddNode(address types.Address, networkAddr string) {
	gigea.AddNode(address, networkAddr)
}

// GetNodes returns a map of all known nodes
func (g *GigeaManager) GetNodes() map[string]*gigea.NodeInfo {
	return gigea.GetNodes()
}

// UpdateNodeLastSeen updates the last seen timestamp for a node
func (g *GigeaManager) UpdateNodeLastSeen(address types.Address) {
	gigea.UpdateNodeLastSeen(address)
}

// GetNonce returns the current consensus nonce
func (g *GigeaManager) GetNonce() uint64 {
	return gigea.GetNonce()
}

// SetNonce sets the consensus nonce
func (g *GigeaManager) SetNonce(nonce uint64) {
	gigea.SetNonce(nonce)
}

// SetStatus sets the consensus status
func (g *GigeaManager) SetStatus(status int) {
	gigea.SetStatus(status)
}

// GetConsensusInfo returns consensus information
func (g *GigeaManager) GetConsensusInfo() map[string]interface{} {
	return gigea.GetConsensusInfo()
}

