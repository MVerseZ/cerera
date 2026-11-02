package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoggerSinksAndLevels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	log, closers, err := newLogger(Config{Path: path, Level: "debug", Console: true})
	if err != nil {
		t.Fatal(err)
	}
	log.Info("hello-file")
	if err := log.Sync(); err != nil && !os.IsNotExist(err) {
		// stdout sync can fail on some platforms; the file sink is what we assert
	}
	for _, c := range closers {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("hello-file")) {
		t.Fatalf("log file %s", data)
	}

	fallback, closers, err := newLogger(Config{Level: "not-a-level"})
	if err != nil || fallback == nil {
		t.Fatal(err)
	}
	for _, c := range closers {
		_ = c.Close()
	}
	fallback.Info("console-default")

	if _, _, err := newLogger(Config{Path: filepath.Join(dir, "missing", "nope.log")}); err == nil {
		t.Fatal("expected open error")
	}
	empty, closers, err := newLogger(Config{})
	if err != nil || empty == nil {
		t.Fatal(err)
	}
	for _, c := range closers {
		_ = c.Close()
	}
}

func TestInitHelpersAndSync(t *testing.T) {
	Sync()
	dir := t.TempDir()
	log, err := Init(Config{Path: filepath.Join(dir, "global.log"), Level: "info", Console: false})
	if err != nil || log == nil {
		t.Fatal(err)
	}
	if L() == nil || Sugar() == nil || Named("node") == nil {
		t.Fatal("helpers returned nil")
	}
	Sugar().Infow("ready", "k", 1)
	Named("node").Info("named")
	again, err := Init(Config{Level: "debug", Console: true})
	if err != nil || again == nil {
		t.Fatal(err)
	}
	Sync()
	Sync()
}
