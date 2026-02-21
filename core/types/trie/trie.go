package trie

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"math/big"
)

type Content interface {
	CalculateHash() ([]byte, error)
	Equals(other Content) (bool, error)
}
type MerkleTree struct {
	Root         *Node
	merkleRoot   []byte
	Leafs        []*Node
	hashStrategy func() hash.Hash
	sort         bool
}
type Node struct {
	Tree   *MerkleTree
	Parent *Node
	Left   *Node
	Right  *Node
	leaf   bool
	dup    bool
	Hash   []byte
	C      Content
	sort   bool
}

func (n *Node) verifyNode(sort bool) ([]byte, error) {
	if n.leaf {
		return n.C.CalculateHash()
	}
	rightBytes, err := n.Right.verifyNode(sort)
	if err != nil {
		return nil, err
	}

	leftBytes, err := n.Left.verifyNode(sort)
	if err != nil {
		return nil, err
	}

	h := n.Tree.hashStrategy()
	if _, err := h.Write(sortAppend(sort, leftBytes, rightBytes)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func (n *Node) calculateNodeHash(sort bool) ([]byte, error) {
	if n.leaf {
		return n.C.CalculateHash()
	}

	h := n.Tree.hashStrategy()
	if _, err := h.Write(sortAppend(sort, n.Left.Hash, n.Right.Hash)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func NewTree(cs []Content) (*MerkleTree, error) {
	var defaultHashStrategy = sha256.New
	t := &MerkleTree{
		hashStrategy: defaultHashStrategy,
		sort:         false,
	}
	root, leafs, err := buildWithContent(cs, t)
	if err != nil {
		return nil, err
	}
	t.Root = root
	t.Leafs = leafs
	t.merkleRoot = root.Hash
	return t, nil
}

func (m *MerkleTree) MerkleRoot() []byte {
	return m.merkleRoot
}

func (m *MerkleTree) Add(content Content) error {
	// Calculate the hash for the new content
	hash, err := content.CalculateHash()
	if err != nil {
		return err
	}

	// Create a new leaf node
	newNode := &Node{
		Tree: m,
		Hash: hash,
		C:    content,
		leaf: true,
	}

	// Add the new leaf to the list of leaf nodes
	m.Leafs = append(m.Leafs, newNode)

	// Rebuild the Merkle Tree
	return m.RebuildTree()
}

func (m *MerkleTree) RebuildTree() error {
	var cs []Content
	for _, c := range m.Leafs {
		cs = append(cs, c.C)
	}
	root, leafs, err := buildWithContent(cs, m)
	if err != nil {
		return err
	}
	m.Root = root
	m.Leafs = leafs
	m.merkleRoot = root.Hash
	return nil
}

func (m *MerkleTree) RebuildTreeWith(cs []Content) error {
	root, leafs, err := buildWithContent(cs, m)
	if err != nil {
		return err
	}
	m.Root = root
	m.Leafs = leafs
	m.merkleRoot = root.Hash
	return nil
}

func (m *MerkleTree) VerifyTree() (bool, error) {
	calculatedMerkleRoot, err := m.Root.verifyNode(m.sort)
	if err != nil {
		return false, err
	}

	if bytes.Compare(m.merkleRoot, calculatedMerkleRoot) == 0 {
		return true, nil
	}
	return false, nil
}

func (m *MerkleTree) VerifyContent(content Content) (bool, error) {
	for _, l := range m.Leafs {
		ok, err := l.C.Equals(content)
		if err != nil {
			return false, err
		}

		if ok {
			currentParent := l.Parent
			for currentParent != nil {
				h := m.hashStrategy()
				rightBytes, err := currentParent.Right.calculateNodeHash(m.sort)
				if err != nil {
					return false, err
				}

				leftBytes, err := currentParent.Left.calculateNodeHash(m.sort)
				if err != nil {
					return false, err
				}

				if _, err := h.Write(sortAppend(m.sort, leftBytes, rightBytes)); err != nil {
					return false, err
				}
				if bytes.Compare(h.Sum(nil), currentParent.Hash) != 0 {
					return false, nil
				}
				currentParent = currentParent.Parent
			}
			return true, nil
		}
	}
	return false, nil
}

func (n *Node) String() string {
	return fmt.Sprintf("%t %t %v %s", n.leaf, n.dup, n.Hash, n.C)
}

// String returns a string representation of the tree. Only leaf nodes are included
// in the output.
func (m *MerkleTree) String() string {
	s := ""
	for _, l := range m.Leafs {
		s += fmt.Sprint(l)
		s += "\n"
	}
	return s
}

func buildWithContent(cs []Content, t *MerkleTree) (*Node, []*Node, error) {
	if len(cs) == 0 {
		return nil, nil, errors.New("error: cannot construct tree with no content")
	}
	var leafs []*Node
	for _, c := range cs {
		hash, err := c.CalculateHash()
		if err != nil {
			return nil, nil, err
		}

		leafs = append(leafs, &Node{
			Hash: hash,
			C:    c,
			leaf: true,
			Tree: t,
		})
	}
	if len(leafs)%2 == 1 {
		duplicate := &Node{
			Hash: leafs[len(leafs)-1].Hash,
			C:    leafs[len(leafs)-1].C,
			leaf: true,
			dup:  true,
			Tree: t,
		}
		leafs = append(leafs, duplicate)
	}
	root, err := buildIntermediate(leafs, t)
	if err != nil {
		return nil, nil, err
	}

	return root, leafs, nil
}

func buildIntermediate(nl []*Node, t *MerkleTree) (*Node, error) {
	var nodes []*Node
	for i := 0; i < len(nl); i += 2 {
		h := t.hashStrategy()
		var left, right int = i, i + 1
		if i+1 == len(nl) {
			right = i
		}
		chash := sortAppend(t.sort, nl[left].Hash, nl[right].Hash)
		if _, err := h.Write(chash); err != nil {
			return nil, err
		}
		n := &Node{
			Left:  nl[left],
			Right: nl[right],
			Hash:  h.Sum(nil),
			Tree:  t,
		}
		nodes = append(nodes, n)
		nl[left].Parent = n
		nl[right].Parent = n
		if len(nl) == 2 {
			return n, nil
		}
	}
	return buildIntermediate(nodes, t)
}

func sortAppend(sort bool, a, b []byte) []byte {
	if !sort {
		return append(a, b...)
	}
	var aBig, bBig big.Int
	aBig.SetBytes(a)
	bBig.SetBytes(b)
	if aBig.Cmp(&bBig) == -1 {
		return append(a, b...)
	}
	return append(b, a...)
}

func (m *MerkleTree) Size() int {
	if m.Root == nil {
		return 0
	}
	return m.Root.size()
}

func (n *Node) size() int {
	if n == nil {
		return 0
	}
	return 1 + n.Left.size() + n.Right.size()
}
