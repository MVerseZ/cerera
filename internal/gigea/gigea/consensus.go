package gigea

import (
	"fmt"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/net"
	"github.com/cerera/internal/cerera/types"
)

type Consensus struct {
	Nonce  uint64
	Status int
}

var C Consensus
var E Engine
var Status []byte

func Init(lAddr types.Address) error {
	C = Consensus{
		Nonce: 1337,
	}
	E = Engine{}
	E.Start(lAddr)
	Status = []byte{0x0, 0x0, 0x0, 0x0, 0x0}
	return nil
}

func SetStatus(s int) {
	C.Status = s
}

func (c *Consensus) Notify(h common.Hash) {
	fmt.Printf("Consensus status:\r\n\t%d\r\n", c.Status)
	fmt.Printf("\t%s\r\n", h)
	net.CereraNode.Alarm([]byte(h.String()))
}

func (e *Engine) Register(a interface{}) {

}
