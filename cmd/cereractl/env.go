package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cerera/config"
)

const nodeMetaFile = "cerera.node.json"

type nodeMeta struct {
	PID       int       `json:"pid"`
	HTTPPort  int       `json:"http_port"`
	P2PPort   string    `json:"p2p_port"`
	DataDir   string    `json:"data_dir"`
	KeyPath   string    `json:"key_path"`
	StartedAt time.Time `json:"started_at"`
	LogFile   string    `json:"log_file"`
}

type nodeOptions struct {
	dataDir   string
	keyPath   string
	httpPort  int
	p2pPort   string
	mode      string
	miner     bool
	inMem     bool
	cereraBin string
}

func defaultDataDir() string {
	if dir := os.Getenv("CERERA_DATA_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
		return dir
	}
	return "."
}

func resolveDataDir(flagValue string) (string, error) {
	dir := flagValue
	if dir == "" {
		dir = defaultDataDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func nodeMetaPath(dataDir string) string {
	return filepath.Join(dataDir, nodeMetaFile)
}

func loadNodeMeta(dataDir string) (*nodeMeta, error) {
	data, err := os.ReadFile(nodeMetaPath(dataDir))
	if err != nil {
		return nil, err
	}
	var meta nodeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func saveNodeMeta(dataDir string, meta *nodeMeta) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(nodeMetaPath(dataDir), payload, 0o644)
}

func removeNodeMeta(dataDir string) {
	_ = os.Remove(nodeMetaPath(dataDir))
}

func defaultKeyPath(dataDir, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	candidates := []string{
		filepath.Join(dataDir, "ddddd.nodekey.pem"),
		"ddddd.nodekey.pem",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "ddddd.nodekey.pem"
}

func defaultHTTPPort(dataDir string, flagValue int) int {
	if flagValue > 0 {
		return flagValue
	}
	cfgPath := filepath.Join(dataDir, config.ConfigFileName)
	if cfg, err := config.ReadConfig(cfgPath); err == nil && cfg.NetCfg.RPC > 0 {
		return cfg.NetCfg.RPC
	}
	return config.DefaultRpcPort
}

func parseNodeOptions(args []string) nodeOptions {
	fs := newFlagSet("node")
	dataDir := fs.String("data-dir", defaultDataDir(), "Cerera data directory")
	keyPath := fs.String("key", "", "Node PEM key path")
	httpPort := fs.Int("http", 0, "HTTP/RPC port (default from config or 1337)")
	p2pPort := fs.String("port", "31000", "P2P port")
	mode := fs.String("mode", "p2p", "Node mode: server, client, p2p")
	miner := fs.Bool("miner", true, "Enable mining")
	inMem := fs.Bool("mem", false, "In-memory storage (ignored when --data-dir is set)")
	cereraBin := fs.String("cerera-bin", "", "Path to cerera binary (default: PATH or sibling)")
	_ = fs.Parse(args)

	resolvedDir, err := resolveDataDir(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid data dir: %v\n", err)
		os.Exit(2)
	}

	return nodeOptions{
		dataDir:   resolvedDir,
		keyPath:   defaultKeyPath(resolvedDir, *keyPath),
		httpPort:  defaultHTTPPort(resolvedDir, *httpPort),
		p2pPort:   *p2pPort,
		mode:      *mode,
		miner:     *miner,
		inMem:     *inMem,
		cereraBin: *cereraBin,
	}
}

func (o nodeOptions) cereraArgs() []string {
	args := []string{
		"--mode=" + o.mode,
		fmt.Sprintf("--http=%d", o.httpPort),
		"--port=" + o.p2pPort,
		"--key=" + o.keyPath,
		"--data-dir=" + o.dataDir,
	}
	if o.miner {
		args = append(args, "--miner")
	} else {
		args = append(args, "--miner=false")
	}
	if o.inMem {
		args = append(args, "--mem=true")
	}
	return args
}

func (o nodeOptions) stdoutLogPath() string {
	return filepath.Join(o.dataDir, "cerera.stdout.log")
}
