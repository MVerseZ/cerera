package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/icenet/peers"
	"github.com/cerera/internal/icenet/protocol"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// SyncCheckInterval is the interval between sync checks
	SyncCheckInterval = 30 * time.Second
	// SyncTimeout is the timeout for a sync operation
	SyncTimeout = 5 * time.Minute
	// MinBlocksAhead is the minimum number of blocks a peer must be ahead to trigger sync
	MinBlocksAhead = 5
)

func syncLogger() *zap.SugaredLogger {
	return logger.Named("sync")
}

// ChainProvider provides access to blockchain data
type ChainProvider interface {
	GetCurrentHeight() int
	GetBlockByHeight(height int) *block.Block
	GetBlockByHash(hash common.Hash) *block.Block
	GetBestHash() common.Hash
	GetGenesisHash() common.Hash
	AddBlock(b *block.Block) error
	GetChainID() int
	ValidateBlock(b *block.Block) error
}

// Manager manages block synchronization with peers
type Manager struct {
	host            host.Host
	handler         *protocol.Handler
	peerManager     *peers.Manager
	serviceProvider service.ServiceProvider
	progress        *SyncProgress
	peerTracker     *PeerSyncTracker
	fetcher         *Fetcher

	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	isSyncing bool
	syncPeer  peer.ID

	// Callbacks
	onSyncComplete func()
	onNewBlock     func(*block.Block)
}

// NewManager creates a new sync manager
func NewManager(
	ctx context.Context,
	h host.Host,
	handler *protocol.Handler,
	peerManager *peers.Manager,
	provider service.ServiceProvider,
) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	peerTracker := NewPeerSyncTracker()

	m := &Manager{
		host:            h,
		handler:         handler,
		peerManager:     peerManager,
		serviceProvider: provider,
		progress:        NewSyncProgress(),
		peerTracker:     peerTracker,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Create fetcher
	m.fetcher = NewFetcher(ctx, h, handler, peerTracker)

	return m
}

// Start starts the sync manager
func (m *Manager) Start() {
	go m.syncLoop()
	syncLogger().Infow("Sync manager started")
}

// Stop stops the sync manager
func (m *Manager) Stop() {
	m.cancel()
	m.fetcher.Stop()
	syncLogger().Infow("Sync manager stopped")
}

// syncLoop periodically checks for sync opportunities
func (m *Manager) syncLoop() {
	ticker := time.NewTicker(SyncCheckInterval)
	defer ticker.Stop()

	// Initial sync check
	m.checkAndSync()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAndSync()
		}
	}
}

// checkAndSync checks if sync is needed and starts it
func (m *Manager) checkAndSync() {
	m.mu.Lock()
	if m.isSyncing {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// Get current height
	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	// Find best peer
	bestPeer := m.peerManager.GetBestPeer()
	if bestPeer == nil {
		return
	}

	// Update peer tracker
	m.peerTracker.UpdatePeer(bestPeer.ID, bestPeer.Height, common.Hash{})

	// Check if sync is needed
	if bestPeer.Height <= currentHeight+MinBlocksAhead {
		return
	}

	syncLogger().Infow("Sync needed",
		"currentHeight", currentHeight,
		"peerHeight", bestPeer.Height,
		"peer", bestPeer.ID,
	)

	// Start sync
	go m.syncWithPeer(bestPeer.ID, currentHeight, bestPeer.Height)
}

// syncWithPeer synchronizes blocks with a specific peer
func (m *Manager) syncWithPeer(peerID peer.ID, startHeight, targetHeight int) {
	m.mu.Lock()
	if m.isSyncing {
		m.mu.Unlock()
		return
	}
	m.isSyncing = true
	m.syncPeer = peerID
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isSyncing = false
		m.syncPeer = ""
		m.mu.Unlock()
	}()

	// Start progress tracking
	m.progress.StartSync(startHeight, targetHeight, peerID)

	syncLogger().Infow("Starting sync",
		"peer", peerID,
		"startHeight", startHeight,
		"targetHeight", targetHeight,
	)

	// Fetch and process blocks in batches
	currentHeight := startHeight
	for currentHeight < targetHeight {
		select {
		case <-m.ctx.Done():
			syncLogger().Infow("Sync cancelled")
			return
		default:
		}

		// Calculate batch end
		batchEnd := currentHeight + m.fetcher.GetBatchSize()
		if batchEnd > targetHeight {
			batchEnd = targetHeight
		}

		m.progress.SetState(StateDownloading)

		// Fetch blocks
		blocks, err := m.fetcher.FetchBlocksWithRetry(currentHeight+1, batchEnd)
		if err != nil {
			syncLogger().Errorw("Failed to fetch blocks",
				"startHeight", currentHeight+1,
				"endHeight", batchEnd,
				"error", err,
			)
			m.progress.RecordError(err.Error())
			// Try with smaller batch
			m.fetcher.AdaptiveBatchSize(5*time.Second, 0.3)
			continue
		}

		if len(blocks) == 0 {
			syncLogger().Warnw("No blocks received", "startHeight", currentHeight+1)
			break
		}

		m.progress.SetState(StateValidating)

		// Process blocks
		processedCount := 0
		for _, b := range blocks {
			if b == nil {
				continue
			}

			// Validate block
			if m.serviceProvider != nil {
				if err := m.serviceProvider.ValidateBlock(b); err != nil {
					syncLogger().Warnw("Block validation failed",
						"height", b.Head.Height,
						"hash", b.Hash,
						"error", err,
					)
					m.progress.RecordError(fmt.Sprintf("validation failed for block %d: %s", b.Head.Height, err))
					continue
				}

				// Add block to chain
				if err := m.serviceProvider.AddBlock(b); err != nil {
					syncLogger().Warnw("Failed to add block",
						"height", b.Head.Height,
						"hash", b.Hash,
						"error", err,
					)
					m.progress.RecordError(fmt.Sprintf("failed to add block %d: %s", b.Head.Height, err))
					continue
				}
			}

			processedCount++
			currentHeight = b.Head.Height

			// Call callback
			if m.onNewBlock != nil {
				m.onNewBlock(b)
			}
		}

		// Update progress
		m.progress.UpdateProgress(currentHeight, len(blocks), processedCount, 0)
		m.progress.SetLastBlockHash(blocks[len(blocks)-1].Hash)

		syncLogger().Debugw("Batch processed",
			"currentHeight", currentHeight,
			"targetHeight", targetHeight,
			"processedCount", processedCount,
			"progress", fmt.Sprintf("%.2f%%", m.progress.Percentage()),
		)
	}

	// Complete sync
	m.progress.Complete()

	syncLogger().Infow("Sync completed",
		"finalHeight", currentHeight,
		"blocksProcessed", m.progress.BlocksProcessed,
		"duration", time.Since(m.progress.StartTime),
	)

	// Call callback
	if m.onSyncComplete != nil {
		m.onSyncComplete()
	}
}

