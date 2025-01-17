package gigea

type Consensus struct {
	Status int
}

var C Consensus
var Status []byte

func InitStatus() {
	C = Consensus{}
	Status = []byte{0x0, 0x0, 0x0, 0x0, 0x0}
}

func SetStatus(s int) {
	C.Status = s
}
