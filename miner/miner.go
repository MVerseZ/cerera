package miner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validator"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const MINER_ID = "CERERA_MINER:937"
const REF_COUNT_INPUTS = 50 // TODO refactor this shit

var (
	ErrServiceRegistryNotFound = errors.New("service registry not found")
	ErrChainServiceNotFound    = errors.New("chain service not found")
	ErrPoolServiceNotFound     = errors.New("pool service not found")
	ErrLatestBlockNotFound     = errors.New("latest block not found")
	ErrInvalidBlock            = errors.New("invalid block")
	ErrBlockHeaderNil          = errors.New("block header is nil")
)

// minerLogger returns the miner logger, ensuring it's initialized after global logger
func minerLogger() *zap.SugaredLogger {
	return logger.Named("miner")
}

func init() {
	prometheus.MustRegister(
		minerBlocksMinedTotal,
		minerMiningAttemptsTotal,
		minerMiningErrorsTotal,
		minerMiningDurationSeconds,
		minerPendingTxsInBlock,
		minerStatus,
		minerHashValidationTotal,
		minerHashValidTotal,
		minerHashInvalidTotal,
		minerNonceSearchAttemptsTotal,
		minerNonceSearchAttempts,
		minerNonceSearchDurationSeconds,
		minerCurrentDifficulty,
		minerCurrentTarget,
		minerHashToTargetRatio,
	)
}

type Miner interface {
	GetID() string
	Start() error
	Stop()
	Status() byte
	SetChain(chain *chain.Chain)
	SetPool(p pool.TxPool)
	SetValidator(v validator.Validator)
	SetBlockPublisher(publisher BlockPublisher)
	SetHeightLock(lock HeightLockChecker)
	OnBlockFinalized(b *block.Block)
	OnChainReorg(b *block.Block)
	Update(tx *types.GTransaction)
}

// BlockPublisher proposes a mined block to the network consensus layer.
type BlockPublisher interface {
	ProposeBlock(b *block.Block) error
}

// PeerCounter is optionally implemented by BlockPublisher (e.g. icenet.Ice).
type PeerCounter interface {
	GetPeerCount() int
}

// HeightLockChecker interface for checking height locks from main package
type HeightLockChecker interface {
	TryLockHeight(height int) bool
	IsHeightLocked(height int) bool
	GetCancelChannel() <-chan struct{}
	GetLockedHeight() int
}

type miner struct {
	mu       sync.Mutex
	ctx      context.Context
	status   byte
	config   *config.Config
	chain    *chain.Chain
	pool     pool.TxPool
	valid    validator.Validator
	publisher BlockPublisher
	heightLock HeightLockChecker
	mining   bool
	stopChan chan struct{}

	miningHeight    int   // Height currently being mined
	miningCancelled int32 // 0 = not cancelled, 1 = cancelled (atomic access)

	MinerTimeout time.Duration
	worker       *Worker

	pendingBlock *block.Block
}

func Init(ctx context.Context, cfg *config.Config) (*miner, error) {
	return &miner{
		ctx:      ctx,
		config:   cfg,
		mining:   false,
		stopChan: make(chan struct{}),
	}, nil
}

func (m *miner) GetID() string {
	return MINER_ID
}

// func (m *miner) Start() error {
// 	m.status = 0x1
// 	m.mining = true
// 	m.stopChan = make(chan struct{})
// 	minerStatus.Set(1)

// 	minerLogger().Infow("[MINER] Starting miner", "id", m.GetID())
// 	m.MinerTimeout = 2 * time.Second

// 	w := NewWorker()
// 	m.worker = w
// 	m.worker.Start()
// 	t := &Task{
// 		id:    m.config.NetCfg.ADDR,
// 		chain: m.config.Chain.ChainID,
// 		prev:  m.chain.GetLatestBlock(),
// 		txs:   nil,
// 	}
// 	w.currentTask = t
// 	m.worker.Compute(nil)
// 	go func() {
// 		for {
// 			select {
// 			case data := <-m.worker.resultCh:
// 				if data.Error != nil {
// 					minerLogger().Infow("[MINER] block mined", "hash", data.Value.Hash)
// 					m.chain.UpdateChain(data.Value)
// 				}
// 				m.worker.Compute(nil)
// 			case <-m.worker.doneCh:
// 				fmt.Printf("doned\n")
// 				m.mining = false
// 			}
// 		}
// 	}()
// 	return nil
// }