// HandleNewPeer handles a newly connected peer
func (m *Manager) HandleNewPeer(peerID peer.ID) {
	// Request status from the peer
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	status, err := m.handler.RequestStatus(ctx, peerID)
	if err != nil {
		syncLogger().Warnw("Failed to get peer status", "peer", peerID, "error", err)
		return
	}

	// Update peer info
	m.peerManager.UpdatePeerInfo(peerID, status.Height, status.LatestHash.String(), status.NodeVersion, status.NodeAddress)
	m.peerTracker.UpdatePeer(peerID, status.Height, status.LatestHash)

	syncLogger().Infow("[SYNC] Peer status received",
		"peer", peerID,
		"height", status.Height,
		"version", status.NodeVersion,
		"address", status.NodeAddress,
	)

	// Check if we need to sync
	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	if status.Height > currentHeight+MinBlocksAhead {
		go m.syncWithPeer(peerID, currentHeight, status.Height)
	}
}

// HandleNewBlock handles a newly received block
func (m *Manager) HandleNewBlock(b *block.Block, fromPeer peer.ID) error {
	if b == nil || b.Head == nil {
		return fmt.Errorf("invalid block")
	}

	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	// Skip if we already have a block at this height or higher
	if b.Head.Height <= currentHeight {
		syncLogger().Debugw("Skipping block - already have block at this height",
			"receivedHeight", b.Head.Height,
			"currentHeight", currentHeight,
			"from", fromPeer,
		)
		return nil
	}

	// Check if this is the next expected block
	if b.Head.Height == currentHeight+1 {
		// Validate and add
		if m.serviceProvider != nil {
			if err := m.serviceProvider.ValidateBlock(b); err != nil {
				return fmt.Errorf("block validation failed: %w", err)
			}

			if err := m.serviceProvider.AddBlock(b); err != nil {
				return fmt.Errorf("failed to add block: %w", err)
			}
		}

		syncLogger().Infow("New block added",
			"height", b.Head.Height,
			"hash", b.Hash,
			"from", fromPeer,
		)

		// Call callback
		if m.onNewBlock != nil {
			m.onNewBlock(b)
		}

		return nil
	}

	// Block is ahead - we might need to sync
	if b.Head.Height > currentHeight+MinBlocksAhead {
		go m.syncWithPeer(fromPeer, currentHeight, b.Head.Height)
	}

	return nil
}

// GetProgress returns the current sync progress
func (m *Manager) GetProgress() SyncProgress {
	return m.progress.GetProgress()
}

// IsSyncing returns true if sync is in progress
func (m *Manager) IsSyncing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isSyncing
}

// GetSyncPeer returns the current sync peer
func (m *Manager) GetSyncPeer() peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncPeer
}

// SetOnSyncComplete sets the callback for sync completion
func (m *Manager) SetOnSyncComplete(callback func()) {
	m.onSyncComplete = callback
}

// SetOnNewBlock sets the callback for new blocks
func (m *Manager) SetOnNewBlock(callback func(*block.Block)) {
	m.onNewBlock = callback
}

// ForceSync forces a sync with the best available peer
func (m *Manager) ForceSync() error {
	bestPeer := m.peerManager.GetBestPeer()
	if bestPeer == nil {
		return fmt.Errorf("no peers available")
	}

	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	if bestPeer.Height <= currentHeight {
		return fmt.Errorf("already at best height")
	}

	go m.syncWithPeer(bestPeer.ID, currentHeight, bestPeer.Height)
	return nil
}

// GetPeerTracker returns the peer sync tracker
func (m *Manager) GetPeerTracker() *PeerSyncTracker {
	return m.peerTracker
}
