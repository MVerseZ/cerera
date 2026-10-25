package backup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/backup"
	"github.com/stretchr/testify/require"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "node-backup.tar.gz")

	cfg, err := config.OpenConfig(filepath.Join(srcDir, config.ConfigFileName))
	require.NoError(t, err)
	require.NoError(t, cfg.ApplyDataDir(srcDir))
	require.NoError(t, cfg.WriteConfigToFile())

	chainFile := filepath.Join(srcDir, config.ChainFileName)
	require.NoError(t, os.WriteFile(chainFile, []byte(`{"hash":"genesis"}`+"\n"), 0o644))

	vaultDir := filepath.Join(srcDir, config.VaultDirName)
	require.NoError(t, os.MkdirAll(vaultDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(vaultDir, ".vault_keys"), []byte(`{"mnemonic":"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about","passphrase":"NODE_PASS"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(vaultDir, "account.bin"), []byte("demo"), 0o600))

	nodeKey := filepath.Join(srcDir, "node.pem")
	require.NoError(t, os.WriteFile(nodeKey, []byte("test-key"), 0o600))

	manifest, err := backup.Backup(srcDir, archivePath, nodeKey)
	require.NoError(t, err)
	require.Equal(t, 1, manifest.ChainBlocks)
	require.Contains(t, manifest.Includes, "nodekey.pem")

	restored, err := backup.Restore(archivePath, dstDir, false)
	require.NoError(t, err)
	require.Equal(t, manifest.ChainBlocks, restored.ChainBlocks)

	require.FileExists(t, filepath.Join(dstDir, config.ConfigFileName))
	require.FileExists(t, filepath.Join(dstDir, config.ChainFileName))
	require.FileExists(t, filepath.Join(dstDir, config.VaultDirName, ".vault_keys"))
	require.FileExists(t, filepath.Join(dstDir, config.VaultDirName, "account.bin"))
	require.FileExists(t, filepath.Join(dstDir, "nodekey.pem"))
}

func TestRestoreRejectsExistingWithoutForce(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "node-backup.tar.gz")

	cfg, err := config.OpenConfig(filepath.Join(srcDir, config.ConfigFileName))
	require.NoError(t, err)
	require.NoError(t, cfg.ApplyDataDir(srcDir))
	require.NoError(t, cfg.WriteConfigToFile())
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, config.ChainFileName), []byte("block\n"), 0o644))
	vaultDir := filepath.Join(srcDir, config.VaultDirName)
	require.NoError(t, os.MkdirAll(vaultDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(vaultDir, ".vault_keys"), []byte(`{"mnemonic":"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about","passphrase":"NODE_PASS"}`), 0o600))

	_, err = backup.Backup(srcDir, archivePath, "")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(dstDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, config.ConfigFileName), []byte("{}"), 0o644))

	_, err = backup.Restore(archivePath, dstDir, false)
	require.Error(t, err)

	_, err = backup.Restore(archivePath, dstDir, true)
	require.NoError(t, err)
}
