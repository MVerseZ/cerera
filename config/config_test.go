package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	var nilCfg *Config
	if nilCfg.ResolveDataDir() != "" {
		t.Fatal("nil data dir")
	}
	if nilCfg.ResolveChainFile() != "./"+ChainFileName {
		t.Fatalf("nil chain file: %s", nilCfg.ResolveChainFile())
	}
	if nilCfg.ResolveVaultDir() != "./"+VaultDirName {
		t.Fatalf("nil vault dir: %s", nilCfg.ResolveVaultDir())
	}
	if err := nilCfg.ApplyDataDir(t.TempDir()); err == nil {
		t.Fatal("expected error for nil config")
	}

	cfg := &Config{Chain: ChainConfig{Path: "EMPTY"}, Vault: VaultConfig{PATH: "EMPTY"}}
	if cfg.ResolveChainFile() != "./"+ChainFileName || cfg.ResolveVaultDir() != "./"+VaultDirName {
		t.Fatalf("empty paths: chain=%s vault=%s", cfg.ResolveChainFile(), cfg.ResolveVaultDir())
	}
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	if cfg.ResolveChainFile() != filepath.Join(cfg.DataDir, ChainFileName) {
		t.Fatalf("data dir chain: %s", cfg.ResolveChainFile())
	}
	if cfg.ResolveVaultDir() != filepath.Join(cfg.DataDir, VaultDirName) {
		t.Fatalf("data dir vault: %s", cfg.ResolveVaultDir())
	}
	cfg.Chain.Path = filepath.Join(cfg.DataDir, "custom.dat")
	cfg.Vault.PATH = filepath.Join(cfg.DataDir, "custom-vault")
	if cfg.ResolveChainFile() != cfg.Chain.Path || cfg.ResolveVaultDir() != cfg.Vault.PATH {
		t.Fatal("explicit paths were ignored")
	}
}

func TestApplyDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "node")
	cfg := &Config{}
	if err := cfg.ApplyDataDir(dir); err != nil {
		t.Fatal(err)
	}
	if cfg.IN_MEM || cfg.Vault.MEM {
		t.Fatal("data dir must disable in-memory mode")
	}
	if cfg.Chain.Path != filepath.Join(cfg.DataDir, ChainFileName) {
		t.Fatalf("chain path %s", cfg.Chain.Path)
	}
	info, err := os.Stat(cfg.DataDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("data dir not created: %v", err)
	}

	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ApplyDataDir(blocked); err == nil {
		t.Fatal("expected mkdir error when data dir is a file")
	}
}

func TestOpenConfigCreateReadAndSeedEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFileName)
	t.Setenv("SEED_NODES", " /ip4/10.0.0.8/tcp/31100 , , /ip4/10.0.0.9/tcp/31101 ")

	cfg, err := OpenConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VERSION != "ALPHA" || cfg.VER != 1 || cfg.Chain.ChainID != 11 {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
	if len(cfg.NetCfg.SeedNodes) != 2 || cfg.NetCfg.SeedNodes[0] != "/ip4/10.0.0.8/tcp/31100" {
		t.Fatalf("seed nodes: %#v", cfg.NetCfg.SeedNodes)
	}
	if !cfg.CheckVersion("ALPHA", 1) || cfg.CheckVersion("ALPHA", 2) {
		t.Fatal("CheckVersion")
	}
	if cfg.GetVersion() != "ALPHA-1_VERSION" {
		t.Fatalf("version %s", cfg.GetVersion())
	}

	loaded, err := ReadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Gossip != cfg.Gossip {
		t.Fatalf("gossip %s", loaded.Gossip)
	}

	t.Setenv("SEED_NODES", "/ip4/10.1.0.1/tcp/4000")
	reloaded, err := OpenConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.NetCfg.SeedNodes) != 1 || reloaded.NetCfg.SeedNodes[0] != "/ip4/10.1.0.1/tcp/4000" {
		t.Fatalf("env override: %#v", reloaded.NetCfg.SeedNodes)
	}
	if reloaded.DataDir == "" {
		t.Fatal("expected data dir from config path")
	}

	if _, err := OpenConfig(dir); err == nil {
		t.Fatal("expected read error for a directory path")
	}
	if _, err := ReadConfig(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected missing file error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConfig(bad); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestOpenConfigDefaultSeeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SEED_NODES", " , ")
	cfg, err := OpenConfig(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NetCfg.SeedNodes) != 4 {
		t.Fatalf("expected built-in seeds, got %#v", cfg.NetCfg.SeedNodes)
	}
}

