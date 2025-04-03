package config

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/cerera/internal/cerera/types"
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
	PID  protocol.ID
	P2P  int
	RPC  int
	ADDR types.Address // address of running node
	PRIV string        // private key of current running node
	PUB  []byte        // public key of current running node
}
type VaultConfig struct {
	MEM  bool
	PATH string
}
type PoolConfig struct {
	MinGas  uint64
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
		cfg = &Config{
			TlsFlag: false,
			POOL: PoolConfig{
				MinGas:  3,
				MaxSize: 1_000_000, // bytes
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
				PID: "/vavilov/1.0.0",
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
	}
	return cfg
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
	var currentNodeAddress types.Address
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
			nodeK = types.DecodePrivKey(ppk)
			ppk = types.EncodePrivateKeyToToString(nodeK)
			currentNodeAddress = types.PubkeyToAddress(nodeK.PublicKey)
		} else {
			nodeK, _ = types.GenerateAccount()
			currentNodeAddress = types.PubkeyToAddress(nodeK.PublicKey)
			ppk = types.EncodePrivateKeyToToString(nodeK)
			err := os.WriteFile(pemFilePath, []byte(ppk), 0644)
			if err != nil {
				panic(err)
			}
		}
	}
	cfg.NetCfg.ADDR = currentNodeAddress
	cfg.NetCfg.PRIV = ppk
	cfg.NetCfg.PUB = types.EncodePublicKeyToByte(&nodeK.PublicKey)

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
	return &cfg, nil
}
