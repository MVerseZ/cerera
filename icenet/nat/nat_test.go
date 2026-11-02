package nat

import (
	"testing"

	"github.com/cerera/config"
)

func TestNATOptionsAndConfig(t *testing.T) {
	opts := GetNATOptions(&config.Config{})
	if len(opts) != 5 {
		t.Fatalf("options %d", len(opts))
	}
	withRelay := GetNATOptions(&config.Config{NetCfg: config.NetworkConfig{RelayNodes: []string{"/ip4/10.0.0.1/tcp/4001"}}})
	if len(withRelay) != len(opts) {
		t.Fatal("relay nodes should not change option count")
	}

	def := DefaultNATConfig()
	if def == nil || !def.EnableUPnP || !def.EnableHolePunching || !def.EnableRelay || def.ForceRelay {
		t.Fatalf("%+v", def)
	}
	if len(def.RelayNodes) != 0 {
		t.Fatal("default relays")
	}

	got := GetNATConfigFromConfig(&config.Config{NetCfg: config.NetworkConfig{RelayNodes: []string{"/ip4/10.0.0.2/tcp/1"}}})
	if len(got.RelayNodes) != 1 || got.RelayNodes[0] != "/ip4/10.0.0.2/tcp/1" {
		t.Fatalf("%+v", got)
	}
	empty := GetNATConfigFromConfig(&config.Config{})
	if len(empty.RelayNodes) != 0 || !empty.EnableRelay {
		t.Fatalf("%+v", empty)
	}
}
