package chain

import "github.com/cerera/core/block"

func blockTimestampSec(b *block.Block) int64 {
	if b == nil || b.Head == nil {
		return 0
	}
	return int64(b.Head.Timestamp / 1000)
}

// computeBlockTimeStats derives average inter-block time from header timestamps (ms → sec).
func computeBlockTimeStats(blocks []*block.Block) (avg float64, lastTs, intervals, totalTime int64) {
	if len(blocks) == 0 {
		return 0, 0, 0, 0
	}
	lastTs = blockTimestampSec(blocks[len(blocks)-1])
	if len(blocks) < 2 {
		return 0, lastTs, 0, 0
	}
	for i := 1; i < len(blocks); i++ {
		if blocks[i-1].Head != nil && blocks[i-1].Head.Height == 0 {
			continue // genesis anchor is not a mined block; skip genesis→block#1 interval
		}
		prev := blockTimestampSec(blocks[i-1])
		cur := blockTimestampSec(blocks[i])
		if cur <= prev {
			continue
		}
		totalTime += cur - prev
		intervals++
	}
	if intervals > 0 {
		avg = float64(totalTime) / float64(intervals)
	}
	return avg, lastTs, intervals, totalTime
}

func blockTimeStatsFromChain(blocks []*block.Block) ChainStats {
	avg, lastTs, intervals, totalTime := computeBlockTimeStats(blocks)
	return ChainStats{
		lastBlockTime: lastTs,
		blockCount:    intervals,
		totalTime:     totalTime,
		avgTime:       avg,
	}
}

func (bc *Chain) recalcBlockTimeStatsLocked() {
	avg, lastTs, intervals, totalTime := computeBlockTimeStats(bc.data)
	bc.stats.lastBlockTime = lastTs
	bc.stats.blockCount = intervals
	bc.stats.totalTime = totalTime
	bc.stats.avgTime = avg
	bc.info.AvgTime = avg
	if bc.metrics != nil {
		bc.metrics.avgBlockTimeSeconds.Set(avg)
	}
}
