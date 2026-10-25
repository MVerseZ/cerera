package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cerera/config"
	"github.com/cerera/core/backup"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "backup":
		runBackup(os.Args[2:])
	case "restore":
		runRestore(os.Args[2:])
	case "inspect":
		runInspect(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func runBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	dataDir := fs.String("data-dir", ".", "Cerera data directory (chain.dat, vault/, config.json)")
	output := fs.String("output", "", "Output .tar.gz archive path (required)")
	nodeKey := fs.String("node-key", "", "Optional node PEM key to include in the archive")
	_ = fs.Parse(args)

	if *output == "" {
		fmt.Fprintln(os.Stderr, "backup: --output is required")
		os.Exit(2)
	}

	manifest, err := backup.Backup(*dataDir, *output, *nodeKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup written to %s\n", *output)
	fmt.Printf("  chain blocks: %d\n", manifest.ChainBlocks)
	fmt.Printf("  includes: %v\n", manifest.Includes)
}

func runRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dataDir := fs.String("data-dir", ".", "Target Cerera data directory")
	input := fs.String("input", "", "Input .tar.gz archive path (required)")
	force := fs.Bool("force", false, "Overwrite existing data directory contents")
	_ = fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "restore: --input is required")
		os.Exit(2)
	}

	manifest, err := backup.Restore(*input, *dataDir, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "restore failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Restored backup from %s into %s\n", *input, *dataDir)
	fmt.Printf("  created at: %s\n", manifest.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  chain blocks: %d\n", manifest.ChainBlocks)
	if manifest.NodeKeyFile != "" {
		fmt.Printf("  node key in archive: %s (copy manually if needed)\n", manifest.NodeKeyFile)
	}
}

func runInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	input := fs.String("input", "", "Backup .tar.gz archive path (required)")
	_ = fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "inspect: --input is required")
		os.Exit(2)
	}

	manifest, err := backup.Inspect(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup version: %d\n", manifest.Version)
	fmt.Printf("Created: %s\n", manifest.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Chain blocks: %d\n", manifest.ChainBlocks)
	fmt.Printf("Includes: %v\n", manifest.Includes)
	if manifest.NodeKeyFile != "" {
		fmt.Printf("Node key: %s\n", manifest.NodeKeyFile)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Cerera node data utility

Usage:
  cereractl backup  --data-dir <dir> --output <file.tar.gz> [--node-key <pem>]
  cereractl restore --data-dir <dir> --input <file.tar.gz> [--force]
  cereractl inspect --input <file.tar.gz>

Data directory layout (disk mode):
  %s/
    %s
    %s/
    %s

Stop the cerera node before restore.

Environment:
  CERERA_DATA_DIR  Default for --data-dir when set

`, "data", config.ChainFileName, config.VaultDirName, config.ConfigFileName)
}

func init() {
	if dir := os.Getenv("CERERA_DATA_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			_ = os.Setenv("CERERA_DATA_DIR", abs)
		}
	}
}
