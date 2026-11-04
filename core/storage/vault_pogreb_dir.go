package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

const vaultDBSubdir = "db"

// resolvePogrebDBDir is the directory passed to pogreb.Open.
// .vault_keys stays in vaultDir (not inside pogreb) so recovery does not move or delete it.
// Legacy installs that already have main.pix in vaultDir keep using that path.
func resolvePogrebDBDir(vaultDir string) (string, error) {
	if vaultDir == "" {
		return "", fmt.Errorf("vault directory is empty")
	}
	legacyMain := filepath.Join(vaultDir, "main.pix")
	if _, err := os.Stat(legacyMain); err == nil {
		return vaultDir, nil
	}
	dbDir := filepath.Join(vaultDir, vaultDBSubdir)
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return "", fmt.Errorf("create vault db dir: %w", err)
	}
	return dbDir, nil
}
