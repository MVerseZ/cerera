// Package backup creates and restores Cerera node data snapshots (chain + vault + config).
package backup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cerera/config"
)

const manifestName = "cerera-backup-manifest.json"
const formatVersion = 1

// Manifest describes a backup archive produced by Backup().
type Manifest struct {
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	DataDir     string    `json:"data_dir,omitempty"`
	ChainFile   string    `json:"chain_file"`
	VaultDir    string    `json:"vault_dir"`
	ConfigFile  string    `json:"config_file"`
	NodeKeyFile string    `json:"node_key_file,omitempty"`
	ChainBlocks int       `json:"chain_blocks"`
	Includes    []string  `json:"includes"`
}

// Layout holds resolved paths for a Cerera data directory.
type Layout struct {
	DataDir    string
	ChainFile  string
	VaultDir   string
	ConfigFile string
}

// ResolveLayout reads config.json (when present) or uses default filenames under dataDir.
func ResolveLayout(dataDir string) (Layout, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve data dir: %w", err)
	}

	layout := Layout{
		DataDir:    abs,
		ChainFile:  filepath.Join(abs, config.ChainFileName),
		VaultDir:   filepath.Join(abs, config.VaultDirName),
		ConfigFile: filepath.Join(abs, config.ConfigFileName),
	}

	cfgPath := layout.ConfigFile
	if _, err := os.Stat(cfgPath); err == nil {
		cfg, err := config.ReadConfig(cfgPath)
		if err != nil {
			return Layout{}, fmt.Errorf("read config: %w", err)
		}
		layout.ChainFile = cfg.ResolveChainFile()
		layout.VaultDir = cfg.ResolveVaultDir()
		layout.ConfigFile = cfgPath
	}
	return layout, nil
}

