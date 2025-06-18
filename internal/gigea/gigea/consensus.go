package gigea

import (
	"fmt"

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

type Consensus struct {
	Nonce  uint64
	Status int
	Voters []types.Address

	Chain chan *block.Block
}

var C Consensus
var G CSP
var Status []byte

func Init(lAddr types.Address) error {
	C = Consensus{
		Nonce:  1337,
		Chain:  make(chan *block.Block),
		Voters: make([]types.Address, 0),
	}
	G = CSP{state: Follower, address: &lAddr}
	C.Voters = append(C.Voters, lAddr)
	E = Engine{}
	E.Start(lAddr)
	Status = []byte{0x0, 0x0, 0x0, 0x0, 0x0}
	return nil
}

func SetStatus(s int) {
	C.Status = s
}

func (c *Consensus) Notify(b *block.Block) { // TODO may be better solution for delegate
	fmt.Printf("Consensus status:\r\n\t%d, %s\r\n", c.Status, G.state)
	// go func() {
	C.Chain <- b
	// net.CereraNode.Alarm(b.ToBytes())
}
