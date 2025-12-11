package gigea

import (
	"context"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	err := Init(ctx, addr)
	require.NoError(t, err)

	// Проверяем, что консенсус инициализирован
	state := GetConsensusState()
	assert.Equal(t, Follower, state)

	// Проверяем, что адрес установлен
	info := GetConsensusInfo()
	assert.NotNil(t, info)
	assert.Equal(t, addr.String(), info["address"])
}

func TestGetConsensusState(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	state := GetConsensusState()
	assert.Equal(t, Follower, state)
}

func TestSetConsensusState(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	SetConsensusState(Leader)
	state := GetConsensusState()
	assert.Equal(t, Leader, state)

	SetConsensusState(Validator)
	state = GetConsensusState()
	assert.Equal(t, Validator, state)
}

func TestGetConsensusStatus(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	status := GetConsensusStatus()
	assert.Equal(t, 0, status)

	SetConsensusStatus(1)
	status = GetConsensusStatus()
	assert.Equal(t, 1, status)
}

func TestAddVoter(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr1)

	// Первый voter уже добавлен при Init
	voters := GetVoters()
	assert.Equal(t, 1, len(voters))
	assert.Equal(t, addr1, voters[0])

	// Добавляем второго voter
	addr2 := types.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	AddVoter(addr2)

	voters = GetVoters()
	assert.Equal(t, 2, len(voters))
	assert.Contains(t, voters, addr1)
	assert.Contains(t, voters, addr2)

	// Попытка добавить того же voter снова не должна добавить дубликат
	AddVoter(addr2)
	voters = GetVoters()
	assert.Equal(t, 2, len(voters))
}

func TestGetVoters(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	Init(ctx, addr1)

	addr2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	AddVoter(addr2)
	AddVoter(addr3)

	voters := GetVoters()
	assert.Equal(t, 3, len(voters))
	assert.Contains(t, voters, addr1)
	assert.Contains(t, voters, addr2)
	assert.Contains(t, voters, addr3)
}

func TestGetConsensusInfo(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	info := GetConsensusInfo()
	require.NotNil(t, info)

	assert.Equal(t, addr.String(), info["address"])
	assert.Equal(t, 1, info["voters"]) // Один voter после Init
	assert.Equal(t, 0, info["nodes"])
	assert.NotNil(t, info["status"])
	assert.NotNil(t, info["nonce"])
}

func TestAddNode(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr1)

	addr2 := types.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	networkAddr := "127.0.0.1:31000"

	AddNode(addr2, networkAddr)

	nodes := GetNodes()
	assert.Equal(t, 1, len(nodes))

	node, exists := nodes[addr2.String()]
	require.True(t, exists)
	assert.Equal(t, addr2, node.Address)
	assert.Equal(t, networkAddr, node.NetworkAddr)
	assert.True(t, node.IsConnected)
}

func TestGetNodes(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
	Init(ctx, addr1)

	addr2 := types.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := types.HexToAddress("0x3333333333333333333333333333333333333333")

	AddNode(addr2, "127.0.0.1:31001")
	AddNode(addr3, "127.0.0.1:31002")

	nodes := GetNodes()
	assert.Equal(t, 2, len(nodes))

	// Проверяем, что возвращается копия (не оригинал)
	nodes[addr2.String()].IsConnected = false
	nodes2 := GetNodes()
	node2, exists := nodes2[addr2.String()]
	require.True(t, exists)
	assert.True(t, node2.IsConnected) // Оригинал не изменился
}

func TestUpdateNodeLastSeen(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr1)

	addr2 := types.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	AddNode(addr2, "127.0.0.1:31000")

	nodes := GetNodes()
	node := nodes[addr2.String()]
	initialLastSeen := node.LastSeen

	// Ждем немного и обновляем
	time.Sleep(10 * time.Millisecond)
	UpdateNodeLastSeen(addr2)

	nodes = GetNodes()
	node = nodes[addr2.String()]
	assert.Greater(t, node.LastSeen, initialLastSeen)
	assert.True(t, node.IsConnected)
}

func TestRemoveNode(t *testing.T) {
	ctx := context.Background()
	addr1 := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr1)

	addr2 := types.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	AddNode(addr2, "127.0.0.1:31000")

	nodes := GetNodes()
	node := nodes[addr2.String()]
	assert.True(t, node.IsConnected)

	RemoveNode(addr2)

	nodes = GetNodes()
	node = nodes[addr2.String()]
	assert.False(t, node.IsConnected)
}

func TestGetAndIncrementNonce(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	initialNonce := GetNonce()

	nonce1 := GetAndIncrementNonce()
	assert.Equal(t, initialNonce, nonce1)

	nonce2 := GetAndIncrementNonce()
	assert.Equal(t, initialNonce+1, nonce2)

	currentNonce := GetNonce()
	assert.Equal(t, initialNonce+2, currentNonce)
}

func TestGetNonce(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	nonce1 := GetNonce()
	nonce2 := GetNonce()

	// GetNonce не должен изменять nonce
	assert.Equal(t, nonce1, nonce2)
}

func TestSetNonce(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	SetNonce(999)
	nonce := GetNonce()
	assert.Equal(t, uint64(999), nonce)

	SetNonce(1000)
	nonce = GetNonce()
	assert.Equal(t, uint64(1000), nonce)
}

func TestConsensusNotify(t *testing.T) {
	ctx := context.Background()
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	Init(ctx, addr)

	// Создаем тестовый блок
	header := &block.Header{
		Height:     1,
		Index:      1,
		Difficulty: 1,
		ChainId:    11,
		Nonce:      12345,
		PrevHash:   common.Hash{},
		Root:       common.Hash{},
		GasLimit:   1000000,
		GasUsed:    0,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}
	b := block.NewBlock(header)

	// Устанавливаем состояние Leader для тестирования Notify
	SetConsensusState(Leader)

	// Notify не должен паниковать
	consensus := GetConsensus()
	consensus.Notify(b)

	// Проверяем, что блок был отправлен в канал (если есть получатель)
	// В реальном сценарии это будет обработано другой горутиной
}

func TestConsensusStateString(t *testing.T) {
	tests := []struct {
		state    ConsensusState
		expected string
	}{
		{Follower, "Follower"},
		{Candidate, "Candidate"},
		{Leader, "Leader"},
		{Validator, "Validator"},
		{Miner, "Miner"},
		{Shutdown, "Shutdown"},
		{ConsensusState(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// Helper function to get consensus instance for testing
func GetConsensus() *Consensus {
	mu.RLock()
	defer mu.RUnlock()
	return &C
}
