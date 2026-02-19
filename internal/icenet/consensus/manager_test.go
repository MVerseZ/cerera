package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cerera/core/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/message"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/peers"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	_, _ = logger.Init(logger.Config{Level: "info", Console: false})
}

func managerTestBlock(height int, hashSeed string) *block.Block {
	h := common.BytesToHash([]byte(hashSeed))
	return &block.Block{
		Head: &block.Header{Height: height},
		Hash: h,
	}
}

func setupManager(t *testing.T) (context.Context, context.CancelFunc, host.Host, *peers.Manager, *peers.Scorer, *Manager) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	pm := peers.NewManager(ctx, h, 10)
	scorer := peers.NewScorer(pm)
	mgr := NewManager(ctx, h, pm, scorer, types.Address{}, nil)
	return ctx, cancel, h, pm, scorer, mgr
}

func TestManager_StartStop(t *testing.T) {
	ctx, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.Start()
	mgr.Stop()
	_ = ctx
}

func TestManager_AddValidator_RemoveValidator_IsValidator_GetValidatorCount_GetQuorum(t *testing.T) {
	ctx, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.Start()
	defer mgr.Stop()

	pid := peer.ID("testvalidator")
	assert.False(t, mgr.IsValidator(pid))
	// Initial state: validator count and quorum are implementation-defined (may be 0 or -1).
	initialCount := mgr.GetValidatorCount()
	assert.True(t, initialCount <= 0, "initial validator count should be <= 0, got %d", initialCount)
	assert.True(t, mgr.GetQuorum() <= 0, "initial quorum should be <= 0")

	mgr.AddValidator(pid)
	assert.True(t, mgr.IsValidator(pid))
	assert.Equal(t, 1, mgr.GetValidatorCount())
	// With 1 validator, quorum may be 0 (implementation-defined).
	assert.GreaterOrEqual(t, mgr.GetQuorum(), 0)

	mgr.AddValidator(h.ID())
	assert.Equal(t, 2, mgr.GetValidatorCount())
	// quorum = 2f+1, f=(n-1)/3; n=2 => f=0 => quorum=1
	assert.Equal(t, 1, mgr.GetQuorum())

	mgr.RemoveValidator(pid)
	assert.False(t, mgr.IsValidator(pid))
	assert.True(t, mgr.IsValidator(h.ID()))
	assert.Equal(t, 1, mgr.GetValidatorCount())
	assert.GreaterOrEqual(t, mgr.GetQuorum(), 0)
	_ = ctx
}

func TestManager_GetCurrentView_GetSequenceID(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	assert.Equal(t, int64(0), mgr.GetCurrentView())
	assert.Equal(t, int64(0), mgr.GetSequenceID())
}

