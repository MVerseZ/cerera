package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/peers"
	"github.com/cerera/icenet/protocol"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// SyncCheckInterval is the interval between sync checks
	SyncCheckInterval = 30 * time.Second
	// SyncTimeout is the timeout for a sync operation
	SyncTimeout = 5 * time.Minute
	// MinBlocksAhead is the minimum number of blocks a peer must be ahead to trigger sync.
	// We keep this low so that nodes that are slightly behind still trigger a catch-up.
	MinBlocksAhead = 1
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
	m.peerTracker.UpdatePeer(bestPeer.ID, bestPeer.Height, protocol.Status{})

	// Chain catch-up: only when the peer is ahead on height.
	if bestPeer.Height > currentHeight+MinBlocksAhead {
		syncLogger().Infow("Sync needed",
			"currentHeight", currentHeight,
			"peerHeight", bestPeer.Height,
			"peer", bestPeer.ID,
		)
		go m.syncChainWithPeer(bestPeer.ID, currentHeight, bestPeer.Height)
	}

	currentStorageSize := 0
	if m.serviceProvider != nil {
		currentStorageSize = m.serviceProvider.GetStorageSize()
	}

	syncLogger().Debugw("Sync check state",
		"height", currentHeight,
		"storageSize", currentStorageSize,
		"bestPeer", bestPeer.ID,
	)

	newBestPeer := m.peerManager.GetBestPeer()
	if bestPeer != newBestPeer {
		syncLogger().Infow("Best peer changed during sync check",
			"previousPeer", bestPeer.ID,
			"newBestPeer", newBestPeer.ID,
		)
		bestPeer = m.peerManager.GetBestPeer()
	}
	bestPeer = newBestPeer
	if bestPeer == nil {
		return
	}

	localStorageSvc := ""
	if m.serviceProvider != nil {
		localStorageSvc = m.serviceProvider.GetStorageServiceName()
	}
	if localStorageSvc != "" && bestPeer.Status.StorageService != "" &&
		localStorageSvc != bestPeer.Status.StorageService {
		return
	}

	// Vault snapshot sync must run even when heights already match; otherwise
	// nodes stay with divergent account sets forever.
	if bestPeer.Status.StorageData > currentStorageSize {
		syncLogger().Infow("Peer has more accounts — triggering storage sync",
			"currentStorageSize", currentStorageSize,
			"peerStorageSize", bestPeer.Status.StorageData,
			"peer", bestPeer.ID,
		)
		go m.syncStorageWithPeer(bestPeer.ID, currentStorageSize, bestPeer.Status.StorageData)
	}

}

