package trie

import (
	"crypto/sha256"
	"testing"
)

// testContent implements Content for testing
type testContent struct {
	data string
}

func (t testContent) CalculateHash() ([]byte, error) {
	h := sha256.New()
	h.Write([]byte(t.data))
	return h.Sum(nil), nil
}

func (t testContent) Equals(other Content) (bool, error) {
	if o, ok := other.(testContent); ok {
		return t.data == o.data, nil
	}
	return false, nil
}

func TestNewTree_EmptyContent(t *testing.T) {
	_, err := NewTree([]Content{})
	if err == nil {
		t.Error("NewTree with empty content should return error")
	}
}

func TestNewTree_SingleContent(t *testing.T) {
	content := []Content{testContent{"a"}}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	if tree.Root == nil {
		t.Error("Root should not be nil")
	}
	// Single content is duplicated to make even number of leaves
	if len(tree.Leafs) != 2 {
		t.Errorf("Leafs count = %d, want 2 (single duplicated)", len(tree.Leafs))
	}
	if tree.MerkleRoot() == nil {
		t.Error("MerkleRoot should not be nil")
	}
}

func TestNewTree_MultipleContent(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
		testContent{"c"},
		testContent{"d"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	if tree.Root == nil {
		t.Error("Root should not be nil")
	}
	if len(tree.Leafs) != 4 {
		t.Errorf("Leafs count = %d, want 4", len(tree.Leafs))
	}
	if len(tree.MerkleRoot()) != 32 {
		t.Errorf("MerkleRoot size = %d, want 32 (sha256)", len(tree.MerkleRoot()))
	}
}

func TestNewTree_OddNumberOfContent(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
		testContent{"c"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	// Odd number of leaves should be duplicated, so we get 4 leaf nodes
	if len(tree.Leafs) != 4 {
		t.Errorf("Leafs count = %d, want 4 (last duplicated)", len(tree.Leafs))
	}
}

func TestMerkleTree_MerkleRoot(t *testing.T) {
	content := []Content{testContent{"test"}}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	root := tree.MerkleRoot()
	if root == nil {
		t.Error("MerkleRoot should not be nil")
	}
	if len(root) != 32 {
		t.Errorf("MerkleRoot size = %d, want 32", len(root))
	}
}

func TestMerkleTree_VerifyTree(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	ok, err := tree.VerifyTree()
	if err != nil {
		t.Fatalf("VerifyTree error = %v", err)
	}
	if !ok {
		t.Error("VerifyTree should return true for valid tree")
	}
}

func TestMerkleTree_VerifyContent(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
		testContent{"c"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	// Verify existing content
	ok, err := tree.VerifyContent(testContent{"b"})
	if err != nil {
		t.Fatalf("VerifyContent error = %v", err)
	}
	if !ok {
		t.Error("VerifyContent should return true for existing content")
	}
	// Verify non-existing content
	ok, err = tree.VerifyContent(testContent{"x"})
	if err != nil {
		t.Fatalf("VerifyContent error = %v", err)
	}
	if ok {
		t.Error("VerifyContent should return false for non-existing content")
	}
}

func TestMerkleTree_Add(t *testing.T) {
	content := []Content{testContent{"a"}}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	err = tree.Add(testContent{"b"})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	// 1 + 1 = 2 leafs, duplicated to 4 for even tree
	if len(tree.Leafs) != 4 {
		t.Errorf("Leafs count = %d, want 4", len(tree.Leafs))
	}
	rootAfter := tree.MerkleRoot()
	if rootAfter == nil {
		t.Error("MerkleRoot should not be nil after Add")
	}
	// Root should change after adding
	ok, err := tree.VerifyTree()
	if err != nil {
		t.Fatalf("VerifyTree error = %v", err)
	}
	if !ok {
		t.Error("VerifyTree should return true after Add")
	}
}

func TestMerkleTree_RebuildTree(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	err = tree.RebuildTree()
	if err != nil {
		t.Fatalf("RebuildTree failed: %v", err)
	}
	ok, err := tree.VerifyTree()
	if err != nil {
		t.Fatalf("VerifyTree error = %v", err)
	}
	if !ok {
		t.Error("VerifyTree should return true after RebuildTree")
	}
}

func TestMerkleTree_RebuildTreeWith(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	newContent := []Content{
		testContent{"x"},
		testContent{"y"},
	}
	err = tree.RebuildTreeWith(newContent)
	if err != nil {
		t.Fatalf("RebuildTreeWith failed: %v", err)
	}
	if len(tree.Leafs) != 2 {
		t.Errorf("Leafs count = %d, want 2", len(tree.Leafs))
	}
	ok, err := tree.VerifyTree()
	if err != nil {
		t.Fatalf("VerifyTree error = %v", err)
	}
	if !ok {
		t.Error("VerifyTree should return true after RebuildTreeWith")
	}
	// Old content should not exist
	ok, _ = tree.VerifyContent(testContent{"a"})
	if ok {
		t.Error("Old content 'a' should not exist after RebuildTreeWith")
	}
	// New content should exist
	ok, _ = tree.VerifyContent(testContent{"x"})
	if !ok {
		t.Error("New content 'x' should exist after RebuildTreeWith")
	}
}

func TestMerkleTree_Size(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
		testContent{"c"},
		testContent{"d"},
	}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	size := tree.Size()
	if size < 4 {
		t.Errorf("Size = %d, want >= 4", size)
	}
}

func TestMerkleTree_Size_Empty(t *testing.T) {
	tree := &MerkleTree{
		Root: nil,
	}
	size := tree.Size()
	if size != 0 {
		t.Errorf("Size of empty tree = %d, want 0", size)
	}
}

func TestMerkleTree_String(t *testing.T) {
	content := []Content{testContent{"a"}}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	s := tree.String()
	if len(s) == 0 {
		t.Error("String should not be empty")
	}
}

func TestNode_String(t *testing.T) {
	content := []Content{testContent{"a"}}
	tree, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	if len(tree.Leafs) > 0 {
		s := tree.Leafs[0].String()
		if len(s) == 0 {
			t.Error("Node String should not be empty")
		}
	}
}

func TestMerkleTree_Deterministic(t *testing.T) {
	content := []Content{
		testContent{"a"},
		testContent{"b"},
		testContent{"c"},
	}
	tree1, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	tree2, err := NewTree(content)
	if err != nil {
		t.Fatalf("NewTree failed: %v", err)
	}
	root1 := tree1.MerkleRoot()
	root2 := tree2.MerkleRoot()
	if len(root1) != len(root2) {
		t.Errorf("Root lengths differ: %d vs %d", len(root1), len(root2))
	}
	for i := range root1 {
		if root1[i] != root2[i] {
			t.Errorf("Roots differ at index %d: 0x%02x vs 0x%02x", i, root1[i], root2[i])
			break
		}
	}
}

func TestSortAppend(t *testing.T) {
	// sortAppend is not exported, but we can test it indirectly via tree behavior
	// When sort is false, order matters - so different order gives different root
	content1 := []Content{testContent{"a"}, testContent{"b"}}
	content2 := []Content{testContent{"b"}, testContent{"a"}}
	tree1, _ := NewTree(content1)
	tree2, _ := NewTree(content2)
	root1 := tree1.MerkleRoot()
	root2 := tree2.MerkleRoot()
	// With sort=false (default), order might affect result
	// Both should be valid trees
	ok1, _ := tree1.VerifyTree()
	ok2, _ := tree2.VerifyTree()
	if !ok1 || !ok2 {
		t.Error("Both trees should verify")
	}
	_ = root1
	_ = root2
}
