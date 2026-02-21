package config

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/cerera/core/address"
	"github.com/cerera/core/crypto"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const DefaultP2pPort = int(6116)
const DefaultRpcPort = int(1337)

var ChainId = big.NewInt(133707331)

type ChainConfig struct {
	ChainID int
	Path    string
	Type    string
}
type NetworkConfig struct {
	PID           protocol.ID
	P2P           int
	RPC           int
	ADDR          address.Address // address of running node
	PRIV          string          // private key of current running node
	PUB           []byte          // public key of current running node
	BootstrapIP   string          // bootstrap node IP address (deprecated, use SeedNodes)
	BootstrapPort string          // bootstrap node port (deprecated, use SeedNodes)
	SeedNodes     []string        // список seed nodes в формате multiaddr libp2p (например: "/ip4/192.168.1.6/tcp/31100") или "ip:port" (будет автоматически конвертирован)
	// Libp2p specific configuration
	BootstrapNodes []string // libp2p bootstrap nodes в формате multiaddr (например: "/ip4/192.168.1.6/tcp/31100/p2p/QmPeerID"). Если пусто, используется SeedNodes
	RelayNodes     []string // libp2p relay nodes в формате multiaddr для NAT traversal (опционально)
	DHTServerMode  bool     // включить DHT server mode для bootstrap узлов (по умолчанию: false, auto mode)
	EnableMDNS     bool     // включить mDNS discovery для локальной сети (по умолчанию: true)
}
type VaultConfig struct {
	MEM  bool
	PATH string
}
type PoolConfig struct {
	MinGas  float64
	MaxSize int
}
type HttpSecConfig struct {
	TLS bool
}
type Sec struct {
	HTTP HttpSecConfig
}

// main configuration struct
type Config struct {
	Vault   VaultConfig
	Chain   ChainConfig // chain config
	TlsFlag bool
	NetCfg  NetworkConfig // network config (p2p, inner address, keys)
	POOL    PoolConfig    // pool config
	SEC     Sec
	AUTOGEN bool   // auto generating blocks
	VERSION string // version field
	VER     int    // other version field
	Gossip  string
	IN_MEM  bool // storage inmem?
}

func GenerageConfig() *Config {
	configFilePath := "config.json"
	cfg := &Config{}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// Default seed nodes
		defaultSeedNodes := []string{
			"/ip4/172.20.0.11/tcp/31100",
			"/ip4/172.25.0.11/tcp/31100",
			"/ip4/172.25.0.12/tcp/31101",
			"/ip4/192.168.1.6/tcp/31100",
		}

		// Check for SEED_NODES environment variable (comma-separated list)
		// Format: "/ip4/172.25.0.11/tcp/31100,/ip4/172.25.0.12/tcp/31101"
		if envSeedNodes := os.Getenv("SEED_NODES"); envSeedNodes != "" {
			seedNodesList := []string{}
			// Split by comma and trim spaces
			for _, seed := range splitAndTrim(envSeedNodes, ",") {
				if seed != "" {
					seedNodesList = append(seedNodesList, seed)
				}
			}
			if len(seedNodesList) > 0 {
				defaultSeedNodes = seedNodesList
				fmt.Printf("[CONFIG] Using seed nodes from environment: %v\n", defaultSeedNodes)
			}
		} else {
			fmt.Printf("[CONFIG] Using default seed nodes: %v\n", defaultSeedNodes)
		}

		cfg = &Config{
			TlsFlag: false,
			POOL: PoolConfig{
				MinGas:  0.01,
				MaxSize: 256 * 256, // bytes
			},
			Vault: VaultConfig{
				MEM:  true,
				PATH: "EMPTY",
			},
			SEC: Sec{
				HTTP: HttpSecConfig{
					TLS: false,
				},
			},
			NetCfg: NetworkConfig{
				PID:           "/vavilov/1.0.0",
				BootstrapIP:   "192.168.1.6",
				BootstrapPort: "31100",
				// Seed nodes in multiaddr format for libp2p
				SeedNodes:      defaultSeedNodes,
				BootstrapNodes: defaultSeedNodes,
				EnableMDNS:     true, // Disabled by default to avoid multicast interface warnings on Windows
			},
			Chain: ChainConfig{
				ChainID: 11,
				Path:    "EMPTY",
				Type:    "VAVILOV",
			},
			VERSION: "ALPHA",
			VER:     1,
			Gossip:  "127.0.0.1:8091",
			IN_MEM:  true,
		}
		cfg.WriteConfigToFile()
	} else {
		cfg, err = ReadConfig(configFilePath)
		if err != nil {
			panic(err)
		}

		// Override seed nodes from environment variable if set
		if envSeedNodes := os.Getenv("SEED_NODES"); envSeedNodes != "" {
			seedNodesList := []string{}
			for _, seed := range splitAndTrim(envSeedNodes, ",") {
				if seed != "" {
					seedNodesList = append(seedNodesList, seed)
				}
			}
			if len(seedNodesList) > 0 {
				cfg.NetCfg.SeedNodes = seedNodesList
				cfg.NetCfg.BootstrapNodes = seedNodesList
				fmt.Printf("[CONFIG] Updated seed nodes from environment: %v\n", seedNodesList)
				cfg.WriteConfigToFile() // Save updated config
			}
		}
	}
	return cfg
}

