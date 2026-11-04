package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
)

func testBlock(height int, seed string) *block.Block {
	return &block.Block{
		Head: &block.Header{Height: height, Difficulty: 2},
		Hash: common.BytesToHash([]byte(seed)),
	}
}

func TestChainFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.dat")
	first := testBlock(0, "genesis")
	second := testBlock(1, "next")

	InitChainVaultWithPath(first, path)
	if err := SaveToVaultWithPath(*second, path); err != nil {
		t.Fatal(err)
	}
	body := append(mustRead(t, path), []byte("\nnot-json\n\n")...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := SyncVaultWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[1].Hash != second.Hash {
		t.Fatalf("loaded %d", len(loaded))
	}
	if _, err := SyncVaultWithPath(filepath.Join(dir, "missing.dat")); err == nil {
		t.Fatal("expected missing file")
	}
	if err := SaveToVaultWithPath(*first, filepath.Join(dir, "missing", "chain.dat")); err == nil {
		t.Fatal("expected write error")
	}

	if err := RewriteChainFile([]*block.Block{nil, first}, path); err != nil {
		t.Fatal(err)
	}
	loaded, err = SyncVaultWithPath(path)
	if err != nil || len(loaded) != 1 {
		t.Fatalf("rewritten %d %v", len(loaded), err)
	}
	if err := RewriteChainFile(nil, ""); err == nil {
		t.Fatal("expected empty path")
	}
}

func TestDefaultChainFileHelpers(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	blockA := testBlock(0, "a")
	blockB := testBlock(1, "b")
	InitChainVault(blockA)
	if err := SaveToVault(*blockB); err != nil {
		t.Fatal(err)
	}
	size, err := GetChainSourceSize()
	if err != nil || size == 0 {
		t.Fatalf("size %d %v", size, err)
	}
	loaded, err := SyncVault()
	if err != nil || len(loaded) != 2 {
		t.Fatalf("sync %d %v", len(loaded), err)
	}

	updated := testBlock(1, "b")
	updated.Head.Difficulty = 9
	if err := UpdateVault(mustJSON(t, updated)); err != nil {
		t.Fatal(err)
	}
	loaded, err = SyncVault()
	if err != nil || len(loaded) != 2 || loaded[1].Head.Difficulty != 9 {
		t.Fatalf("updated %d %v", len(loaded), err)
	}
	if err := UpdateVault(mustJSON(t, testBlock(2, "c"))); err != nil {
		t.Fatal(err)
	}
	loaded, err = SyncVault()
	if err != nil || len(loaded) != 3 {
		t.Fatalf("appended %d %v", len(loaded), err)
	}
	if err := UpdateVault([]byte("{")); err == nil {
		t.Fatal("expected bad update")
	}

	if err := ClearVault(); err != nil {
		t.Fatal(err)
	}
	size, err = GetChainSourceSize()
	if err != nil || size != 0 {
		t.Fatalf("cleared %d %v", size, err)
	}
	if err := os.Remove("chain.dat"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetChainSourceSize(); err == nil {
		t.Fatal("expected missing file")
	}
	if err := ClearVault(); err == nil {
		t.Fatal("expected truncate error")
	}
	if err := UpdateVault(mustJSON(t, blockA)); err == nil {
		t.Fatal("expected missing vault")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustJSON(t *testing.T, b *block.Block) []byte {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
