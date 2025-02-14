package config

// import (
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// )

// func TestGenerateConfig(t *testing.T) {
// 	cfg := GenerageConfig()

// 	assert.NotNil(t, cfg)
// 	assert.False(t, cfg.TlsFlag)
// 	assert.Equal(t, uint64(3), cfg.POOL.MinGas)
// 	assert.Equal(t, 1000, cfg.POOL.MaxSize)
// 	assert.True(t, cfg.Vault.MEM)
// 	assert.Equal(t, "", cfg.Vault.PATH)
// 	assert.False(t, cfg.SEC.HTTP.TLS)
// 	assert.Equal(t, "/vavilov/1.0.0", string(cfg.NetCfg.PID))
// 	assert.Equal(t, "ALPHA", cfg.VERSION)
// 	assert.Equal(t, 1, cfg.VER)
// }

// func TestSetPorts(t *testing.T) {
// 	cfg := &Config{}
// 	cfg.SetPorts(8080, 30303)
// 	assert.Equal(t, 8080, cfg.NetCfg.RPC)
// 	assert.Equal(t, 30303, cfg.NetCfg.P2P)

// 	cfg.SetPorts(-1, -1)
// 	assert.Equal(t, DefaultRpcPort, cfg.NetCfg.RPC)
// 	assert.Equal(t, DefaultP2pPort, cfg.NetCfg.P2P)
// }

// func TestSetAutoGen(t *testing.T) {
// 	cfg := &Config{}
// 	cfg.SetAutoGen(true)
// 	assert.True(t, cfg.AUTOGEN)

// 	cfg.SetAutoGen(false)
// 	assert.False(t, cfg.AUTOGEN)
// }

// func TestCheckVersion(t *testing.T) {
// 	cfg := &Config{VERSION: "ALPHA", VER: 1}
// 	assert.True(t, cfg.CheckVersion("ALPHA", 1))
// 	assert.False(t, cfg.CheckVersion("BETA", 2))
// }

// func TestGetVersion(t *testing.T) {
// 	cfg := &Config{VERSION: "ALPHA", VER: 1}
// 	version := cfg.GetVersion()
// 	assert.Equal(t, "ALPHA-1_VERSION", version)
// }

// func TestUpdateVaultPath(t *testing.T) {
// 	cfg := &Config{}
// 	cfg.UpdateVaultPath("/new/path")
// 	assert.Equal(t, "/new/path", cfg.Vault.PATH)
// }
