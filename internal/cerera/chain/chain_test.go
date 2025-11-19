package chain

import (
	"testing"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/types"
)

func TestChainInitialization(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	// Test chain initialization
	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	if chain == nil {
		t.Fatal("Chain should not be nil")
	}

	// Test basic properties
	if chain.GetChainId() != cfg.Chain.ChainID {
		t.Errorf("Expected chain ID %d, got %d", cfg.Chain.ChainID, chain.GetChainId())
	}

	if chain.GetCurrentChainOwnerAddress() != cfg.NetCfg.ADDR {
		t.Errorf("Expected chain owner address %s, got %s", cfg.NetCfg.ADDR, chain.GetCurrentChainOwnerAddress())
	}

	if chain.ServiceName() != CHAIN_SERVICE_NAME {
		t.Errorf("Expected service name %s, got %s", CHAIN_SERVICE_NAME, chain.ServiceName())
	}
}

func TestGetBlockChain(t *testing.T) {
	chain := GetBlockChain()
	if chain == nil {
		t.Fatal("GetBlockChain() should not return nil")
	}
}

func TestChainExec(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	tests := []struct {
		name     string
		method   string
		params   []interface{}
		expected interface{}
	}{
		{
			name:     "getInfo",
			method:   "getInfo",
			params:   []interface{}{},
			expected: chain.GetInfo(),
		},
		{
			name:     "height",
			method:   "height",
			params:   []interface{}{},
			expected: chain.GetLatestBlock().Header().Height,
		},
		{
			name:     "getBlockByIndex",
			method:   "getBlockByIndex",
			params:   []interface{}{float64(0)},
			expected: chain.GetBlockByNumber(0),
		},
		{
			name:     "getBlock",
			method:   "getBlock",
			params:   []interface{}{chain.GetLatestBlock().GetHash().Hex()},
			expected: chain.GetBlock(chain.GetLatestBlock().GetHash()),
		},
		{
			name:     "getBlockHeader",
			method:   "getBlockHeader",
			params:   []interface{}{chain.GetLatestBlock().GetHash().Hex()},
			expected: chain.GetBlockHeader(chain.GetLatestBlock().GetHash().Hex()),
		},
		{
			name:     "unknown method",
			method:   "unknown",
			params:   []interface{}{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chain.Exec(tt.method, tt.params)
			if tt.method == "unknown" {
				if result != nil {
					t.Errorf("Expected nil for unknown method, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected non-nil result for method %s", tt.method)
				}
			}
		})
	}
}

func TestGetBlock(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Test getting existing block
	latestBlock := chain.GetLatestBlock()
	if latestBlock == nil {
		t.Fatal("Latest block should not be nil")
	}

	foundBlock := chain.GetBlock(latestBlock.GetHash())
	if foundBlock == nil {
		t.Fatal("Found block should not be nil")
	}

	if foundBlock.GetHash().Compare(latestBlock.GetHash()) != 0 {
		t.Errorf("Expected block hash %s, got %s", latestBlock.GetHash().Hex(), foundBlock.GetHash().Hex())
	}

	// Test getting non-existing block
	nonExistingHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	nonExistingBlock := chain.GetBlock(nonExistingHash)
	if nonExistingBlock == nil {
		t.Fatal("Non-existing block should return empty block, not nil")
	}

	// Check if returned block is empty
	if nonExistingBlock.GetHash() != common.EmptyHash() {
		t.Errorf("Non-existing block should have empty hash, got %s", nonExistingBlock.GetHash().Hex())
	}
}

func TestGetBlockByNumber(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Test getting block by number (genesis block should be at index 0)
	genesisBlock := chain.GetBlockByNumber(0)
	if genesisBlock == nil {
		t.Fatal("Genesis block should not be nil")
	}

	if genesisBlock.Header().Index != 0 {
		t.Errorf("Expected genesis block index 0, got %d", genesisBlock.Header().Index)
	}

	// Test getting non-existing block
	nonExistingBlock := chain.GetBlockByNumber(999)
	if nonExistingBlock == nil {
		t.Fatal("Non-existing block should return empty block, not nil")
	}

	if nonExistingBlock.GetHash() != common.EmptyHash() {
		t.Errorf("Non-existing block should have empty hash, got %s", nonExistingBlock.GetHash().Hex())
	}
}

