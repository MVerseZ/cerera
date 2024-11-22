package consensus

import (
	"fmt"
	"net"
	"testing"

	"github.com/cerera/internal/cerera/types"
	"github.com/stretchr/testify/assert"
)

func TestConsensus_0_Element(t *testing.T) {
	Start()
	assert.Equal(t, ConsensusStatus(), float64(1.0))
}

func TestConsensus_1_Element(t *testing.T) {
	var cereraAddress = types.HexToAddress("0x00000000f")
	addr, err := net.ResolveTCPAddr("tcp", "1.1.1.1:1234")
	if err != nil {
		panic(err)
	}

	Start()
	Add(addr, cereraAddress)
	assert.Equal(t, ConsensusStatus(), float64(1.0))
}

func TestConsensus_More1(t *testing.T) {
	var cereraAddress1 = types.HexToAddress("0x00000000f")
	var cereraAddress2 = types.HexToAddress("0x00000000b")
	addr1, err := net.ResolveTCPAddr("tcp", "1.1.1.1:1234")
	if err != nil {
		panic(err)
	}
	addr2, err := net.ResolveTCPAddr("tcp", "1.1.1.1:1233")
	if err != nil {
		panic(err)
	}

	Start()
	Add(addr1, cereraAddress1)
	Add(addr2, cereraAddress2)
	assert.NotEqual(t, ConsensusStatus(), float64(1.0))
}

func TestStatus(t *testing.T) {
	Start()
	for i := 0; i < 7; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("1.1.1.%d:1234", i))
		if err != nil {
			panic(err)
		}
		Add(
			addr,
			types.HexToAddress(fmt.Sprintf("0x000000000%x", i)),
		)
	}

	PrintInfo(false)
}
