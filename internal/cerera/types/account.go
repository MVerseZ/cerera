package types

import (
	"encoding/json"
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
	// bip32 data
	// MPub     *bip32.Key
	// MPriv    *bip32.Key
	Mnemonic string
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
	buf, err := json.Marshal(sa)
	if err != nil {
		panic(err)
	}
	return buf
}

func BytesToStateAccount(data []byte) StateAccount {
	sa := &StateAccount{}
	err := json.Unmarshal(data, sa)
	if err != nil {
		panic(err)
	}
	return *sa
}
