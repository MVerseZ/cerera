package miner

import (
	"testing"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
)

type mockPeerPublisher struct {
	peerCount              int
	bootstrapPhaseDone     bool
	bootstrapConnectFailed bool
	bootstrapDone          chan struct{}
}

func (m *mockPeerPublisher) GetPeerCount() int { return m.peerCount }

func (m *mockPeerPublisher) ProposeBlock(_ *block.Block) error { return nil }

func (m *mockPeerPublisher) BootstrapPhaseDone() bool { return m.bootstrapPhaseDone }

func (m *mockPeerPublisher) BootstrapConnectFailed() bool { return m.bootstrapConnectFailed }

func (m *mockPeerPublisher) BootstrapDone() <-chan struct{} { return m.bootstrapDone }

func TestShouldWaitForPeers_BootstrapFailedStartsSolo(t *testing.T) {
	done := make(chan struct{})
	close(done)

	m := &miner{
		config: &config.Config{
			NetCfg: config.NetworkConfig{
				SeedNodes: []string{"/ip4/127.0.0.1/tcp/31100"},
			},
		},
		publisher: &mockPeerPublisher{
			peerCount:              0,
			bootstrapPhaseDone:     true,
			bootstrapConnectFailed: true,
			bootstrapDone:          done,
		},
	}

	if m.shouldWaitForPeers() {
		t.Fatal("should not wait when bootstrap connect failed")
	}
}

func TestShouldWaitForPeers_WaitsWhileBootstrapPending(t *testing.T) {
	m := &miner{
		config: &config.Config{
			NetCfg: config.NetworkConfig{
				BootstrapNodes: []string{"/ip4/127.0.0.1/tcp/31100/p2p/QmTest"},
			},
		},
		publisher: &mockPeerPublisher{
			peerCount:              0,
			bootstrapPhaseDone:     false,
			bootstrapConnectFailed: false,
			bootstrapDone:          make(chan struct{}),
		},
	}

	if !m.shouldWaitForPeers() {
		t.Fatal("should wait while bootstrap phase is pending and peers are configured")
	}
}

func TestShouldWaitForPeers_NoConfiguredPeers(t *testing.T) {
	m := &miner{
		config:    &config.Config{},
		publisher: &mockPeerPublisher{},
	}
	if m.shouldWaitForPeers() {
		t.Fatal("should not wait when no bootstrap peers configured")
	}
}
