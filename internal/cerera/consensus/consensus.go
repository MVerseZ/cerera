package consensus

import (
	"fmt"
	"net"
	"strings"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

type Voter struct {
	addr  net.Addr
	cAddr types.Address
}

type Consensus struct {
	Nodes                   []Voter
	Latest                  common.Hash
	CurrentProposalsPercent float64
	CurrentStatus           float64
}

var c Consensus

func Start() {
	c = Consensus{
		Nodes:  make([]Voter, 0),
		Latest: block.CrvBlockHash(block.Genesis()), //simplify
	}
	c.CurrentStatus = 1.0
	c.CurrentProposalsPercent = CalculateProposals(c)
}

func Add(address net.Addr, cAddress types.Address) float64 {
	for _, v := range c.Nodes {
		if v.cAddr == cAddress {
			return 0xf
		}
	}

	c.Nodes = append(c.Nodes, Voter{
		addr:  address,
		cAddr: cAddress,
	})
	return CalculateProposals(c)
}

func CalculateProposals(c Consensus) float64 {
	if len(c.Nodes) < 2 {
		return 1.0
	}
	if len(c.Nodes) == 2 {
		return 0.5
	}
	var newProposalW = ((float64(len(c.Nodes)) / 2) + 1) / float64(len(c.Nodes))
	fmt.Println(newProposalW)
	return newProposalW
}

func ConsensusStatus() float64 {
	return c.CurrentProposalsPercent
}

// SPEC_CODES
func HandleConsensusRequest(netAddr net.Addr, method string, params []interface{}) interface{} {
	var m = strings.Split(method, ".")[2]
	if m == "join" {
		strCereraAddr, ok1 := params[0].(string)
		if !ok1 {
			return 0xf
		}
		return Add(netAddr, types.HexToAddress(strCereraAddr))
	}
	return 0x1
}

func ConfirmBlock(b block.Block) bool {
	if c.CurrentProposalsPercent == 1.0 {
		return true
	} else {
	}
	return false
}