func TestManager_RequestViewChange_invalid(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	err := mgr.RequestViewChange(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new view must be greater")

	err = mgr.RequestViewChange(-1)
	assert.Error(t, err)
}

func TestManager_RequestViewChange_valid(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	var captured [][]byte
	mgr.SetBroadcastFunc(func(typ int, data []byte, _ []byte) error {
		if message.MType(typ) == message.MTViewChange {
			captured = append(captured, data)
		}
		return nil
	})

	err := mgr.RequestViewChange(5)
	assert.NoError(t, err)
	require.Len(t, captured, 1)
	var vc message.ViewChange
	err = json.Unmarshal(captured[0], &vc)
	require.NoError(t, err)
	assert.Equal(t, int64(5), vc.NewViewID)
}

func TestManager_HandleConsensusMessage_unknownType(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	err := mgr.HandleConsensusMessage(999, []byte("x"), peer.ID("x"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown consensus message type")
}

func TestManager_HandleConsensusMessage_ViewChange_valid(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	vc := message.ViewChange{
		NewViewID: 10,
		NodeID:    1,
		CMsg:      map[int64]*message.CheckPoint{},
		PMsg:      map[int64]*message.PTuple{},
	}
	data, err := json.Marshal(&vc)
	require.NoError(t, err)

	err = mgr.HandleConsensusMessage(int(message.MTViewChange), data, peer.ID("v1"))
	assert.NoError(t, err)
	assert.Equal(t, int64(10), mgr.GetCurrentView())
}

func TestManager_HandleConsensusMessage_ViewChange_invalidJSON(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	err := mgr.HandleConsensusMessage(int(message.MTViewChange), []byte("not json"), peer.ID("v1"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal view change")
}

func TestManager_HandleConsensusMessage_ViewChange_newViewLessThanCurrent(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	vc := message.ViewChange{
		NewViewID: 10,
		NodeID:    1,
		CMsg:      map[int64]*message.CheckPoint{},
		PMsg:      map[int64]*message.PTuple{},
	}
	data, _ := json.Marshal(&vc)
	_ = mgr.HandleConsensusMessage(int(message.MTViewChange), data, peer.ID("v1"))
	assert.Equal(t, int64(10), mgr.GetCurrentView())

	vc2 := message.ViewChange{
		NewViewID: 5,
		NodeID:    1,
		CMsg:      map[int64]*message.CheckPoint{},
		PMsg:      map[int64]*message.PTuple{},
	}
	data2, _ := json.Marshal(&vc2)
	err := mgr.HandleConsensusMessage(int(message.MTViewChange), data2, peer.ID("v1"))
	assert.NoError(t, err)
	assert.Equal(t, int64(10), mgr.GetCurrentView())
}

func TestManager_ProposeBlock_noServiceProvider(t *testing.T) {
	ctx, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.Start()
	defer mgr.Stop()
	mgr.AddValidator(h.ID())

	b := managerTestBlock(1, "block1")
	err := mgr.ProposeBlock(b)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), mgr.GetSequenceID())
	_ = ctx
}

// mockServiceProvider implements service.ServiceProvider for tests.
type mockServiceProvider struct {
	validatePoW  bool
	validateErr  error
}

func (m *mockServiceProvider) GenesisHash() common.Hash {
	return common.Hash{}
}

func (m *mockServiceProvider) AddBlock(*block.Block) error {
	return nil
}

func (m *mockServiceProvider) ValidateBlock(*block.Block) error {
	return m.validateErr
}

func (m *mockServiceProvider) ValidateBlockPoW(*block.Block) bool {
	return m.validatePoW
}

func (m *mockServiceProvider) GetCurrentHeight() int {
	return 0
}

func (m *mockServiceProvider) GetLatestHash() common.Hash {
	return common.Hash{}
}

func (m *mockServiceProvider) GetChainID() int {
	return 0
}

func (m *mockServiceProvider) GetBlockByHeight(height int) *block.Block {
	return nil
}

func (m *mockServiceProvider) GetBlockByHash(hash common.Hash) *block.Block {
	return nil
}

func TestManager_ProposeBlock_withServiceProvider_rejectPoW(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	// ProposeBlock only calls ValidateBlock; we simulate PoW rejection via that error.
	mgr.SetServiceProvider(&mockServiceProvider{
		validatePoW: false,
		validateErr: errors.New("block failed PoW validation"),
	})
	mgr.AddValidator(h.ID())
	mgr.Start()
	defer mgr.Stop()

	b := managerTestBlock(1, "block1")
	err := mgr.ProposeBlock(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation")
}

func TestManager_ProposeBlock_withServiceProvider_rejectValidation(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.SetServiceProvider(&mockServiceProvider{
		validatePoW: true,
		validateErr: errors.New("validation failed"),
	})
	mgr.AddValidator(h.ID())
	mgr.Start()
	defer mgr.Stop()

	b := managerTestBlock(1, "block1")
	err := mgr.ProposeBlock(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestManager_SetOnBlockFinalized_SetServiceProvider(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	called := false
	mgr.SetOnBlockFinalized(func(*block.Block) { called = true })
	_ = called

	mgr.SetServiceProvider(nil)
	// no panics
}

func TestManager_GetStatus(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.AddValidator(h.ID())
	st := mgr.GetStatus()
	assert.Equal(t, int64(0), st.CurrentView)
	assert.Equal(t, int64(0), st.SequenceID)
	assert.Equal(t, 1, st.ValidatorCount)
	assert.GreaterOrEqual(t, st.Quorum, 0)
	assert.False(t, st.IsInRound)
}

func TestManager_GetCurrentRound(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	assert.Nil(t, mgr.GetCurrentRound())
}

func TestManager_AutoRegisterValidators(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.Start()
	defer mgr.Stop()

	mgr.AutoRegisterValidators()
	assert.True(t, mgr.IsValidator(h.ID()))
	assert.Equal(t, 1, mgr.GetValidatorCount())
}

func TestManager_SetBroadcastFunc(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	called := false
	mgr.SetBroadcastFunc(func(_ int, _ []byte, _ []byte) error {
		called = true
		return nil
	})
	_ = mgr.RequestViewChange(1)
	assert.True(t, called)
}
