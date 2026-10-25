package coinbase

import (
	"testing"

	"github.com/cerera/core/types"
)

func TestBlockReward(t *testing.T) {
	want := BlockReward(0)
	got := BlockReward(0)
	if got.Cmp(want) != 0 {
		t.Fatalf("BlockReward() = %v, want %v", got, want)
	}

	got.SetUint64(0)
	if BlockReward(0).Cmp(want) != 0 {
		t.Fatalf("BlockReward() should return a copy of the reward amount")
	}
}

func TestCreateCoinBaseTransation(t *testing.T) {
	nonce := uint64(42)
	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	tx := CreateCoinBaseTransation(nonce, addr, 0)

	if tx.Nonce() != nonce {
		t.Fatalf("nonce mismatch: got %d want %d", tx.Nonce(), nonce)
	}

	if tx.To() == nil || *tx.To() != addr {
		t.Fatalf("to mismatch: got %v want %v", tx.To(), addr)
	}

	if tx.Value().Cmp(BlockReward(0)) != 0 {
		t.Fatalf("value mismatch: got %v want %v", tx.Value(), BlockReward(0))
	}
}