// Backup writes a gzip-compressed tar archive with chain, vault, config, and optional node key.
func Backup(dataDir, archivePath, nodeKeyPath string) (*Manifest, error) {
	layout, err := ResolveLayout(dataDir)
	if err != nil {
		return nil, err
	}

	if err := validateBackupSources(layout); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Clean(archivePath)), 0o755); err != nil && filepath.Dir(archivePath) != "." {
		return nil, fmt.Errorf("create archive dir: %w", err)
	}

	out, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	manifest := &Manifest{
		Version:    formatVersion,
		CreatedAt:  time.Now().UTC(),
		DataDir:    layout.DataDir,
		ChainFile:  config.ChainFileName,
		VaultDir:   config.VaultDirName + "/",
		ConfigFile: config.ConfigFileName,
		Includes:   []string{config.ConfigFileName},
	}
	manifest.ChainBlocks = countChainBlocks(layout.ChainFile)

	if err := addFileToTar(tw, layout.ConfigFile, config.ConfigFileName); err != nil {
		return nil, err
	}
	if err := addFileToTar(tw, layout.ChainFile, config.ChainFileName); err != nil {
		return nil, err
	}
	manifest.Includes = append(manifest.Includes, config.ChainFileName)

	vaultPrefix := config.VaultDirName + string(os.PathSeparator)
	if err := addDirToTar(tw, layout.VaultDir, vaultPrefix); err != nil {
		return nil, err
	}
	manifest.Includes = append(manifest.Includes, config.VaultDirName+"/")

	if nodeKeyPath != "" {
		if _, err := os.Stat(nodeKeyPath); err == nil {
			if err := addFileToTar(tw, nodeKeyPath, "nodekey.pem"); err != nil {
				return nil, err
			}
			manifest.NodeKeyFile = "nodekey.pem"
			manifest.Includes = append(manifest.Includes, "nodekey.pem")
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := addBytesToTar(tw, manifestName, manifestBytes, 0o644); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return manifest, nil
}

// Inspect reads manifest metadata from an archive without extracting it.
func Inspect(archivePath string) (*Manifest, error) {
	manifestBytes, err := readTarEntry(archivePath, manifestName)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &manifest, nil
}

// Restore extracts a backup archive into dataDir. Existing files are moved aside unless force is true.
func Restore(archivePath, dataDir string, force bool) (*Manifest, error) {
	manifest, err := Inspect(archivePath)
	if err != nil {
		return nil, err
	}
	if manifest.Version != formatVersion {
		return nil, fmt.Errorf("unsupported backup version %d", manifest.Version)
	}

	layout, err := ResolveLayout(dataDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(layout.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	if !force {
		if _, err := os.Stat(layout.ConfigFile); err == nil {
			return nil, fmt.Errorf("data dir already initialized at %s (use --force to overwrite)", layout.DataDir)
		}
	} else if err := quarantineExisting(layout); err != nil {
		return nil, err
	}

	if err := extractArchive(archivePath, layout.DataDir); err != nil {
		return nil, err
	}

	cfgPath := filepath.Join(layout.DataDir, config.ConfigFileName)
	if _, err := os.Stat(cfgPath); err == nil {
		cfg, err := config.ReadConfig(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("read restored config: %w", err)
		}
		if err := cfg.ApplyDataDir(layout.DataDir); err != nil {
			return nil, err
		}
		if err := cfg.WriteConfigToFile(); err != nil {
			return nil, fmt.Errorf("write restored config: %w", err)
		}
	}

	return manifest, nil
}

func validateBackupSources(layout Layout) error {
	if _, err := os.Stat(layout.ConfigFile); err != nil {
		return fmt.Errorf("config missing: %s", layout.ConfigFile)
	}
	if _, err := os.Stat(layout.ChainFile); os.IsNotExist(err) {
		return fmt.Errorf("chain file missing: %s", layout.ChainFile)
	}
	if info, err := os.Stat(layout.VaultDir); err != nil || !info.IsDir() {
		return fmt.Errorf("vault dir missing: %s", layout.VaultDir)
	}
	keysFile := filepath.Join(layout.VaultDir, ".vault_keys")
	if _, err := os.Stat(keysFile); err != nil {
		return fmt.Errorf("vault keys missing: %s (required to decrypt accounts)", keysFile)
	}
	return nil
}

func countChainBlocks(chainFile string) int {
	f, err := os.Open(chainFile)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

func addFileToTar(tw *tar.Writer, srcPath, tarName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}
	if info.IsDir() {
		return addDirToTar(tw, srcPath, tarName)
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer file.Close()

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(tarName)
	return writeTarEntry(tw, header, file)
}

func addDirToTar(tw *tar.Writer, srcDir, tarPrefix string) error {
	tarPrefix = filepath.ToSlash(strings.TrimSuffix(tarPrefix, "/")) + "/"
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := tarPrefix + filepath.ToSlash(rel)
		if info.IsDir() {
			header := &tar.Header{
				Name:     name,
				Typeflag: tar.TypeDir,
				Mode:     0o700,
				ModTime:  info.ModTime(),
			}
			return tw.WriteHeader(header)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		return writeTarEntry(tw, header, file)
	})
}

func addBytesToTar(tw *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarEntry(tw *tar.Writer, header *tar.Header, r io.Reader) error {
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := io.Copy(tw, r)
	return err
}

func readTarEntry(archivePath, entryName string) ([]byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == entryName || header.Name == entryName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("manifest not found in archive")
}

func extractArchive(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			return fmt.Errorf("unsupported tar entry type %c for %s", header.Typeflag, header.Name)
		}
	}
}

func quarantineExisting(layout Layout) error {
	stamp := time.Now().UTC().Format("20060102-150405")
	backupRoot := filepath.Join(layout.DataDir, ".restore-backup-"+stamp)
	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		return err
	}
	for _, src := range []string{layout.ConfigFile, layout.ChainFile, layout.VaultDir} {
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(backupRoot, filepath.Base(src))
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("quarantine %s: %w", src, err)
		}
	}
	return nil
}
