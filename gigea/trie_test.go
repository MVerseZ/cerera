package gigea

import (
	"math/big"
	"testing"

	"github.com/cerera/core/types"
	"github.com/cerera/core/common"
	"github.com/cerera/internal/coinbase"
	"golang.org/x/crypto/sha3"
)

var cbto = types.HexToAddress("0xfffffffffff00000000000000000000557D0B284521d71A7fCA1e1C3F289849989E80B0B810000000000000000000000")
var toAddr = types.HexToAddress("0xc9C5c06E29500000000000000000000557D0B284521d71A7fCA1e1C3F289849989E80B0B810000000000000000000000")
var toAddr2 = types.HexToAddress("0xc9C5c06E29500000000000000000000557D0B284521d71A7fCA1e1C3F289849989E80B0B8100000000000000000000ff")

func TestCreate(t *testing.T) {
	var cbtx = coinbase.CreateCoinBaseTransation(1, cbto)

	var content = make([]types.GTransaction, 0)
	content = append(content, cbtx)

	for ii := range 11 {
		var txVal = 10.0
		var tx = types.NewTransaction(
			uint64(ii),
			toAddr,
			types.FloatToBigInt(txVal),
			1000,
			big.NewInt(1000),
			[]byte("Message"),
		)
		content = append(content, *tx)
	}

	// var lastTx = types.NewTransaction(
	// 	1,
	// 	toAddr2,
	// 	big.NewInt(11111),
	// 	1000,
	// 	big.NewInt(1000),
	// 	[]byte("Message last"),
	// )

	trie, err := NewTreeWithHashStrategySorted(content, sha3.NewLegacyKeccak256, true)
	if err != nil {
		t.Errorf("error while create trie %s\r\n", err)
	}
	v, err := trie.VerifyTree()
	if err != nil || !v {
		t.Errorf("error while verify true %s\r\n", err)
	}

	// check first tx is coinbase
	var rootHash = common.BytesToHash(trie.merkleRoot)
	if rootHash == cbtx.Hash() {
		t.Errorf("diff root hash with coinbase tx! Expected: %s actual: %s\r\n", rootHash, cbtx.Hash())
	}

	// fmt.Println(">>>>>>>")
	// for _, v := range content {
	// 	fmt.Println(v.Hash())
	// }
	// fmt.Println(">>>>>>>")
	// trie.Root.PrintTree("--")
	// fmt.Println(">>>>>>>")

	// check existing
	aa, bb, err := trie.GetMerklePath(content[4])
	if len(aa) <= 0 && len(bb) <= 0 {
		t.Errorf("not in trie!, %s\r\n", content[4].Hash())
	}
	if err != nil {
		t.Errorf("error! %s\r\n", err)
	}

	if trie.sort != true {
		t.Error("trie is not sort!")
	}
}
