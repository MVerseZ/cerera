package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePogrebDBDir_NewUsesSubdir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePogrebDBDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, vaultDBSubdir)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("db dir not created: %v", err)
	}
}

func TestResolvePogrebDBDir_LegacyRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.pix"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePogrebDBDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("legacy: got %q want %q", got, dir)
	}
}
