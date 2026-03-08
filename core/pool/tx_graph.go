package pool

import (
	"math/big"
	"sort"
	"strconv"
	"sync"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// nonceKey returns a compact string key for the (sender, nonce) pair.
// Used as secondary index to locate the unique predecessor/successor of a tx.
func nonceKey(addr types.Address, nonce uint64) string {
	return addr.Hex() + ":" + strconv.FormatUint(nonce, 10)
}

// txNode represents a single transaction inside the dependency graph.
// It tracks parent/child links along the sender's nonce sequence and caches
// the CPFP-aggregated metrics of the whole ancestor chain for O(1) ordering.
type txNode struct {
	tx       *types.GTransaction
	parent   *txNode                  // predecessor: same sender, nonce-1; nil if no in-pool parent
	children map[common.Hash]*txNode  // successors: same sender, nonce+1, …

	// Ancestor-chain aggregates (self + all in-pool ancestors).
	// Recomputed by propagateAncestors whenever the graph topology changes.
	ancestorFee *big.Int // ΣCost()  of ancestor chain including self
	ancestorGas uint64   // ΣGas()   of ancestor chain including self
}

// TxGraph maintains the "parent-child" dependency graph of mempool transactions.
//
// # Dependency rule
//
// Transaction B directly depends on transaction A when:
//
//	B.From() == A.From()  &&  B.Nonce() == A.Nonce()+1
//
// In the account+nonce model this is the only form of dependency:
// tx with nonce N cannot be mined before tx with nonce N-1 from the same sender.
//
// # Child Pays For Parent (CPFP)
//
// If transaction A has a low fee and is "stuck" in the pool, its recipient can
// create transaction B (spending from A's output) with a very high fee.
// The miner cannot include B without also including A, but the combined package
// (A+B) has a better effective fee rate than A alone:
//
//	effectiveRate(A→B) = (fee_A + fee_B) / (gas_A + gas_B)
//
// GetMiningPackage returns transactions pre-sorted by this rate, with ancestors
// always emitted before their descendants.
//
// # Thread-safety
//
// TxGraph has its own RWMutex, independent of the pool's mu.
// Lock-ordering when both locks must be held: pool.mu → graph.mu (never reverse).
type TxGraph struct {
	mu       sync.RWMutex
	nodes    map[common.Hash]*txNode // primary index:   tx hash  → node
	nonceIdx map[string]common.Hash  // secondary index: nonceKey → tx hash
}

// NewTxGraph constructs an empty TxGraph.
func NewTxGraph() *TxGraph {
	return &TxGraph{
		nodes:    make(map[common.Hash]*txNode),
		nonceIdx: make(map[string]common.Hash),
	}
}

// AddTx inserts tx into the graph, wires parent/child links based on nonce
// adjacency, and propagates updated ancestor metrics to all affected descendants.
//
// Transactions with a zero From() address (unsigned) are stored as isolated
// roots with no dependency links.
func (g *TxGraph) AddTx(tx *types.GTransaction) {
	g.mu.Lock()
	defer g.mu.Unlock()

	hash := tx.Hash()
	if _, exists := g.nodes[hash]; exists {
		return
	}

	addr := tx.From()
	nonce := tx.Nonce()
	node := &txNode{
		tx:          tx,
		children:    make(map[common.Hash]*txNode),
		ancestorFee: new(big.Int).Set(tx.Cost()),
		ancestorGas: tx.Gas(),
	}

	zeroAddr := (types.Address{})

	// Wire parent link: look up the in-pool tx from same sender with nonce-1.
	if nonce > 0 && addr != zeroAddr {
		if parentHash, ok := g.nonceIdx[nonceKey(addr, nonce-1)]; ok {
			if parentNode, ok := g.nodes[parentHash]; ok {
				node.parent = parentNode
				parentNode.children[hash] = node
				node.ancestorFee = new(big.Int).Add(parentNode.ancestorFee, tx.Cost())
				node.ancestorGas = parentNode.ancestorGas + tx.Gas()
			}
		}
	}

	// Register before wiring child so collectChain finds this node.
	g.nodes[hash] = node
	g.nonceIdx[nonceKey(addr, nonce)] = hash

	// Wire child link: if a tx with nonce+1 arrived earlier (out-of-order
	// insertion), adopt it and propagate ancestor metrics downward.
	if addr != zeroAddr {
		if childHash, ok := g.nonceIdx[nonceKey(addr, nonce+1)]; ok {
			if childNode, ok := g.nodes[childHash]; ok && childNode.parent == nil {
				childNode.parent = node
				node.children[childHash] = childNode
				g.propagateAncestors(childNode)
			}
		}
	}
}

// RemoveTx removes a transaction from the graph.
// Its children are promoted to its parent (grandparent adoption), and all
// affected descendants have their ancestor metrics recomputed.
func (g *TxGraph) RemoveTx(hash common.Hash) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, ok := g.nodes[hash]
	if !ok {
		return
	}

	addr := node.tx.From()
	nonce := node.tx.Nonce()

	// Detach self from parent's children map.
	if node.parent != nil {
		delete(node.parent.children, hash)
	}

	// Promote each child to grandparent (or to root if no grandparent).
	for childHash, child := range node.children {
		if node.parent != nil {
			child.parent = node.parent
			node.parent.children[childHash] = child
		} else {
			child.parent = nil
		}
		g.propagateAncestors(child)
	}

	delete(g.nodes, hash)
	delete(g.nonceIdx, nonceKey(addr, nonce))
}

