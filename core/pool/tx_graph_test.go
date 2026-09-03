package pool

import (
	"math/big"
	"testing"

	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
)

// ----------------------------------------------------------------------------
// Mock implementations for testing
// ----------------------------------------------------------------------------

type mockAddress string

func (m mockAddress) Hex() string { return string(m) }

type mockHash [32]byte

func (m mockHash) Hex() string { return "0x" + string(m[:]) }

// mockTransaction implements enough of types.GTransaction for graph tests.
type mockTransaction struct {
	hash  common.Hash
	from  types.Address
	nonce uint64
	cost  *big.Int
	gas   uint64
}

func (tx *mockTransaction) Hash() common.Hash   { return tx.hash }
func (tx *mockTransaction) From() types.Address { return tx.from }
func (tx *mockTransaction) Nonce() uint64       { return tx.nonce }
func (tx *mockTransaction) Cost() *big.Int      { return new(big.Int).Set(tx.cost) }
func (tx *mockTransaction) Gas() uint64         { return tx.gas }
func (tx *mockTransaction) To() *types.Address  { return nil }
func (tx *mockTransaction) Value() *big.Int     { return big.NewInt(0) }
func (tx *mockTransaction) Data() []byte        { return nil }

// ----------------------------------------------------------------------------
// Test Helpers
// ----------------------------------------------------------------------------

func assertEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

