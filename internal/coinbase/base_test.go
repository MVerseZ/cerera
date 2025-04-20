package coinbase

import (
	"testing"
)

func TestReward(t *testing.T) {
	InitOperationData()
	if blockReward.Cmp(RewardBlock()) != 0 {
		t.Errorf("expected %v, got %v\n", blockReward, RewardBlock())
	}
}