func (m *miner) Start() error {
	m.mu.Lock()
	m.status = 0x1
	m.mining = true
	m.stopChan = make(chan struct{})
	minerStatus.Set(1)

	minerLogger().Infow("[MINER] Starting miner", "id", m.GetID())
	m.MinerTimeout = 2 * time.Second

	w := NewWorker()
	m.worker = w
	if m.heightLock != nil {
		w.SetHeightLock(m.heightLock)
	}
	m.attachStateRootFn(w)
	m.worker.Start()
	if m.shouldWaitForPeers() {
		go m.waitForPeersAndMine()
	} else {
		m.worker.Compute(m.newTask(nil))
	}
	m.mu.Unlock()

	go m.resultLoop()
	go m.watchHeightLock()
	return nil
}

func (m *miner) newTask(txs []*types.GTransaction) *Task {
	return &Task{
		id:    m.config.NetCfg.ADDR,
		chain: m.config.Chain.ChainID,
		prev:  m.chain.GetLatestBlock(),
		txs:   txs,
	}
}

func (m *miner) resultLoop() {
	for {
		select {
		case data := <-m.worker.resultCh:
			m.handleResult(data)
		case <-m.stopChan:
			fmt.Println("Miner stopped")
			if m.worker != nil {
				m.worker.Stop()
			}
			return
		}
	}
}

func (m *miner) watchHeightLock() {
	if m.heightLock == nil {
		return
	}

	cancelCh := m.heightLock.GetCancelChannel()
	for {
		select {
		case <-m.stopChan:
			return
		case <-cancelCh:
			m.onChainHeadUpdated(nil)
			cancelCh = m.heightLock.GetCancelChannel()
		}
	}
}

func (m *miner) onChainHeadUpdated(finalized *block.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.mining || m.worker == nil || m.chain == nil {
		return
	}

	blockToApply := finalized
	if blockToApply == nil {
		blockToApply = m.pendingBlock
	}
	if blockToApply != nil {
		m.applyBlock(blockToApply)
		m.clearProcessedTransactions(blockToApply.Transactions)
	}
	m.pendingBlock = nil

	var txs []*types.GTransaction
	if current := m.worker.GetCurrentTask(); current != nil {
		txs = remainingTxs(current, blockToApply)
	}
	m.worker.Compute(m.newTask(txs))
}

func (m *miner) handleResult(data Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.mining || m.worker == nil {
		return
	}
	if data.Error != nil {
		minerLogger().Errorw("[MINER] mining error", "error", data.Error)
		return
	}
	if m.worker.isStale(data.jobID) {
		minerLogger().Debugw("[MINER] ignoring stale mining result")
		return
	}

	if m.heightLock != nil && data.Value != nil && data.Value.Head != nil {
		if m.heightLock.IsHeightLocked(data.Value.Head.Height) {
			minerLogger().Debugw("[MINER] height already locked, skipping proposal",
				"height", data.Value.Head.Height,
			)
			m.worker.Compute(m.newTask(nil))
			return
		}
	}

	if m.publisher != nil {
		if m.shouldWaitForPeers() {
			minerLogger().Infow("[MINER] waiting for peers before proposing block")
			go m.waitForPeersAndMine()
			return
		}
		if err := m.publisher.ProposeBlock(data.Value); err != nil {
			minerLogger().Errorw("[MINER] failed to propose block", "error", err)
			current := m.worker.GetCurrentTask()
			var txs []*types.GTransaction
			if current != nil {
				txs = current.txs
			}
			m.worker.Compute(m.newTask(txs))
			return
		}
		minerLogger().Infow("[MINER] block proposed for consensus",
			"height", data.Value.Head.Height,
			"hash", data.Value.Hash,
		)
		m.pendingBlock = data.Value
		return
	}

	if err := m.chain.UpdateChain(data.Value); err != nil {
		minerLogger().Errorw("[MINER] failed to update chain", "error", err)
		current := m.worker.GetCurrentTask()
		var txs []*types.GTransaction
		if current != nil {
			txs = current.txs
		}
		m.worker.Compute(m.newTask(txs))
		return
	}

	m.applyBlock(data.Value)
	m.clearProcessedTransactions(data.Value.Transactions)
	remaining := remainingTxs(m.worker.GetCurrentTask(), data.Value)
	m.worker.Compute(m.newTask(remaining))
}