func assertTrue(t *testing.T, cond bool, msg string) {
	t.Helper()
	if !cond {
		t.Errorf("assertion failed: %s", msg)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestTxGraph_AddTx_Independent(t *testing.T) {
	g := NewTxGraph()
	privKey1, _ := crypto.GenerateKey()
	addr1 := crypto.PubkeyToAddress(&privKey1.PublicKey)
	privKey2, _ := crypto.GenerateKey()
	addr2 := crypto.PubkeyToAddress(&privKey2.PublicKey)

	tx1, _ := types.CreateTransaction(1, addr1, 1.0, 1000, "")
	tx2, _ := types.CreateTransaction(1, addr2, 1.0, 1000, "")

	g.AddTx(tx1)
	g.AddTx(tx2)

	assertEqual(t, g.GraphSize(), 2, "graph size")
	assertEqual(t, g.AncestorCount(tx1.Hash()), 0, "ancestor count tx1")
	assertEqual(t, g.AncestorCount(tx2.Hash()), 0, "ancestor count tx2")
	assertEqual(t, g.DescendantCount(tx1.Hash()), 0, "descendant count tx1")
	assertEqual(t, g.DescendantCount(tx2.Hash()), 0, "descendant count tx2")
}

// func TestTxGraph_AddTx_SequentialNonces(t *testing.T) {
// 	g := NewTxGraph()
// 	privKey1, _ := crypto.GenerateKey()
// 	addr1 := crypto.PubkeyToAddress(privKey1.PublicKey)

// 	tx0, _ := types.CreateTransaction(1, addr1, 1.0, 1000, "")
// 	tx1, _ := types.CreateTransaction(1, addr1, 3.0, 1000, "")
// 	tx2, _ := types.CreateTransaction(1, addr1, 2.0, 1000, "")

// 	// Add in order
// 	g.AddTx(tx0)
// 	g.AddTx(tx1)
// 	g.AddTx(tx2)

// 	// Check ancestors
// 	assertEqual(t, g.AncestorCount(tx0.Hash()), 0, "ancestor tx0")
// 	assertEqual(t, g.AncestorCount(tx1.Hash()), 1, "ancestor tx1")
// 	assertEqual(t, g.AncestorCount(tx2.Hash()), 2, "ancestor tx2")

// 	// Check descendants
// 	assertEqual(t, g.DescendantCount(tx0.Hash()), 2, "descendant tx0")
// 	assertEqual(t, g.DescendantCount(tx1.Hash()), 1, "descendant tx1")
// 	assertEqual(t, g.DescendantCount(tx2.Hash()), 0, "descendant tx2")

// 	// Verify aggregated metrics on the node
// 	node0 := g.nodes[tx0.Hash()]
// 	node1 := g.nodes[tx1.Hash()]
// 	node2 := g.nodes[tx2.Hash()]

// 	// node0: self only
// 	assertTrue(t, node0.ancestorFee.Cmp(tx0.Cost()) == 0, "node0 ancestorFee == tx0.cost")
// 	assertEqual(t, node0.ancestorGas, tx0.Gas(), "node0 ancestorGas")

// 	// node1: tx0+tx1
// 	expectedFee1 := new(big.Int).Add(tx0.Cost(), tx1.Cost())
// 	assertTrue(t, node1.ancestorFee.Cmp(expectedFee1) == 0, "node1 ancestorFee = tx0+tx1")
// 	assertEqual(t, node1.ancestorGas, tx0.Gas()+tx1.Gas(), "node1 ancestorGas")

// 	// node2: tx0+tx1+tx2
// 	expectedFee2 := new(big.Int).Add(expectedFee1, tx2.Cost())
// 	assertTrue(t, node2.ancestorFee.Cmp(expectedFee2) == 0, "node2 ancestorFee = sum all")
// 	assertEqual(t, node2.ancestorGas, tx0.Gas()+tx1.Gas()+tx2.Gas(), "node2 ancestorGas")
// }

// func TestTxGraph_AddTx_OutOfOrder(t *testing.T) {
// 	g := NewTxGraph()
// 	privKey1, _ := crypto.GenerateKey()
// 	addr := crypto.PubkeyToAddress(privKey1.PublicKey)

// 	tx0, _ := types.CreateTransaction(1, addr, 1.0, 1000, "")
// 	tx1, _ := types.CreateTransaction(21, addr, 3.0, 3000, "")
// 	tx2, _ := types.CreateTransaction(11, addr, 2.0, 1, "")

// 	// Add child first, then parent
// 	g.AddTx(tx1)
// 	g.AddTx(tx2)
// 	g.AddTx(tx0)

// 	// After adding all, links should be correct
// 	assertEqual(t, g.AncestorCount(tx0.Hash()), 0, "ancestor tx0")
// 	assertEqual(t, g.AncestorCount(tx1.Hash()), 1, "ancestor tx1")
// 	assertEqual(t, g.AncestorCount(tx2.Hash()), 2, "ancestor tx2")

// 	// Check that node1's ancestorFee includes tx0 now
// 	node1 := g.nodes[tx1.Hash()]
// 	expectedFee1 := new(big.Int).Add(tx0.Cost(), tx1.Cost())
// 	assertTrue(t, node1.ancestorFee.Cmp(expectedFee1) == 0, "node1 ancestorFee after parent added")
// }

// func TestTxGraph_AddTx_ZeroAddressNoLinking(t *testing.T) {
// 	g := NewTxGraph()
// 	zeroAddr := zeroAddress()

// 	tx0 := newTestTx(zeroAddr, 0, 100, 1000)
// 	tx1 := newTestTx(zeroAddr, 1, 150, 800)

// 	g.AddTx(tx0)
// 	g.AddTx(tx1)

// 	// Should not link because From() is zero address
// 	assertEqual(t, g.AncestorCount(tx0.Hash()), 0, "ancestor tx0")
// 	assertEqual(t, g.AncestorCount(tx1.Hash()), 0, "ancestor tx1")
// 	assertEqual(t, g.DescendantCount(tx0.Hash()), 0, "descendant tx0")
// 	assertEqual(t, g.DescendantCount(tx1.Hash()), 0, "descendant tx1")

// 	// Ensure nonceIdx is not used for zero address (key includes address.Hex() which for zero address is non-empty string but still we treat specially)
// 	// Our implementation only wires if addr != zeroAddr, so check that no parent/child set.
// 	node0 := g.nodes[tx0.Hash()]
// 	node1 := g.nodes[tx1.Hash()]
// 	assertTrue(t, node0.parent == nil, "tx0 parent nil")
// 	assertTrue(t, node1.parent == nil, "tx1 parent nil")
// 	assertEqual(t, len(node0.children), 0, "tx0 children count")
// 	assertEqual(t, len(node1.children), 0, "tx1 children count")
// }

// func TestTxGraph_AddTx_DuplicateIgnored(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xC")
// 	tx := newTestTx(addr, 0, 100, 1000)

// 	g.AddTx(tx)
// 	size1 := g.GraphSize()
// 	g.AddTx(tx) // duplicate
// 	size2 := g.GraphSize()

// 	assertEqual(t, size1, 1, "size after first add")
// 	assertEqual(t, size2, 1, "size after duplicate add")
// }

// func TestTxGraph_RemoveTx_Leaf(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xD")
// 	tx0 := newTestTx(addr, 0, 100, 1000)
// 	tx1 := newTestTx(addr, 1, 150, 800)

// 	g.AddTx(tx0)
// 	g.AddTx(tx1)

// 	g.RemoveTx(tx1.Hash())
// 	assertEqual(t, g.GraphSize(), 1, "graph size after removing leaf")
// 	assertEqual(t, g.DescendantCount(tx0.Hash()), 0, "descendant of root after removal")
// 	_, ok := g.nodes[tx1.Hash()]
// 	assertTrue(t, !ok, "tx1 node removed")
// }

// func TestTxGraph_RemoveTx_MiddleWithChildren(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xE")
// 	tx0 := newTestTx(addr, 0, 100, 1000)
// 	tx1 := newTestTx(addr, 1, 150, 800)
// 	tx2 := newTestTx(addr, 2, 200, 1200)

// 	g.AddTx(tx0)
// 	g.AddTx(tx1)
// 	g.AddTx(tx2)

// 	// Remove tx1 (nonce 1). tx2 should become child of tx0.
// 	g.RemoveTx(tx1.Hash())

// 	assertEqual(t, g.GraphSize(), 2, "graph size after removal")
// 	// Check tx2's parent is now tx0
// 	node2 := g.nodes[tx2.Hash()]
// 	assertTrue(t, node2.parent != nil, "tx2 has parent")
// 	assertTrue(t, node2.parent.tx.Hash() == tx0.Hash(), "tx2 parent is tx0")
// 	// Check tx0's children
// 	node0 := g.nodes[tx0.Hash()]
// 	assertEqual(t, len(node0.children), 1, "tx0 children count")
// 	_, hasTx2 := node0.children[tx2.Hash()]
// 	assertTrue(t, hasTx2, "tx0 children contains tx2")

// 	// Ancestor metrics should be updated: tx2's ancestorFee = tx0+tx2
// 	expectedFee2 := new(big.Int).Add(tx0.Cost(), tx2.Cost())
// 	assertTrue(t, node2.ancestorFee.Cmp(expectedFee2) == 0, "tx2 ancestorFee after promotion")
// 	assertEqual(t, node2.ancestorGas, tx0.Gas()+tx2.Gas(), "tx2 ancestorGas after promotion")
// }

// func TestTxGraph_RemoveTx_RootWithChildren(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xF")
// 	tx0 := newTestTx(addr, 0, 100, 1000)
// 	tx1 := newTestTx(addr, 1, 150, 800)

// 	g.AddTx(tx0)
// 	g.AddTx(tx1)

// 	// Remove root tx0. tx1 should become root (parent nil)
// 	g.RemoveTx(tx0.Hash())

// 	assertEqual(t, g.GraphSize(), 1, "graph size after removal")
// 	node1 := g.nodes[tx1.Hash()]
// 	assertTrue(t, node1.parent == nil, "tx1 parent nil")
// 	assertEqual(t, node1.ancestorFee, tx1.Cost(), "tx1 ancestorFee after promotion")
// 	assertEqual(t, node1.ancestorGas, tx1.Gas(), "tx1 ancestorGas after promotion")
// }

// func TestTxGraph_RemoveTx_NonExistent(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xG")
// 	tx := newTestTx(addr, 0, 100, 1000)
// 	g.AddTx(tx)

// 	hash := common.BytesToHash([]byte("nonexistent"))
// 	g.RemoveTx(hash) // should not panic
// 	assertEqual(t, g.GraphSize(), 1, "size unchanged")
// }

// func TestTxGraph_CollectPackages_SingleChain(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xH")
// 	tx0 := newTestTx(addr, 0, 100, 1000)
// 	tx1 := newTestTx(addr, 1, 150, 800)
// 	tx2 := newTestTx(addr, 2, 200, 1200)

// 	g.AddTx(tx0)
// 	g.AddTx(tx1)
// 	g.AddTx(tx2)

// 	packages := g.collectPackages()
// 	assertEqual(t, len(packages), 1, "one package from root")
// 	pkg := packages[0]
// 	assertEqual(t, len(pkg.txs), 3, "package contains 3 txs")
// 	assertEqual(t, pkg.txs[0].Hash(), tx0.Hash(), "first tx is tx0")
// 	assertEqual(t, pkg.txs[1].Hash(), tx1.Hash(), "second tx is tx1")
// 	assertEqual(t, pkg.txs[2].Hash(), tx2.Hash(), "third tx is tx2")

// 	expectedTotalFee := new(big.Int).Add(tx0.Cost(), tx1.Cost())
// 	expectedTotalFee.Add(expectedTotalFee, tx2.Cost())
// 	assertTrue(t, pkg.totalFee.Cmp(expectedTotalFee) == 0, "totalFee sum")
// 	assertEqual(t, pkg.totalGas, tx0.Gas()+tx1.Gas()+tx2.Gas(), "totalGas sum")
// }

// func TestTxGraph_CollectPackages_MultipleRoots(t *testing.T) {
// 	g := NewTxGraph()
// 	addrA := address("0xA")
// 	addrB := address("0xB")

// 	txA0 := newTestTx(addrA, 0, 100, 1000)
// 	txA1 := newTestTx(addrA, 1, 150, 800)
// 	txB0 := newTestTx(addrB, 0, 500, 2000)

// 	g.AddTx(txA0)
// 	g.AddTx(txA1)
// 	g.AddTx(txB0)

// 	packages := g.collectPackages()
// 	assertEqual(t, len(packages), 2, "two packages")

// 	// Sort packages by root nonce? No, order not guaranteed; we just check contents.
// 	// Verify each package contains correct txs.
// 	foundA := false
// 	foundB := false
// 	for _, pkg := range packages {
// 		if len(pkg.txs) == 2 && pkg.txs[0].From() == addrA {
// 			foundA = true
// 			assertEqual(t, pkg.txs[1].Nonce(), uint64(1), "A chain includes nonce1")
// 		} else if len(pkg.txs) == 1 && pkg.txs[0].From() == addrB {
// 			foundB = true
// 		}
// 	}
// 	assertTrue(t, foundA, "found package for address A")
// 	assertTrue(t, foundB, "found package for address B")
// }

// func TestTxGraph_GetMiningPackage_BasicOrder(t *testing.T) {
// 	g := NewTxGraph()
// 	addrA := address("0xA")
// 	addrB := address("0xB")

// 	// Package A: low fee rate (100/1000 = 0.1)
// 	txA := newTestTx(addrA, 0, 100, 1000)
// 	// Package B: high fee rate (500/1000 = 0.5)
// 	txB := newTestTx(addrB, 0, 500, 1000)

// 	g.AddTx(txA)
// 	g.AddTx(txB)

// 	// With maxTxs=2, both should be returned, order by rate descending (B first)
// 	result := g.GetMiningPackage(2)
// 	assertEqual(t, len(result), 2, "got 2 txs")
// 	assertEqual(t, result[0].Hash(), txB.Hash(), "first tx is from B (higher rate)")
// 	assertEqual(t, result[1].Hash(), txA.Hash(), "second tx is from A")
// }

// func TestTxGraph_GetMiningPackage_CPFP_Effect(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xC")

// 	// Parent: low fee (10 sat per gas)
// 	parent := newTestTx(addr, 0, 10, 1000) // rate = 0.01
// 	// Child: very high fee (1000 sat per gas)
// 	child := newTestTx(addr, 1, 1000, 1000) // rate = 1.0

// 	g.AddTx(parent)
// 	g.AddTx(child)

// 	// Without CPFP, parent alone would be low. But combined package rate:
// 	// totalFee = 1010, totalGas = 2000 => 0.505, which should be higher than many.
// 	// Add a competing independent tx with fee 500 on 1000 gas (0.5)
// 	competing := newTestTx(address("0xD"), 0, 500, 1000) // rate 0.5
// 	g.AddTx(competing)

// 	result := g.GetMiningPackage(3)
// 	assertEqual(t, len(result), 3, "all txs fit")

// 	// The CPFP package (parent+child) should come before competing because 0.505 > 0.5
// 	assertEqual(t, result[0].Hash(), parent.Hash(), "first is parent")
// 	assertEqual(t, result[1].Hash(), child.Hash(), "second is child")
// 	assertEqual(t, result[2].Hash(), competing.Hash(), "third is competing")
// }

// func TestTxGraph_GetMiningPackage_MaxTxsLimit(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xE")
// 	// Create chain of 5 transactions
// 	var txs []*mockTransaction
// 	for i := 0; i < 5; i++ {
// 		tx := newTestTx(addr, uint64(i), int64(100+i*10), 1000)
// 		txs = append(txs, tx)
// 		g.AddTx(tx)
// 	}

// 	// Request maxTxs=3
// 	result := g.GetMiningPackage(3)
// 	assertEqual(t, len(result), 3, "returned 3 txs")
// 	// Should return the first 3 from the chain (lowest nonces because only one package)
// 	assertEqual(t, result[0].Hash(), txs[0].Hash(), "first tx nonce0")
// 	assertEqual(t, result[1].Hash(), txs[1].Hash(), "second tx nonce1")
// 	assertEqual(t, result[2].Hash(), txs[2].Hash(), "third tx nonce2")
// }

// func TestTxGraph_GetMiningPackage_EmptyGraph(t *testing.T) {
// 	g := NewTxGraph()
// 	result := g.GetMiningPackage(10)
// 	assertEqual(t, len(result), 0, "empty result")
// }

// func TestTxGraph_GetMiningPackage_MaxTxsZero(t *testing.T) {
// 	g := NewTxGraph()
// 	addr := address("0xF")
// 	tx := newTestTx(addr, 0, 100, 1000)
// 	g.AddTx(tx)
// 	result := g.GetMiningPackage(0)
// 	assertEqual(t, len(result), 0, "maxTxs=0 returns empty")
// }

// func TestTxGraph_GetMiningPackage_PackageRatePrecomputed(t *testing.T) {
// 	// Ensure that the sorting uses the effective rate of the whole package,
// 	// not the root's individual rate.
// 	g := NewTxGraph()
// 	addr1 := address("0xG")
// 	addr2 := address("0xH")

// 	// Package from addr1: root low fee, child high fee -> combined moderate.
// 	parent1 := newTestTx(addr1, 0, 1, 1000)   // 0.001
// 	child1 := newTestTx(addr1, 1, 1000, 1000) // 1.0, combined = 1001/2000=0.5005
// 	g.AddTx(parent1)
// 	g.AddTx(child1)

// 	// Package from addr2: single tx with fee 500 on 1000 gas (0.5)
// 	single := newTestTx(addr2, 0, 500, 1000) // 0.5
// 	g.AddTx(single)

// 	result := g.GetMiningPackage(3)
// 	// The CPFP package (0.5005) should come before single (0.5)
// 	assertEqual(t, result[0].Hash(), parent1.Hash(), "first is parent of CPFP package")
// 	assertEqual(t, result[1].Hash(), child1.Hash(), "second is child")
// 	assertEqual(t, result[2].Hash(), single.Hash(), "third is single tx")
// }

// func TestTxGraph_AncestorCount_UnknownHash(t *testing.T) {
// 	g := NewTxGraph()
// 	unknown := common.BytesToHash([]byte("unknown"))
// 	assertEqual(t, g.AncestorCount(unknown), 0, "unknown hash returns 0")
// }

// func TestTxGraph_DescendantCount_UnknownHash(t *testing.T) {
// 	g := NewTxGraph()
// 	unknown := common.BytesToHash([]byte("unknown"))
// 	assertEqual(t, g.DescendantCount(unknown), 0, "unknown hash returns 0")
// }

// func TestTxGraph_GraphSize(t *testing.T) {
// 	g := NewTxGraph()
// 	assertEqual(t, g.GraphSize(), 0, "initial size 0")
// 	addr := address("0xI")
// 	tx := newTestTx(addr, 0, 100, 1000)
// 	g.AddTx(tx)
// 	assertEqual(t, g.GraphSize(), 1, "size after add")
// 	g.RemoveTx(tx.Hash())
// 	assertEqual(t, g.GraphSize(), 0, "size after remove")
// }

// func TestTxGraph_ConcurrentReads(t *testing.T) {
// 	// Simple test to verify RLock allows concurrent reads.
// 	g := NewTxGraph()
// 	addr := address("0xJ")
// 	tx := newTestTx(addr, 0, 100, 1000)
// 	g.AddTx(tx)

// 	var wg sync.WaitGroup
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			_ = g.AncestorCount(tx.Hash())
// 			_ = g.DescendantCount(tx.Hash())
// 			_ = g.GraphSize()
// 			_ = g.GetMiningPackage(5)
// 		}()
// 	}
// 	wg.Wait()
// }

// // Benchmark simple chain insertion
// func BenchmarkTxGraph_AddChain(b *testing.B) {
// 	addr := address("bench")
// 	g := NewTxGraph()
// 	txs := make([]*mockTransaction, b.N)
// 	for i := 0; i < b.N; i++ {
// 		txs[i] = newTestTx(addr, uint64(i), 100, 1000)
// 	}
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		g.AddTx(txs[i])
// 	}
// }

// // Benchmark GetMiningPackage on a large chain
// func BenchmarkTxGraph_GetMiningPackage(b *testing.B) {
// 	addr := address("bench")
// 	g := NewTxGraph()
// 	const numTxs = 10000
// 	for i := 0; i < numTxs; i++ {
// 		g.AddTx(newTestTx(addr, uint64(i), 100, 1000))
// 	}
// 	b.ResetTimer()
// 	for i := 0; i < b.N; i++ {
// 		_ = g.GetMiningPackage(5000)
// 	}
// }