// syncChainWithPeer synchronizes blocks with a specific peer.
func (m *Manager) syncChainWithPeer(peerID peer.ID, startHeight, targetHeight int) {
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

const storageSnapshotChunkLimit = 256

// syncStorageWithPeer pulls serialized vault accounts from the peer (chunked) and merges locally.
// It does not use the block-sync isSyncing flag so it can run in parallel with chain catch-up.
func (m *Manager) syncStorageWithPeer(peerID peer.ID, startStorageSize, targetSize int) {
	if m.handler == nil || m.serviceProvider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, SyncTimeout)
	defer cancel()

	syncLogger().Infow("Starting storage sync",
		"peer", peerID,
		"localAccountCount", startStorageSize,
		"peerReportedCount", targetSize,
	)

	offset := 0
	applied := 0
	for {
		select {
		case <-ctx.Done():
			syncLogger().Infow("Storage sync cancelled or timed out", "peer", peerID, "offset", offset)
			return
		default:
		}

		resp, err := m.handler.RequestStorageSnapshot(ctx, peerID, offset, storageSnapshotChunkLimit)
		if err != nil {
			syncLogger().Errorw("Storage snapshot request failed", "peer", peerID, "offset", offset, "error", err)
			return
		}

		m.serviceProvider.ApplyStorageAccounts(resp.Accounts)
		applied += len(resp.Accounts)

		if !resp.More {
			break
		}
		if resp.NextOffset <= offset {
			syncLogger().Warnw("Storage snapshot nextOffset did not advance, stopping", "peer", peerID, "offset", offset)
			break
		}
		offset = resp.NextOffset
	}

	nowCount := 0
	if m.serviceProvider != nil {
		nowCount = m.serviceProvider.GetStorageSize()
	}

	if nowCount < targetSize {
		syncLogger().Warnw("Storage sync ended with fewer accounts than peer reported at start (retry on next sync tick)",
			"peer", peerID,
			"accountCountNow", nowCount,
			"peerReportedAtStart", targetSize,
			"blobsApplied", applied,
		)
	}

	syncLogger().Infow("Storage sync completed",
		"peer", peerID,
		"blobsApplied", applied,
		"accountCountNow", nowCount,
	)
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

	// Build local status for comparison (chain + storage fingerprint).
	localStatus, _ := protocol.GetStatus(m.serviceProvider)

	// Compare local and remote status.
	chainMatch := (localStatus.ChainID == status.Status.ChainID) &&
		(localStatus.GenesisHash == status.Status.GenesisHash)
	storageSvcMatch := localStatus.StorageService == status.Status.StorageService
	storageCountMatch := status.Status.StorageData == localStatus.StorageData

	switch {
	case !chainMatch:
		syncLogger().Warnw("[SYNC] Peer incompatible: chain mismatch",
			"peer", peerID,
			"localChainID", localStatus.ChainID,
			"remoteChainID", status.Status.ChainID,
			"localGenesis", localStatus.GenesisHash,
			"remoteGenesis", status.Status.GenesisHash,
		)
	case !storageSvcMatch:
		syncLogger().Warnw("[SYNC] Peer incompatible: vault/storage service mismatch",
			"peer", peerID,
			"localStorageService", localStatus.StorageService,
			"remoteStorageService", status.Status.StorageService,
		)
	case !storageCountMatch:
		syncLogger().Infow("[SYNC] Peer usable (chain+vault OK); account counts differ — periodic sync will catch up",
			"peer", peerID,
			"localAccounts", localStatus.StorageData,
			"remoteAccounts", status.Status.StorageData,
			"storageService", localStatus.StorageService,
		)
	default:
		syncLogger().Infow("[SYNC] Peer ready: chain+storage match",
			"peer", peerID,
			"localChainID", localStatus.ChainID,
			"remoteChainID", status.Status.ChainID,
			"storageService", localStatus.StorageService,
		)
	}

	// Update peer info
	m.peerManager.UpdatePeerInfo(peerID, status.Height, status.Status, status.NodeAddress)
	// Ready for sync/block work if chain and vault implementation match; divergent
	// account counts must not hide the peer from GetBestPeer or storage sync stalls.
	if chainMatch && storageSvcMatch {
		m.peerManager.MarkPeerReady(peerID, true)
	} else {
		m.peerManager.MarkPeerReady(peerID, false)
	}
	m.peerTracker.UpdatePeer(peerID, status.Height, status.Status)

	syncLogger().Infow("[SYNC] Peer status received",
		"peer", peerID,
		"height", status.Height,
		"version", status.Version,
		"address", status.NodeAddress,
	)

	// Check if we need to sync
	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	if status.Height > currentHeight+MinBlocksAhead {
		go m.syncChainWithPeer(peerID, currentHeight, status.Height)
	}

	localStorage := 0
	if m.serviceProvider != nil {
		localStorage = m.serviceProvider.GetStorageSize()
	}
	if chainMatch &&
		localStatus.StorageService == status.Status.StorageService &&
		status.Status.StorageData > localStorage {
		go m.syncStorageWithPeer(peerID, localStorage, status.Status.StorageData)
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
		go m.syncChainWithPeer(fromPeer, currentHeight, b.Head.Height)
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

	go m.syncChainWithPeer(bestPeer.ID, currentHeight, bestPeer.Height)
	return nil
}

// GetPeerTracker returns the peer sync tracker
func (m *Manager) GetPeerTracker() *PeerSyncTracker {
	return m.peerTracker
}
