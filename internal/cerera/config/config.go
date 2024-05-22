package config

import (
	"crypto/ecdsa"
	"math/big"
	"os"

	"github.com/cerera/internal/cerera/types"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const DefaultP2pPort = int(6116)
const DefaultRpcPort = int(1337)

var ChainId = big.NewInt(133707331)

type ChainConfig struct {
	ChainID *big.Int
	Path    string
	Type    string
}
type NetworkConfig struct {
	PID  protocol.ID
	P2P  int
	RPC  int
	ADDR types.Address    // address of running node
	PRIV string           // private key of current running node
	PUB  *ecdsa.PublicKey // public key of current running node
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
type Config struct {
	Chain   ChainConfig // chain config
	TlsFlag bool
	NetCfg  NetworkConfig // network config (p2p, inner address, keys)
	POOL    PoolConfig    // pool config
	SEC     Sec
	AUTOGEN bool // auto generating blocks
}

func GenerageConfig() *Config {
	return &Config{
		TlsFlag: false,
		POOL: PoolConfig{
			MinGas:  3,
			MaxSize: 1000,
		},
		SEC: Sec{
			HTTP: HttpSecConfig{
				TLS: false,
			},
		},
		NetCfg: NetworkConfig{
			PID: "/vavilov/1.0.0",
		},
	}
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
	cfg.NetCfg.PUB = &nodeK.PublicKey

	cfg.Chain.ChainID = ChainId
	cfg.Chain.Path = "dat.db"
}
func (cfg *Config) SetAutoGen(f bool) {
	if !cfg.AUTOGEN {
		cfg.AUTOGEN = false
	}
	cfg.AUTOGEN = f
}
