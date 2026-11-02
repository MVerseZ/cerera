package gigea

import (
	"testing"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/types"
)

func TestRingAndExecuteCtl(t *testing.T) {
	T.Counter = 3
	if Get().Counter != 3 {
		t.Fatal(Get().Counter)
	}
	var ring Ring
	ring.Execute()
	if ExecuteCtl(7) != 0 {
		t.Fatal("execute ctl")
	}
}

func TestConsensusStateMachine(t *testing.T) {
	addr := types.HexToAddress("0x11")
	other := types.HexToAddress("0x22")
	mu.Lock()
	C = Consensus{
		Nonce:  10,
		Status: 1,
		Voters: []types.Address{addr},
		Nodes:  make(map[string]*NodeInfo),
		Chain:  make(chan *block.Block, 2),
	}
	G = CSP{state: Follower, address: &addr}
	mu.Unlock()

	if GetConsensusState() != Follower || GetConsensusStatus() != 1 || GetNonce() != 10 {
		t.Fatal("initial consensus snapshot")
	}
	for _, state := range []ConsensusState{Follower, Candidate, Leader, Validator, Miner, Shutdown, 99} {
		if state.String() == "" {
			t.Fatal("empty state name")
		}
	}
	if ConsensusState(99).String() != "Unknown" {
		t.Fatal(ConsensusState(99).String())
	}

	SetConsensusState(Leader)
	SetStatus(4)
	if GetConsensusState() != Leader || GetConsensusStatus() != 4 {
		t.Fatal("state update")
	}
	SetConsensusStatus(2)
	if GetConsensusStatus() != 2 {
		t.Fatal(GetConsensusStatus())
	}

	if got := GetAndIncrementNonce(); got != 10 || GetNonce() != 11 {
		t.Fatalf("nonce %d then %d", got, GetNonce())
	}
	SetNonce(11)
	SetNonce(20)
	if GetNonce() != 20 {
		t.Fatal(GetNonce())
	}

	AddVoter(addr)
	AddVoter(other)
	voters := GetVoters()
	if len(voters) != 2 || voters[1] != other {
		t.Fatalf("voters %#v", voters)
	}

	AddNode(other, "10.0.0.2:4000")
	UpdateNodeLastSeen(other)
	UpdateNodeLastSeen(addr)
	nodes := GetNodes()
	if nodes[other.String()] == nil || !nodes[other.String()].IsConnected {
		t.Fatal("node missing")
	}
	RemoveNode(other)
	RemoveNode(addr)
	if GetNodes()[other.String()].IsConnected {
		t.Fatal("node still connected")
	}

	info := GetConsensusInfo()
	if info["nonce"] != uint64(20) || info["voters"] != 2 || info["status"] != 2 {
		t.Fatalf("%#v", info)
	}

	C.Notify(&block.Block{Head: &block.Header{Height: 1}})
	select {
	case <-C.Chain:
	default:
		t.Fatal("leader did not forward the block")
	}
	SetConsensusState(Miner)
	C.Notify(&block.Block{Head: &block.Header{Height: 2}})
	select {
	case <-C.Chain:
	default:
		t.Fatal("miner did not forward the block")
	}
	SetConsensusState(Follower)
	C.Notify(&block.Block{})
	select {
	case <-C.Chain:
		t.Fatal("follower forwarded a block")
	default:
	}

	updateConsensusMetrics()
}

func TestEngineStartAndListen(t *testing.T) {
	mu.Lock()
	C.Chain = nil
	mu.Unlock()

	e := &Engine{Port: 1}
	e.Start(types.Address{})
	if e.BlockFunnel == nil || e.stopCh == nil || e.Owner != (types.Address{}) {
		t.Fatal("engine was not started")
	}
	e.Register(nil)
	e.MaintainTicker.Stop()
	close(e.stopCh)
	time.Sleep(20 * time.Millisecond)

	stopped := &Engine{
		BlockFunnel: make(chan *block.Block, 1),
		stopCh:      make(chan struct{}),
	}
	stopped.BlockFunnel <- &block.Block{Head: &block.Header{Height: 1}}
	done := make(chan struct{})
	go func() {
		stopped.Listen()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	close(stopped.stopCh)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listen did not stop")
	}
}
