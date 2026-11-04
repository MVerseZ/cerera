package cli

import (
	"path/filepath"
	"testing"
)

func TestParseOptions_InMemWithoutDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	opts, err := ParseOptions([]string{"-mem"})
	if err != nil {
		t.Fatalf("ParseOptions: %v", err)
	}
	if !opts.Config.IN_MEM {
		t.Fatal("expected IN_MEM from -mem flag")
	}
	if opts.LogPath != "logfile" {
		t.Fatalf("LogPath = %q, want logfile", opts.LogPath)
	}
	if opts.Port != "31000" {
		t.Fatalf("Port = %q, want default 31000", opts.Port)
	}
}

func TestParseOptions_DataDir(t *testing.T) {
	dataDir := t.TempDir()

	opts, err := ParseOptions([]string{"-data-dir", dataDir, "-mem"})
	if err != nil {
		t.Fatalf("ParseOptions: %v", err)
	}
	if !opts.Config.IN_MEM {
		t.Fatal("expected IN_MEM")
	}
	wantLog := filepath.Join(dataDir, "logfile")
	if opts.LogPath != wantLog {
		t.Fatalf("LogPath = %q, want %q", opts.LogPath, wantLog)
	}
	if opts.Config.DataDir != dataDir {
		t.Fatalf("DataDir = %q, want %q", opts.Config.DataDir, dataDir)
	}
}

func TestParseOptions_UnknownFlag(t *testing.T) {
	_, err := ParseOptions([]string{"-not-a-real-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