func (m *miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = 0x0
	m.mining = false
	minerStatus.Set(0)
	if m.worker != nil {
		m.worker.Stop()
	}
	if m.stopChan != nil {
		close(m.stopChan)
	}
	minerLogger().Info("[MINER] Miner stopped")
}

func (m *miner) Update(tx *types.GTransaction) {
	if tx == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.mining || m.worker == nil {
		return
	}

	txs := m.pool.GetMiningPackage(m.pool.GetInfo().Size)

	minerLogger().Infow("[MINER] New pool tx, restarting miner", "tx", tx.Hash(), "txs", len(txs))
	m.worker.Compute(m.newTask(txs))
}

func (m *miner) Status() byte {
	return m.status
}

func (m *miner) SetChain(chain *chain.Chain) {
	m.chain = chain
}

func (m *miner) SetPool(p pool.TxPool) {
	m.pool = p
}

func (m *miner) SetValidator(v validator.Validator) {
	m.valid = v
	if m.worker != nil {
		m.attachStateRootFn(m.worker)
	}
}

func (m *miner) attachStateRootFn(w *Worker) {
	if m.valid == nil || w == nil {
		return
	}
	w.SetStateRootFn(m.valid.ComputeBlockStateRoot)
}

func (m *miner) SetBlockPublisher(publisher BlockPublisher) {
	m.publisher = publisher
}

func (m *miner) SetHeightLock(lock HeightLockChecker) {
	m.heightLock = lock
}

func (m *miner) OnBlockFinalized(b *block.Block) {
	m.onChainHeadUpdated(b)
}

func (m *miner) OnChainReorg(b *block.Block) {
	m.onChainHeadUpdated(b)
}

func (m *miner) applyBlock(b *block.Block) {
	if m.valid == nil || b == nil {
		return
	}
	idx := 0
	if h := b.Header(); h != nil {
		idx = int(h.Index)
	}
	executed := 0
	for i := range b.Transactions {
		tx := b.Transactions[i]
		// Faucet RPC already credits the account; do not apply it twice if it lands in a block.
		if tx.Type() == types.FaucetTxType {
			continue
		}
		if err := m.valid.ExecuteTransaction(tx); err != nil {
			minerLogger().Errorw("[MINER] failed to execute tx",
				"tx", tx.Hash(),
				"type", tx.Type(),
				"error", err)
			continue
		}
		m.valid.UpdateTxTree(&b.Transactions[i], idx)
		executed++
	}
	if executed > 0 {
		height := 0
		if h := b.Header(); h != nil {
			height = h.Height
		}
		minerLogger().Infow("[MINER] executed block txs", "count", executed, "height", height)
	}
}

func (m *miner) clearProcessedTransactions(txs []types.GTransaction) {
	if m.pool == nil {
		return
	}
	for i := range txs {
		txType := txs[i].Type()
		if txType == types.CoinbaseTxType || txType == types.FaucetTxType {
			continue
		}
		// ExecuteTransaction already removes successful txs; ignore "not in mempool".
		_ = m.pool.RemoveFromPool(txs[i].Hash())
	}
}

func (m *miner) shouldWaitForPeers() bool {
	if len(m.config.NetCfg.SeedNodes) == 0 {
		return false
	}
	pc, ok := m.publisher.(PeerCounter)
	return ok && pc.GetPeerCount() == 0
}

func (m *miner) waitForPeersAndMine() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.mu.Lock()
			waiting := m.shouldWaitForPeers()
			if !waiting && m.mining && m.worker != nil {
				m.worker.Compute(m.newTask(nil))
				m.mu.Unlock()
				return
			}
			m.mu.Unlock()
			if waiting {
				minerLogger().Infow("[MINER] waiting for peers before mining")
			}
		}
	}
}

func (m *miner) ServiceName() string {
	return MINER_ID
}

func (m *miner) Methods() map[string]service.RPCHandler {
	return nil
}
