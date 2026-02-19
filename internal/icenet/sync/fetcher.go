package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/internal/icenet/protocol"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// DefaultBatchSize is the default number of blocks to request at once
	DefaultBatchSize = 100
	// MaxBatchSize is the maximum number of blocks to request at once
	MaxBatchSize = 500
	// MinBatchSize is the minimum number of blocks to request at once
	MinBatchSize = 10
	// FetchTimeout is the timeout for fetching blocks
	FetchTimeout = 60 * time.Second
	// MaxRetries is the maximum number of retries for fetching blocks
	MaxRetries = 3
	// MaxConcurrentFetches is the maximum number of concurrent fetch operations
	MaxConcurrentFetches = 3
)

// FetchTask represents a block fetch task
type FetchTask struct {
	StartHeight int
	Count       int
	PeerID      peer.ID
	Retries     int
}

// FetchResult represents the result of a block fetch
type FetchResult struct {
	Task    *FetchTask
	Blocks  []*block.Block
	Error   error
	Latency time.Duration
}

// Fetcher handles parallel block fetching from multiple peers
type Fetcher struct {
	host        host.Host
	handler     *protocol.Handler
	peerTracker *PeerSyncTracker
	batchSize   int
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc

	// Channel for fetch results
	resultChan chan *FetchResult
}

// NewFetcher creates a new block fetcher
func NewFetcher(ctx context.Context, h host.Host, handler *protocol.Handler, peerTracker *PeerSyncTracker) *Fetcher {
	ctx, cancel := context.WithCancel(ctx)
	return &Fetcher{
		host:        h,
		handler:     handler,
		peerTracker: peerTracker,
		batchSize:   DefaultBatchSize,
		ctx:         ctx,
		cancel:      cancel,
		resultChan:  make(chan *FetchResult, MaxConcurrentFetches*2),
	}
}

// SetBatchSize sets the batch size for fetching blocks
func (f *Fetcher) SetBatchSize(size int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if size < MinBatchSize {
		size = MinBatchSize
	} else if size > MaxBatchSize {
		size = MaxBatchSize
	}
	f.batchSize = size
}

// GetBatchSize returns the current batch size
func (f *Fetcher) GetBatchSize() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.batchSize
}

// FetchBlocks fetches blocks from startHeight to endHeight from multiple peers
func (f *Fetcher) FetchBlocks(startHeight, endHeight int) ([]*block.Block, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("invalid range: start %d > end %d", startHeight, endHeight)
	}

	totalBlocks := endHeight - startHeight + 1
	allBlocks := make([]*block.Block, 0, totalBlocks)
	blockMap := make(map[int]*block.Block)
	var mapMu sync.Mutex

	// Get available peers
	peers := f.peerTracker.GetBestPeers(endHeight)
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers available for syncing")
	}

	// Create tasks
	tasks := f.createTasks(startHeight, endHeight, peers)
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks created")
	}

	// Create work queue
	taskChan := make(chan *FetchTask, len(tasks))
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Create result channel
	resultChan := make(chan *FetchResult, len(tasks))
	var wg sync.WaitGroup

	// Start workers
	numWorkers := MaxConcurrentFetches
	if numWorkers > len(peers) {
		numWorkers = len(peers)
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				result := f.fetchTask(task)
				resultChan <- result
			}
		}()
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var fetchErr error
	for result := range resultChan {
		if result.Error != nil {
			syncLogger().Warnw("Fetch task failed",
				"startHeight", result.Task.StartHeight,
				"peer", result.Task.PeerID,
				"error", result.Error,
			)
			f.peerTracker.RecordFailure(result.Task.PeerID)
			if fetchErr == nil {
				fetchErr = result.Error
			}
			continue
		}

		// Record success
		f.peerTracker.RecordBlocksReceived(result.Task.PeerID, len(result.Blocks))
		f.peerTracker.RecordLatency(result.Task.PeerID, result.Latency)

		// Add blocks to map
		mapMu.Lock()
		for _, b := range result.Blocks {
			if b != nil && b.Head != nil {
				blockMap[b.Head.Height] = b
			}
		}
		mapMu.Unlock()

		syncLogger().Debugw("Fetch task completed",
			"startHeight", result.Task.StartHeight,
			"blocksReceived", len(result.Blocks),
			"latency", result.Latency,
		)
	}

	// Build ordered block list
	for height := startHeight; height <= endHeight; height++ {
		if b, exists := blockMap[height]; exists {
			allBlocks = append(allBlocks, b)
		} else {
			// Missing block
			syncLogger().Warnw("Missing block in fetch result", "height", height)
		}
	}

	if len(allBlocks) == 0 && fetchErr != nil {
		return nil, fetchErr
	}

	return allBlocks, nil
}