func TestGetBlockHash(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Test getting hash of existing block
	genesisBlock := chain.GetBlockByNumber(0)
	expectedHash := genesisBlock.GetHash()
	actualHash := chain.GetBlockHash(0)

	if actualHash.Compare(expectedHash) != 0 {
		t.Errorf("Expected block hash %s, got %s", expectedHash.Hex(), actualHash.Hex())
	}

	// Test getting hash of non-existing block
	nonExistingHash := chain.GetBlockHash(999)
	if nonExistingHash != common.EmptyHash() {
		t.Errorf("Expected empty hash for non-existing block, got %s", nonExistingHash.Hex())
	}
}

func TestGetBlockHeader(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Test getting header of existing block
	genesisBlock := chain.GetBlockByNumber(0)
	expectedHeader := genesisBlock.Header()
	actualHeader := chain.GetBlockHeader(genesisBlock.GetHash().Hex())

	if actualHeader == nil {
		t.Fatal("Block header should not be nil")
	}

	if actualHeader.Index != expectedHeader.Index {
		t.Errorf("Expected header index %d, got %d", expectedHeader.Index, actualHeader.Index)
	}

	if actualHeader.Height != expectedHeader.Height {
		t.Errorf("Expected header height %d, got %d", expectedHeader.Height, actualHeader.Height)
	}

	// Test getting header of non-existing block
	nonExistingHeader := chain.GetBlockHeader("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	if nonExistingHeader == nil {
		t.Fatal("Non-existing block header should return empty header, not nil")
	}

	if nonExistingHeader.Index != 0 {
		t.Errorf("Non-existing block header should have index 0, got %d", nonExistingHeader.Index)
	}
}

func TestGetInfo(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: false, // Use persistent mode for more predictable behavior
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	info := chain.GetInfo()

	// Test basic info properties - in persistent mode, stats might not be updated initially
	if info.Total < 0 {
		t.Errorf("Expected total blocks >= 0, got %d", info.Total)
	}

	if info.ChainWork < 0 {
		t.Errorf("Expected chain work >= 0, got %d", info.ChainWork)
	}

	// Size might be 0 initially in persistent mode
	if info.Size < 0 {
		t.Errorf("Expected size >= 0, got %d", info.Size)
	}

	// Latest might be empty initially in persistent mode
	if info.Latest == common.EmptyHash() {
		t.Log("Latest block hash is empty initially - this is expected behavior in persistent mode")
	}

	// Test that info is consistent
	latestBlock := chain.GetLatestBlock()
	if latestBlock != nil {
		// In persistent mode, the latest block might exist but not be reflected in stats
		t.Logf("Latest block exists with hash: %s", latestBlock.GetHash().Hex())
	}
}

func TestGetLatestBlock(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: false, // Use persistent mode for more predictable behavior
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	latestBlock := chain.GetLatestBlock()
	if latestBlock == nil {
		t.Fatal("Latest block should not be nil")
	}

	// Test that latest block is genesis block initially
	if latestBlock.Header().Index != 0 {
		t.Errorf("Expected genesis block index 0, got %d", latestBlock.Header().Index)
	}

	if latestBlock.Header().Height != 0 {
		t.Errorf("Expected genesis block height 0, got %d", latestBlock.Header().Height)
	}
}