func TestGenerageConfigAndMutators(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("SEED_NODES", "")

	cfg, err := GenerageConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir

	cfg.SetPorts(0, -1)
	if cfg.NetCfg.RPC != DefaultRpcPort || cfg.NetCfg.P2P != DefaultP2pPort {
		t.Fatalf("default ports rpc=%d p2p=%d", cfg.NetCfg.RPC, cfg.NetCfg.P2P)
	}
	cfg.SetPorts(1400, 1500)
	if cfg.NetCfg.RPC != 1400 || cfg.NetCfg.P2P != 1500 {
		t.Fatalf("custom ports rpc=%d p2p=%d", cfg.NetCfg.RPC, cfg.NetCfg.P2P)
	}

	cfg.SetAutoGen(true)
	if !cfg.AUTOGEN {
		t.Fatal("autogen not set")
	}
	cfg.SetAutoGen(false)
	if cfg.AUTOGEN {
		t.Fatal("autogen not cleared")
	}
	cfg.SetInMem(true)
	if !cfg.IN_MEM {
		t.Fatal("in-mem flag")
	}
	cfg.UpdateChainPath(filepath.Join(dir, "chain.dat"))
	cfg.UpdateVaultPath(filepath.Join(dir, "vault"))
	if cfg.Chain.Path == "" || cfg.Vault.PATH == "" {
		t.Fatal("paths not updated")
	}

	persisted, err := ReadConfig(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if persisted.NetCfg.RPC != 1400 || !persisted.IN_MEM {
		t.Fatalf("file not updated: %+v", persisted.NetCfg)
	}
}

func TestSetNodeKey(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{DataDir: dir}

	keyPath := filepath.Join(dir, "nodekey.pem")
	if err := cfg.SetNodeKey(keyPath); err != nil {
		t.Fatal(err)
	}
	if cfg.NetCfg.PRIV == "" || len(cfg.NetCfg.PUB) == 0 || cfg.NetCfg.ADDR == (cfg.NetCfg.ADDR) && len(cfg.NetCfg.PRIV) == 0 {
		t.Fatal("node key was not stored")
	}
	first := cfg.NetCfg.ADDR
	if err := cfg.SetNodeKey(keyPath); err != nil {
		t.Fatalf("reload existing key: %v", err)
	}
	if cfg.NetCfg.ADDR != first {
		t.Fatal("reloaded key changed the node address")
	}

	if err := os.WriteFile(keyPath, []byte("not a pem"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetNodeKey(keyPath); err == nil {
		t.Fatal("expected decode error")
	}
	if err := cfg.SetNodeKey(dir); err == nil {
		t.Fatal("expected open error for a directory")
	}

	t.Chdir(dir)
	fresh := &Config{DataDir: dir}
	if err := fresh.SetNodeKey(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ddddd.nodekey.pem")); err != nil {
		t.Fatal(err)
	}
}

func TestConvertSeedNodesToMultiaddr(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("SEED_NODES", "")

	cfg := newDefaultConfig(nil)
	cfg.DataDir = dir
	cfg.NetCfg.SeedNodes = []string{"10.0.0.5:32000", "/ip4/10.0.0.6/tcp/1", "10.0.0.7"}
	cfg.NetCfg.BootstrapNodes = nil
	if err := cfg.WriteConfigToFile(); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadConfig(configFilePath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/ip4/10.0.0.5/tcp/32000", "/ip4/10.0.0.6/tcp/1", "/ip4/10.0.0.7/tcp/31100"}
	if strings.Join(loaded.NetCfg.SeedNodes, ",") != strings.Join(want, ",") {
		t.Fatalf("converted seeds %#v", loaded.NetCfg.SeedNodes)
	}
	if strings.Join(loaded.NetCfg.BootstrapNodes, ",") != strings.Join(want, ",") {
		t.Fatalf("bootstrap %#v", loaded.NetCfg.BootstrapNodes)
	}

	loaded.NetCfg.BootstrapNodes = []string{"/ip4/1.2.3.4/tcp/1"}
	loaded.NetCfg.SeedNodes = []string{"9.9.9.9:1"}
	if err := loaded.WriteConfigToFile(); err != nil {
		t.Fatal(err)
	}
	again, err := ReadConfig(configFilePath(loaded))
	if err != nil {
		t.Fatal(err)
	}
	if len(again.NetCfg.SeedNodes) != 1 || again.NetCfg.SeedNodes[0] != "9.9.9.9:1" {
		t.Fatalf("seeds should stay untouched when bootstrap is set: %#v", again.NetCfg.SeedNodes)
	}

	applySeedNodesFromEnv(nil)
}

func TestSplitAndTrim(t *testing.T) {
	got := splitAndTrim(" a, ,b ,", ",")
	if strings.Join(got, "|") != "a|b" {
		t.Fatalf("%#v", got)
	}
}
