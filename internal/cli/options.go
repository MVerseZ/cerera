package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cerera/config"
)

// Options holds configuration and runtime flags for a Cerera node process.
type Options struct {
	Config   config.Config
	Mode     string
	Port     string
	HTTPPort int
	Mine     bool
	LogPath  string
}

// ParseOptions loads config and parses node flags from args (typically os.Args[1:]).
func ParseOptions(args []string) (Options, error) {
	var opts Options

	fs := flag.NewFlagSet("cerera", flag.ContinueOnError)
	port := fs.String("port", "31000", "p2p port for connection")
	keyPath := fs.String("key", "", "path to pem key")
	mode := fs.String("mode", "server", "Режим работы: server, client, or p2p")
	httpPort := fs.Int("http", 8080, "Порт для http сервера")
	mine := fs.Bool("miner", true, "Флаг для добычи новых блоков")
	inMem := fs.Bool("mem", false, "Хранение данных в памяти (true) или на диске (false)")
	dataDir := fs.String("data-dir", "", "Каталог данных: chain.dat, vault/, config.json")
	tls := fs.Bool("s", false, "Включить HTTPS (TLS)")

	if err := fs.Parse(args); err != nil {
		return opts, fmt.Errorf("parse flags: %w", err)
	}

	if *dataDir == "" {
		if envDir := os.Getenv("CERERA_DATA_DIR"); envDir != "" {
			*dataDir = envDir
		}
	}

	var cfg *config.Config
	var err error
	logPath := "logfile"

	if *dataDir != "" {
		if err := os.MkdirAll(*dataDir, 0o700); err != nil {
			return opts, fmt.Errorf("create data dir: %w", err)
		}
		cfgPath := filepath.Join(*dataDir, config.ConfigFileName)
		cfg, err = config.OpenConfig(cfgPath)
		if err != nil {
			return opts, fmt.Errorf("open config: %w", err)
		}
		if err := cfg.ApplyDataDir(*dataDir); err != nil {
			return opts, fmt.Errorf("apply data dir: %w", err)
		}
		if *inMem {
			cfg.SetInMem(true)
		} else {
			cfg.IN_MEM = false
			if err := cfg.WriteConfigToFile(); err != nil {
				return opts, fmt.Errorf("write config: %w", err)
			}
		}
		logPath = filepath.Join(*dataDir, "logfile")
	} else {
		cfg, err = config.GenerageConfig()
		if err != nil {
			return opts, fmt.Errorf("generate config: %w", err)
		}
		cfg.SetInMem(*inMem)
	}

	cfg.SetNodeKey(*keyPath)
	cfg.SetAutoGen(true)
	cfg.SEC.HTTP.TLS = *tls

	opts.Config = *cfg
	opts.Mode = *mode
	opts.Port = *port
	opts.HTTPPort = *httpPort
	opts.Mine = *mine
	opts.LogPath = logPath
	return opts, nil
}