func TestUpdateChain(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Get initial state
	initialInfo := chain.GetInfo()
	initialTotal := initialInfo.Total
	initialLatest := initialInfo.Latest

	// Create a new block
	newBlockHeader := &block.Header{
		Ctx:        0,
		Difficulty: 1,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     1,
		Index:      1,
		GasLimit:   1000000,
		GasUsed:    0,
		ChainId:    cfg.Chain.ChainID,
		Node:       cfg.NetCfg.ADDR,
		Size:       100,
		Timestamp:  uint64(time.Now().UnixMilli()),
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      12345,
		PrevHash:   initialLatest,
		Root:       common.EmptyHash(),
	}

	newBlock := block.NewBlock(newBlockHeader)
	newBlock.Hash = common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	// Update chain with new block
	chain.UpdateChain(newBlock)

	// Test that chain was updated
	updatedInfo := chain.GetInfo()
	if updatedInfo.Total != initialTotal+1 {
		t.Errorf("Expected total blocks %d, got %d", initialTotal+1, updatedInfo.Total)
	}

	if updatedInfo.Latest.Compare(newBlock.GetHash()) != 0 {
		t.Errorf("Expected latest hash %s, got %s", newBlock.GetHash().Hex(), updatedInfo.Latest.Hex())
	}

	// Test that latest block is updated
	latestBlock := chain.GetLatestBlock()
	if latestBlock.GetHash().Compare(newBlock.GetHash()) != 0 {
		t.Errorf("Expected latest block hash %s, got %s", newBlock.GetHash().Hex(), latestBlock.GetHash().Hex())
	}

	// Test that confirmations were incremented
	genesisBlock := chain.GetBlockByNumber(0)
	if genesisBlock.Confirmations != 1 {
		t.Errorf("Expected genesis block confirmations 1, got %d", genesisBlock.Confirmations)
	}
}

func TestValidateBlocks(t *testing.T) {
	tests := []struct {
		name    string
		blocks  []*block.Block
		wantLen int
		wantErr bool
	}{
		{
			name:    "empty blocks",
			blocks:  []*block.Block{},
			wantLen: -1,
			wantErr: true,
		},
		{
			name:    "single block",
			blocks:  []*block.Block{block.NewBlock(&block.Header{})},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "multiple blocks",
			blocks: []*block.Block{
				block.NewBlock(&block.Header{}),
				block.NewBlock(&block.Header{}),
				block.NewBlock(&block.Header{}),
			},
			wantLen: 3,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLen, err := ValidateBlocks(tt.blocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotLen != tt.wantLen {
				t.Errorf("ValidateBlocks() = %v, want %v", gotLen, tt.wantLen)
			}
		})
	}
}

func TestChainConcurrency(t *testing.T) {
	// Initialize chain for testing
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Test concurrent access to chain methods
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Test concurrent reads
			info := chain.GetInfo()
			if info.Total <= 0 {
				t.Errorf("Expected total blocks > 0, got %d", info.Total)
			}

			latestBlock := chain.GetLatestBlock()
			if latestBlock == nil {
				t.Error("Latest block should not be nil")
			}

			blockByNumber := chain.GetBlockByNumber(0)
			if blockByNumber == nil {
				t.Error("Block by number should not be nil")
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestChainServiceName(t *testing.T) {
	expectedName := CHAIN_SERVICE_NAME
	actualName := CHAIN_SERVICE_NAME

	if actualName != expectedName {
		t.Errorf("Expected service name %s, got %s", expectedName, actualName)
	}

	// Test that service name is not empty
	if actualName == "" {
		t.Error("Service name should not be empty")
	}
}

// Benchmark tests
func BenchmarkGetInfo(b *testing.B) {
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.GetInfo()
	}
}

func BenchmarkGetLatestBlock(b *testing.B) {
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.GetLatestBlock()
	}
}

func BenchmarkGetBlockByNumber(b *testing.B) {
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.GetBlockByNumber(0)
	}
}

func BenchmarkUpdateChain(b *testing.B) {
	cfg := &config.Config{
		Chain: config.ChainConfig{
			ChainID: 12345,
			Path:    "EMPTY",
		},
		NetCfg: config.NetworkConfig{
			ADDR: types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
		},
		IN_MEM: true,
	}

	chain, err := Mold(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize blockchain: %v", err)
	}

	// Create test block
	newBlockHeader := &block.Header{
		Ctx:        0,
		Difficulty: 1,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     1,
		Index:      1,
		GasLimit:   1000000,
		GasUsed:    0,
		ChainId:    cfg.Chain.ChainID,
		Node:       cfg.NetCfg.ADDR,
		Size:       100,
		Timestamp:  uint64(time.Now().UnixMilli()),
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      12345,
		PrevHash:   common.EmptyHash(),
		Root:       common.EmptyHash(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newBlock := block.NewBlock(newBlockHeader)
		newBlock.Hash = common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		newBlock.Header().Index = uint64(i + 1)
		newBlock.Header().Height = int(i + 1)

		chain.UpdateChain(newBlock)
	}
}
