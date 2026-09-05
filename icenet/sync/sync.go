package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/peers"
	"github.com/cerera/icenet/protocol"
	"github.com/cerera/icenet/service"
	"github.com/cerera/internal/logger"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

const (
	// SyncCheckInterval is the interval between sync checks
	SyncCheckInterval = 30 * time.Second
	// SyncTimeout is the timeout for a sync operation
	SyncTimeout = 5 * time.Minute
	// peerStatusRefreshTimeout bounds status RPC during periodic sync checks.
	peerStatusRefreshTimeout = 10 * time.Second
)

// shouldSyncChain reports whether the local node should catch up to peerHeight.
func shouldSyncChain(peerHeight, localHeight int) bool {
	return peerHeight > localHeight
}

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
	serviceProvider *service.ServiceProvider
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

	forkDetector ForkDetector
}

// ForkDetector handles competing blocks and orphan storage during sync.
type ForkDetector interface {
	OnCompetingBlock(local, remote *block.Block, from peer.ID)
	OnPrevHashMismatch(localHead, incoming *block.Block, from peer.ID)
	ProcessOrphan(b *block.Block, from peer.ID) bool
	OnBlockLinked(b *block.Block)
}

// NewManager creates a new sync manager
func NewManager(
	ctx context.Context,
	h host.Host,
	handler *protocol.Handler,
	peerManager *peers.Manager,
	provider *service.ServiceProvider,
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

// refreshPeerStatuses re-queries ready peers so GetBestPeer uses current heights.
func (m *Manager) refreshPeerStatuses() {
	if m.handler == nil || m.peerManager == nil {
		return
	}

	for _, p := range m.peerManager.GetPeers() {
		if p == nil || !p.IsReady {
			continue
		}

		ctx, cancel := context.WithTimeout(m.ctx, peerStatusRefreshTimeout)
		status, err := m.handler.RequestStatus(ctx, p.ID)
		cancel()
		if err != nil {
			syncLogger().Debugw("Failed to refresh peer status",
				"peer", p.ID,
				"error", err,
			)
			continue
		}

		m.peerManager.UpdatePeerInfo(p.ID, status.Height, status.Status, status.NodeAddress)
		m.peerTracker.UpdatePeer(p.ID, status.Height, status.Status)
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

	m.refreshPeerStatuses()

	// Get current height
	currentHeight := 0
	if m.serviceProvider != nil {
		currentHeight = m.serviceProvider.GetCurrentHeight()
	}

	// Find best peer after refresh
	bestPeer := m.peerManager.GetBestPeer()
	if bestPeer == nil {
		return
	}

	// Chain catch-up: when any ready peer is ahead on height.
	if shouldSyncChain(bestPeer.Height, currentHeight) {
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
		"bestPeerHeight", bestPeer.Height,
	)

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

	if pi := m.peerManager.GetPeer(peerID); pi != nil {
		m.peerTracker.UpdatePeer(peerID, targetHeight, pi.Status)
	}

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
					if strings.Contains(err.Error(), "Prev hash diff") {
						syncLogger().Warnw("Sync fork detected — stopping batch",
							"peer", peerID,
							"height", b.Head.Height,
							"localHeight", currentHeight,
						)
						if m.forkDetector != nil {
							localHead := m.serviceProvider.GetBlockByHeight(currentHeight)
							m.forkDetector.OnPrevHashMismatch(localHead, b, peerID)
							m.forkDetector.ProcessOrphan(b, peerID)
						}
						break
					}
					continue
				}
			}

			processedCount++
			currentHeight = b.Head.Height

			// Call callback
			if m.onNewBlock != nil {
				m.onNewBlock(b)
			}
			if m.forkDetector != nil {
				m.forkDetector.OnBlockLinked(b)
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

	syncLogger().Infow("[SYNC] Start sync with:",
		"peer", peerID,
	)

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

	if shouldSyncChain(status.Height, currentHeight) {
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
		if b.Head.Height == currentHeight {
			local := m.serviceProvider.GetBlockByHeight(currentHeight)
			if local != nil && local.Hash != b.Hash && m.forkDetector != nil {
				m.forkDetector.OnCompetingBlock(local, b, fromPeer)
				m.forkDetector.ProcessOrphan(b, fromPeer)
			}
		}
		syncLogger().Debugw("Skipping block - already have block at this height",
			"receivedHeight", b.Head.Height,
			"currentHeight", currentHeight,
			"from", fromPeer,
		)
		return nil
	}

	// Side chain: next height but different parent
	if b.Head.Height == currentHeight+1 {
		latestHash := m.serviceProvider.GetLatestHash()
		if latestHash != (common.Hash{}) && b.Head.PrevHash != latestHash {
			if m.forkDetector != nil {
				localHead := m.serviceProvider.GetBlockByHeight(currentHeight)
				m.forkDetector.OnPrevHashMismatch(localHead, b, fromPeer)
				m.forkDetector.ProcessOrphan(b, fromPeer)
			}
			return nil
		}
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
		if m.forkDetector != nil {
			m.forkDetector.OnBlockLinked(b)
		}

		return nil
	}

	// Unknown parent — store as orphan
	if m.forkDetector != nil && m.forkDetector.ProcessOrphan(b, fromPeer) {
		return nil
	}

	// Block is ahead - we might need to sync
	if shouldSyncChain(b.Head.Height, currentHeight) {
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

// SetForkDetector wires fork/orphan handling during sync.
func (m *Manager) SetForkDetector(det ForkDetector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forkDetector = det
}

// SetOnNewBlock sets the callback for new blocks
func (m *Manager) SetOnNewBlock(callback func(*block.Block)) {
	m.onNewBlock = callback
}

// ForceSync forces a sync with the best available peer
func (m *Manager) ForceSync() error {
	m.refreshPeerStatuses()

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
