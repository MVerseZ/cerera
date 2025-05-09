package consensus

import (
	"fmt"
	"net"
	"strings"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
)

type Voter struct {
	addr                net.Addr
	cAddr               types.Address
	syncPercent         float32
	lastResponseCounter int
}

type ConsensusOld struct {
	Nodes                   []Voter
	Latest                  common.Hash
	CurrentProposalsPercent float64
	CurrentStatus           float64
}

var c *ConsensusOld

func Start() {
	c = &ConsensusOld{
		Nodes:  make([]Voter, 0),
		Latest: types.EmptyCodeRootHash,
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
		addr:                address,
		cAddr:               cAddress,
		lastResponseCounter: 0,
		syncPercent:         0.0,
	})
	return CalculateProposals(c)
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
	return []byte{0xa, 0xa, 0xa}
}

func CalculateProposals(c *ConsensusOld) float64 {
	if len(c.Nodes) < 2 {
		c.CurrentProposalsPercent = 1.0
		return 1.0
	}
	if len(c.Nodes) == 2 {
		c.CurrentProposalsPercent = 0.5
		return 0.5
	}
	var newProposalW = ((float64(len(c.Nodes)) / 2) + 1) / float64(len(c.Nodes))
	c.CurrentProposalsPercent = newProposalW
	return newProposalW
}

// func ConfirmBlock(b block.Block) bool {
// 	fmt.Printf("Confirmind block %s with percent %f\r\n", b.Hash(), c.CurrentProposalsPercent)
// 	if c.CurrentProposalsPercent == 1.0 {
// 		return true
// 	} else {
// 		return false
// 	}
// }

func PrintInfo(v bool) {
	fmt.Printf(
		"Proposal percent: %f\r\nStatus: %f\r\nLatest: %s\r\nTotal: %d\r\n",
		c.CurrentProposalsPercent,
		c.CurrentStatus,
		c.Latest,
		len(c.Nodes),
	)
	if v {
		for i := 0; i < len(c.Nodes); i++ {
			fmt.Printf("Voter - %s:%s\r\n", c.Nodes[i].addr, c.Nodes[i].cAddr)
		}
	}
}