// createTasks creates fetch tasks distributed across available peers
func (f *Fetcher) createTasks(startHeight, endHeight int, peers []*PeerSyncState) []*FetchTask {
	tasks := make([]*FetchTask, 0)
	batchSize := f.GetBatchSize()

	peerIndex := 0
	for height := startHeight; height <= endHeight; height += batchSize {
		count := batchSize
		if height+count > endHeight+1 {
			count = endHeight - height + 1
		}

		// Select peer for this task (round-robin)
		peerID := peers[peerIndex%len(peers)].PeerID
		peerIndex++

		tasks = append(tasks, &FetchTask{
			StartHeight: height,
			Count:       count,
			PeerID:      peerID,
			Retries:     0,
		})
	}

	return tasks
}

// fetchTask executes a single fetch task
func (f *Fetcher) fetchTask(task *FetchTask) *FetchResult {
	start := time.Now()
	result := &FetchResult{
		Task: task,
	}

	ctx, cancel := context.WithTimeout(f.ctx, FetchTimeout)
	defer cancel()

	blocks, err := f.handler.RequestBlocks(ctx, task.PeerID, task.StartHeight, task.Count)
	result.Latency = time.Since(start)

	if err != nil {
		result.Error = err
		return result
	}

	result.Blocks = blocks
	return result
}

// FetchBlocksWithRetry fetches blocks with automatic retry on failure
func (f *Fetcher) FetchBlocksWithRetry(startHeight, endHeight int) ([]*block.Block, error) {
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-f.ctx.Done():
				return nil, f.ctx.Err()
			case <-time.After(backoff):
			}
		}

		blocks, err := f.FetchBlocks(startHeight, endHeight)
		if err == nil && len(blocks) > 0 {
			return blocks, nil
		}

		lastErr = err
		syncLogger().Warnw("Fetch attempt failed",
			"attempt", attempt+1,
			"maxRetries", MaxRetries,
			"error", err,
		)
	}

	return nil, fmt.Errorf("failed after %d retries: %w", MaxRetries, lastErr)
}

// FetchSingleBlock fetches a single block by height
func (f *Fetcher) FetchSingleBlock(height int, peerID peer.ID) (*block.Block, error) {
	ctx, cancel := context.WithTimeout(f.ctx, FetchTimeout)
	defer cancel()

	blocks, err := f.handler.RequestBlocks(ctx, peerID, height, 1)
	if err != nil {
		return nil, err
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no block returned for height %d", height)
	}

	return blocks[0], nil
}

// Stop stops the fetcher
func (f *Fetcher) Stop() {
	f.cancel()
}

// AdaptiveBatchSize adjusts batch size based on network conditions
func (f *Fetcher) AdaptiveBatchSize(latency time.Duration, successRate float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Adjust based on latency
	if latency < 500*time.Millisecond && successRate > 0.9 {
		// Fast network with high success rate - increase batch size
		f.batchSize = min(f.batchSize+10, MaxBatchSize)
	} else if latency > 2*time.Second || successRate < 0.5 {
		// Slow network or low success rate - decrease batch size
		f.batchSize = max(f.batchSize-10, MinBatchSize)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