// propagateAncestors recomputes ancestorFee / ancestorGas for n and all of its
// descendants. Must be called with g.mu held (write lock).
func (g *TxGraph) propagateAncestors(n *txNode) {
	if n.parent != nil {
		n.ancestorFee = new(big.Int).Add(n.parent.ancestorFee, n.tx.Cost())
		n.ancestorGas = n.parent.ancestorGas + n.tx.Gas()
	} else {
		n.ancestorFee = new(big.Int).Set(n.tx.Cost())
		n.ancestorGas = n.tx.Gas()
	}
	for _, child := range n.children {
		g.propagateAncestors(child)
	}
}

// collectChain walks the nonce-linked chain from node downward via nonceIdx,
// returning all transactions in ascending-nonce (dependency-safe) order.
// Must be called with g.mu held (at least read lock).
func (g *TxGraph) collectChain(node *txNode) []*types.GTransaction {
	var chain []*types.GTransaction
	cur := node
	for cur != nil {
		chain = append(chain, cur.tx)
		addr := cur.tx.From()
		childHash, ok := g.nonceIdx[nonceKey(addr, cur.tx.Nonce()+1)]
		if !ok {
			break
		}
		cur = g.nodes[childHash]
	}
	return chain
}

// cpfpPackage is a miner-selectable unit: one or more consecutive transactions
// from the same sender, ranked by effective fee rate as a group.
type cpfpPackage struct {
	txs      []*types.GTransaction
	totalFee *big.Int // ΣCost()  across all txs in the chain
	totalGas uint64   // ΣGas()   across all txs in the chain
}

// effectiveFeeRate returns totalFee / totalGas as a *big.Rat for exact ordering.
// A higher value means this package offers more fee per unit of block gas.
func (p *cpfpPackage) effectiveFeeRate() *big.Rat {
	if p.totalGas == 0 {
		return new(big.Rat)
	}
	return new(big.Rat).SetFrac(
		new(big.Int).Set(p.totalFee),
		new(big.Int).SetUint64(p.totalGas),
	)
}

