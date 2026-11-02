package gigea

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

func sampleTx(nonce uint64) types.GTransaction {
	tx := types.NewTransaction(
		nonce,
		types.HexToAddress("0x01"),
		big.NewInt(1000),
		100,
		big.NewInt(1),
		[]byte("payload"),
	)
	return *tx
}

func TestMerkleTreeEdges(t *testing.T) {
	if _, err := NewTree(nil); err == nil {
		t.Fatal("expected empty tree error")
	}
	if _, err := NewTreeWithHashStrategy(nil, sha256.New); err == nil {
		t.Fatal("expected empty tree error")
	}

	one := []types.GTransaction{coinbase.CreateCoinBaseTransation(1, types.HexToAddress("0x02"), 0)}
	tree, err := NewTree(one)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.MerkleRoot()) == 0 {
		t.Fatal("missing root")
	}
	missing := sampleTx(9)
	path, index, err := tree.GetMerklePath(one[0])
	if err != nil || len(path) == 0 || len(index) == 0 {
		t.Fatalf("single leaf path %v %v %v", path, index, err)
	}
	if _, _, err := tree.GetMerklePath(missing); err != nil {
		t.Fatal(err)
	}
	if err := tree.RebuildTree(); err != nil {
		t.Fatal(err)
	}
	if err := tree.RebuildTreeWith(nil); err == nil {
		t.Fatal("expected rebuild error")
	}
	if tree.String() == "" || tree.Leafs[0].String() == "" {
		t.Fatal("empty tree string")
	}
	tree.Root.PrintTree("")

	three := []types.GTransaction{sampleTx(1), sampleTx(2), sampleTx(3)}
	sorted, err := NewTreeWithHashStrategy(three, sha256.New)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := sorted.VerifyTree(); err != nil || !ok {
		t.Fatal(err)
	}
	if _, _, err := sorted.GetMerklePath(three[1]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sortAppend(false, []byte{2}, []byte{1}), []byte{2, 1}) {
		t.Fatal("unsorted append")
	}
	if !bytes.Equal(sortAppend(true, []byte{2}, []byte{1}), []byte{1, 2}) {
		t.Fatal("sorted append")
	}
}
