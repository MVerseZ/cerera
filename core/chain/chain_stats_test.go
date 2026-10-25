package chain

import (
	"testing"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
)

func makeBlockWithTimestamp(height int, tsMillis uint64, prev common.Hash) *block.Block {
	head := &block.Header{
		Height:     height,
		Index:      uint64(height),
		Timestamp:  tsMillis,
		PrevHash:   prev,
		Difficulty: 1,
		GasLimit:   1_000_000,
		ChainId:    1,
	}
	return block.NewBlockWithHeaderAndHash(head)
}

func TestComputeBlockTimeStats(t *testing.T) {
	genesis := makeBlockWithTimestamp(0, 1_000_000, common.Hash{})
	b1 := makeBlockWithTimestamp(1, 1_003_000, genesis.Hash) // +3s from genesis (ignored)
	b2 := makeBlockWithTimestamp(2, 1_007_000, b1.Hash)      // +4s from b1

	avg, lastTs, intervals, total := computeBlockTimeStats([]*block.Block{genesis, b1, b2})
	if intervals != 1 {
		t.Fatalf("intervals = %d, want 1 (genesis→b1 excluded)", intervals)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	if avg != 4 {
		t.Fatalf("avg = %v, want 4", avg)
	}
	if lastTs != 1007 {
		t.Fatalf("lastTs = %d, want 1007", lastTs)
	}
}

func TestComputeBlockTimeStats_OnlyGenesisAndFirstBlock(t *testing.T) {
	genesis := makeBlockWithTimestamp(0, 1_000_000, common.Hash{})
	b1 := makeBlockWithTimestamp(1, 9_000_000_000, genesis.Hash)

	avg, _, intervals, total := computeBlockTimeStats([]*block.Block{genesis, b1})
	if intervals != 0 || total != 0 || avg != 0 {
		t.Fatalf("expected no intervals after genesis, got avg=%v intervals=%d total=%d", avg, intervals, total)
	}
}

func TestComputeBlockTimeStats_SingleBlock(t *testing.T) {
	genesis := makeBlockWithTimestamp(0, 5_000, common.Hash{})
	avg, lastTs, intervals, total := computeBlockTimeStats([]*block.Block{genesis})
	if avg != 0 || intervals != 0 || total != 0 || lastTs != 5 {
		t.Fatalf("unexpected stats avg=%v intervals=%d total=%d lastTs=%d", avg, intervals, total, lastTs)
	}
}

func TestComputeBlockTimeStats_SkipsNonMonotonicTimestamps(t *testing.T) {
	genesis := makeBlockWithTimestamp(0, 10_000, common.Hash{})
	b1 := makeBlockWithTimestamp(1, 10_000, genesis.Hash)
	b2 := makeBlockWithTimestamp(2, 15_000, b1.Hash)

	avg, _, intervals, total := computeBlockTimeStats([]*block.Block{genesis, b1, b2})
	if intervals != 1 || total != 5 || avg != 5 {
		t.Fatalf("avg=%v intervals=%d total=%d", avg, intervals, total)
	}
}