// splitAndTrim splits a string by separator and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
func (cfg *Config) SetPorts(rpc int, p2p int) {
	if rpc == -1 || rpc == 0 {
		cfg.NetCfg.RPC = DefaultRpcPort
	} else {
		cfg.NetCfg.RPC = rpc
	}
	if p2p == -1 || p2p == 0 {
		cfg.NetCfg.P2P = DefaultP2pPort
	} else {
		cfg.NetCfg.P2P = p2p
	}
	cfg.WriteConfigToFile()
}
func (cfg *Config) SetNodeKey(pemFilePath string) {
	if pemFilePath == "" {
		// use dafault
		pemFilePath = "ddddd.nodekey.pem"
	}
	var currentNodeAddress address.Address
	var nodeK *ecdsa.PrivateKey
	var ppk string
	{ // private key of node
		if _, err := os.Stat(pemFilePath); err == nil {
			f, err := os.Open(pemFilePath)
			if err != nil {
				panic(err)
			}
			b1 := make([]byte, 221)
			n1, err := f.Read(b1)
			if err != nil {
				panic(err)
			}
			ppk = string(b1[:n1])
			nodeK = crypto.DecodePrivKey(ppk)
			ppk = crypto.EncodePrivateKeyToToString(nodeK)
			currentNodeAddress = crypto.PubkeyToAddress(nodeK.PublicKey)
		} else {
			nodeK, _ = crypto.GenerateAccount()
			currentNodeAddress = crypto.PubkeyToAddress(nodeK.PublicKey)
			ppk = crypto.EncodePrivateKeyToToString(nodeK)
			err := os.WriteFile(pemFilePath, []byte(ppk), 0644)
			if err != nil {
				panic(err)
			}
		}
	}
	cfg.NetCfg.ADDR = currentNodeAddress
	cfg.NetCfg.PRIV = ppk
	cfg.NetCfg.PUB = crypto.EncodePublicKeyToByte(&nodeK.PublicKey)

	cfg.WriteConfigToFile()
}
func (cfg *Config) SetAutoGen(f bool) {
	if !cfg.AUTOGEN {
		cfg.AUTOGEN = false
	}
	cfg.AUTOGEN = f
	cfg.WriteConfigToFile()
}
func (cfg *Config) CheckVersion(version string, ver int) bool {
	return (cfg.VER == ver) && (cfg.VERSION == version)
}
func (cfg *Config) GetVersion() string {
	return fmt.Sprintf("%s-%d_VERSION", cfg.VERSION, cfg.VER)
}
func (cfg *Config) UpdateVaultPath(newPath string) {
	cfg.Vault.PATH = newPath
	cfg.WriteConfigToFile()
}
func (cfg *Config) UpdateChainPath(newPath string) {
	cfg.Chain.Path = newPath
	cfg.WriteConfigToFile()
}
func (cfg *Config) WriteConfigToFile() error {
	fileData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("config.json", fileData, 0644)
	if err != nil {
		panic(err)
	}
	return nil
}
func (cfg *Config) SetInMem(p bool) {
	cfg.IN_MEM = p
	cfg.WriteConfigToFile()
}
func ReadConfig(filePath string) (*Config, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config from file: %v", err)
	}
	var cfg Config
	err = json.Unmarshal(fileData, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Convert old format seed nodes to multiaddr format if needed
	cfg.convertSeedNodesToMultiaddr()

	return &cfg, nil
}

// convertSeedNodesToMultiaddr converts seed nodes from ip:port format to multiaddr format
func (cfg *Config) convertSeedNodesToMultiaddr() {
	// If BootstrapNodes is empty but SeedNodes has old format, convert them
	if len(cfg.NetCfg.BootstrapNodes) == 0 && len(cfg.NetCfg.SeedNodes) > 0 {
		converted := false
		newSeedNodes := make([]string, 0, len(cfg.NetCfg.SeedNodes))

		for _, seed := range cfg.NetCfg.SeedNodes {
			// Check if already in multiaddr format
			if len(seed) > 0 && seed[0] == '/' {
				// Already in multiaddr format
				newSeedNodes = append(newSeedNodes, seed)
			} else {
				// Convert ip:port to multiaddr format
				// Extract IP and port
				var ip, port string
				for i := len(seed) - 1; i >= 0; i-- {
					if seed[i] == ':' {
						ip = seed[:i]
						port = seed[i+1:]
						break
					}
				}

				if ip == "" {
					ip = seed
					port = "31100" // default port
				}

				multiaddrStr := fmt.Sprintf("/ip4/%s/tcp/%s", ip, port)
				newSeedNodes = append(newSeedNodes, multiaddrStr)
				converted = true
			}
		}

		if converted {
			cfg.NetCfg.SeedNodes = newSeedNodes
			cfg.NetCfg.BootstrapNodes = newSeedNodes
			cfg.WriteConfigToFile() // Save converted config
		}
	}
}
