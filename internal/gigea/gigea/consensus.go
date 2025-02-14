package gigea

import "github.com/cerera/internal/cerera/types"

type Consensus struct {
	Nonce  uint64
	Status int
}

var C Consensus
var E Engine
var Status []byte

func Init(lAddr types.Address) {
	C = Consensus{
		Nonce: 1337,
	}
	E = Engine{}
	E.Start(lAddr)
	Status = []byte{0x0, 0x0, 0x0, 0x0, 0x0}
}

func SetStatus(s int) {
	C.Status = s
}