// GetMiningPackage returns up to maxTxs transactions in CPFP-aware,
// dependency-safe order (ancestors before descendants).
//
// Algorithm:
//  1. Walk every root node (no in-pool parent) to collect its full nonce chain.
//  2. Each chain becomes a cpfpPackage; its effective fee rate = ΣfeeChain / ΣgasChain.
//     Because the sum includes the high-fee child, a cheap parent whose child
//     pays generously gets a boosted rate — the CPFP effect.
//  3. Packages are sorted by effective fee rate descending.
//  4. Transactions are emitted package-by-package until maxTxs is reached.
//     Within a package the root (lowest nonce) is always emitted first.
func (g *TxGraph) GetMiningPackage(maxTxs int) []*types.GTransaction {
	// Phase 1: collect packages under RLock (minimise lock hold time).
	// Sort and emit phases run after the lock is released to avoid blocking
	// AddTx/RemoveTx writers for the full O(n log n) duration.
	packages := g.collectPackages()
	if len(packages) == 0 || maxTxs <= 0 {
		return nil
	}

	// Phase 2: pre-compute effective fee rate once per package (no lock needed;
	// cpfpPackage is a local snapshot). This avoids allocating big.Rat twice per
	// comparison inside sort, which caused heavy GC pressure under large pools.
	type ratedPkg struct {
		pkg  *cpfpPackage
		rate *big.Rat
	}
	rated := make([]ratedPkg, len(packages))
	for i, pkg := range packages {
		rated[i] = ratedPkg{pkg: pkg, rate: pkg.effectiveFeeRate()}
	}

	// Phase 3: sort highest effective fee rate first (no lock held).
	sort.Slice(rated, func(i, j int) bool {
		return rated[i].rate.Cmp(rated[j].rate) > 0
	})

	// Phase 4: emit transactions greedily up to maxTxs.
	result := make([]*types.GTransaction, 0, maxTxs)
	for _, r := range rated {
		for _, tx := range r.pkg.txs {
			if len(result) >= maxTxs {
				return result
			}
			result = append(result, tx)
		}
	}
	return result
}

// collectPackages builds the list of cpfpPackages from root nodes under RLock.
// Keeping this as a separate method limits the critical section to the O(n)
// graph traversal only; sorting runs outside the lock.
func (g *TxGraph) collectPackages() []*cpfpPackage {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.nodes) == 0 {
		return nil
	}

	packages := make([]*cpfpPackage, 0, len(g.nodes))
	for _, node := range g.nodes {
		if node.parent != nil {
			continue // non-root: covered by its root's package
		}
		chain := g.collectChain(node)
		if len(chain) == 0 {
			continue
		}
		// The last node's pre-computed ancestorFee/ancestorGas represents the
		// entire chain sum — O(1) instead of iterating the chain again.
		last := g.nodes[chain[len(chain)-1].Hash()]
		var totalFee *big.Int
		var totalGas uint64
		if last != nil {
			totalFee = new(big.Int).Set(last.ancestorFee)
			totalGas = last.ancestorGas
		} else {
			// Fallback (should not happen in a consistent graph).
			totalFee = new(big.Int)
			for _, tx := range chain {
				totalFee.Add(totalFee, tx.Cost())
				totalGas += tx.Gas()
			}
		}
		packages = append(packages, &cpfpPackage{
			txs:      chain,
			totalFee: totalFee,
			totalGas: totalGas,
		})
	}
	return packages
}

// AncestorCount returns the number of in-pool ancestors for the given tx hash.
// Returns 0 if the hash is unknown or the tx has no in-pool parents.
func (g *TxGraph) AncestorCount(hash common.Hash) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[hash]
	if !ok {
		return 0
	}
	count := 0
	for cur := node.parent; cur != nil; cur = cur.parent {
		count++
	}
	return count
}

// DescendantCount returns the number of in-pool descendants for the given tx hash.
func (g *TxGraph) DescendantCount(hash common.Hash) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[hash]
	if !ok {
		return 0
	}
	return g.countDescendants(node)
}

func (g *TxGraph) countDescendants(n *txNode) int {
	count := 0
	for _, child := range n.children {
		count++
		count += g.countDescendants(child)
	}
	return count
}

// GraphSize returns the total number of transactions tracked in the graph.
func (g *TxGraph) GraphSize() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}
