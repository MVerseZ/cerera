package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cerera/internal/cerera/block"
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

// managerTestBlock creates a block for manager tests.
func managerTestBlock(height int, hashSeed string) *block.Block {
	h := common.BytesToHash([]byte(hashSeed))
	return &block.Block{
		Head: &block.Header{Height: height},
		Hash: h,
	}
}

func setupManager(t *testing.T) (context.Context, context.CancelFunc, host.Host, *peers.Manager, *peers.Scorer, *Manager) {
	ctx, cancel := context.WithCancel(context.Background())
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	pm := peers.NewManager(ctx, h, 10)
	scorer := peers.NewScorer(pm)
	mgr := NewManager(ctx, h, pm, scorer, nil, types.Address{})
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
	assert.Equal(t, 0, mgr.GetValidatorCount())
	assert.Equal(t, 0, mgr.GetQuorum())

	mgr.AddValidator(pid)
	assert.True(t, mgr.IsValidator(pid))
	assert.Equal(t, 1, mgr.GetValidatorCount())
	assert.Equal(t, 1, mgr.GetQuorum())

	mgr.AddValidator(h.ID())
	assert.Equal(t, 2, mgr.GetValidatorCount())
	// quorum = 2f+1, f=(n-1)/3; n=2 => f=0 => quorum=1
	assert.Equal(t, 1, mgr.GetQuorum())

	mgr.RemoveValidator(pid)
	assert.False(t, mgr.IsValidator(pid))
	assert.True(t, mgr.IsValidator(h.ID()))
	assert.Equal(t, 1, mgr.GetValidatorCount())
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

	vc := message.ViewChange{NewViewID: 10, NodeID: 1, CMsg: map[int64]*message.CheckPoint{}, PMsg: map[int64]*message.PTuple{}}
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

	vc := message.ViewChange{NewViewID: 10, NodeID: 1, CMsg: map[int64]*message.CheckPoint{}, PMsg: map[int64]*message.PTuple{}}
	data, _ := json.Marshal(&vc)
	_ = mgr.HandleConsensusMessage(int(message.MTViewChange), data, peer.ID("v1"))
	assert.Equal(t, int64(10), mgr.GetCurrentView())

	vc2 := message.ViewChange{NewViewID: 5, NodeID: 1, CMsg: map[int64]*message.CheckPoint{}, PMsg: map[int64]*message.PTuple{}}
	data2, _ := json.Marshal(&vc2)
	err := mgr.HandleConsensusMessage(int(message.MTViewChange), data2, peer.ID("v1"))
	assert.NoError(t, err)
	// view must not decrease
	assert.Equal(t, int64(10), mgr.GetCurrentView())
}

func TestManager_ProposeBlock_nilValidator(t *testing.T) {
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

type mockValidator struct {
	powOk bool
	valOk bool
}

func (m *mockValidator) ValidateBlockPoW(*block.Block) bool { return m.powOk }
func (m *mockValidator) ValidateBlock(*block.Block) error {
	if m.valOk {
		return nil
	}
	return errors.New("validation failed")
}

func TestManager_ProposeBlock_withValidator_rejectPoW(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.SetBlockValidator(&mockValidator{powOk: false, valOk: true})
	mgr.AddValidator(h.ID())
	mgr.Start()
	defer mgr.Stop()

	b := managerTestBlock(1, "block1")
	err := mgr.ProposeBlock(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PoW validation")
}

func TestManager_ProposeBlock_withValidator_rejectValidation(t *testing.T) {
	_, cancel, h, _, _, mgr := setupManager(t)
	defer cancel()

	mgr.SetBlockValidator(&mockValidator{powOk: true, valOk: false})
	mgr.AddValidator(h.ID())
	mgr.Start()
	defer mgr.Stop()

	b := managerTestBlock(1, "block1")
	err := mgr.ProposeBlock(b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestManager_SetOnBlockFinalized_SetChainProvider_SetHeightLockProvider_SetBlockValidator(t *testing.T) {
	_, cancel, _, _, _, mgr := setupManager(t)
	defer cancel()

	called := false
	mgr.SetOnBlockFinalized(func(*block.Block) { called = true })
	// run path that invokes it would need handleCommitQuorum; we only check setters
	_ = called

	mgr.SetChainProvider(nil)
	mgr.SetHeightLockProvider(nil)
	mgr.SetBlockValidator(nil)
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
	assert.Equal(t, 1, st.Quorum)
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
	// peer manager has no connected peers; only self is added
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
	// RequestViewChange uses it
	_ = mgr.RequestViewChange(1)
	assert.True(t, called)
}
