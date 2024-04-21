package types

import (
	"bytes"
	"encoding/gob"
	"math/big"

	"github.com/cerera/internal/cerera/common"
)

type StateAccount struct {
	Address  Address
	Balance  *big.Int
	Bloom    []byte
	CodeHash []byte
	Name     string
	Nonce    uint64
	Root     common.Hash // merkle root of the storage trie
	Status   string
	// Treasury []*coinbase.CoinBase
	Inputs     []common.Hash
	Passphrase common.Hash
}

func (sa *StateAccount) BloomUp() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] < 0xf {
		sa.Bloom[1] = tmpBloom + 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) BloomDown() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] > 0x1 {
		sa.Bloom[1] = tmpBloom - 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) Bytes() []byte {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(sa)
	if err != nil {
		panic(err)
	}
	var saBytes = append(buf.Bytes(), sa.Bloom...)
	return saBytes
}

func BytesToStateAccount(data []byte) StateAccount {
	p := StateAccount{}
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&p)
	if err != nil {
		panic(err)
	}
	return p
}
