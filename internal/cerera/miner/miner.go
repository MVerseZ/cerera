package miner

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

// Miner is a minimal block builder that:
// - listens for new transactions from the mempool
// - builds a block template based on the latest chain head
// - assembles a block with coinbase + pending txs
// - computes hash and submits the block to the chain (no PoW)
type Miner struct {
	chain  *chain.Chain
	pool   *pool.Pool
	status string

	headerTemplate *block.Header
	quit           chan struct{}
	prepared       []*types.GTransaction
}

var m *Miner

// MinerObserver subscribes to mempool updates.
type MinerObserver struct{}

func (MinerObserver) GetID() string { return "OBSERVER_MINER" }

func (MinerObserver) Update(tx *types.GTransaction) {
	if m == nil {
		return
	}
	m.prepared = append(m.prepared, tx)
}

// Init initializes a singleton miner instance.
func Init() error {
	m = &Miner{
		chain:    chain.GetBlockChain(),
		pool:     pool.Get(),
		status:   "ALLOC",
		quit:     make(chan struct{}),
		prepared: make([]*types.GTransaction, 0),
	}
	m.pool.Register(MinerObserver{})
	m.updateTemplate()
	m.status = "READY"
	return nil
}

// Run starts a simple loop that periodically tries to assemble and submit a block.
func Run() {
	if m == nil {
		return
	}
	m.status = "RUN"
	for {
		select {
		case <-m.quit:
			m.status = "STOP"
			return
		case <-time.After(1 * time.Second):
			m.updateTemplate()
			if b := m.tryBuildBlock(); b != nil {
				m.submitBlock(b)
			}
		}
	}
}

// Stop stops miner loop.
func Stop() {
	if m == nil {
		return
	}
	select {
	case <-m.quit:
		// already closed
	default:
		close(m.quit)
	}
}

func (m *Miner) updateTemplate() {
	latest := m.chain.GetLatestBlock()
	m.headerTemplate = &block.Header{
		Ctx:        latest.Head.Ctx,
		Difficulty: latest.Head.Difficulty,
		Extra:      [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Height:     latest.Head.Height + 1,
		Index:      latest.Head.Index + 1,
		GasLimit:   latest.Head.GasLimit,
		GasUsed:    0,
		ChainId:    m.chain.GetChainId(),
		Node:       m.chain.GetCurrentChainOwnerAddress(),
		Size:       0,
		V:          [8]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x1, 0x0},
		Nonce:      latest.Head.Nonce + 1,
		PrevHash:   latest.Hash,
		Root:       latest.Head.Root,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}
}

// tryBuildBlock assembles a block from the template, coinbase and pending txs.
// No PoW: It simply computes logical hash and returns a ready block.
func (m *Miner) tryBuildBlock() *block.Block {
	if m.headerTemplate == nil {
		return nil
	}
	b := block.NewBlockWithHeader(m.headerTemplate)

	// Coinbase transaction
	cbTx := types.NewCoinBaseTransaction(
		m.headerTemplate.Nonce,
		m.chain.GetCurrentChainOwnerAddress(),
		coinbase.RewardBlock(),
		100,
		types.FloatToBigInt(1_000_000.0),
		[]byte("COINBASE_TX"),
	)
	b.Transactions = append(b.Transactions, *cbTx)
	b.Head.GasUsed += cbTx.Gas()

	// Pending transactions from pool
	pending := m.pool.GetPendingTransactions()
	for i := range pending {
		b.Transactions = append(b.Transactions, pending[i])
		b.Head.GasUsed += pending[i].Gas()
	}

	// Fill computed fields
	b.Head.Timestamp = uint64(time.Now().UnixMilli())
	b.Hash = block.CrvBlockHash(*b)

	bb, _ := json.Marshal(b)
	b.Head.Size = len(bb)

	return b
}

func (m *Miner) submitBlock(b *block.Block) {
	fmt.Printf("miner: submit block idx=%d hash=%s txs=%d\n", b.Head.Index, b.GetHash(), len(b.Transactions))
	for _, tx := range b.Transactions {
		m.pool.RemoveFromPool(tx.Hash())
	}
	m.chain.UpdateChain(b)
}

// GetMiner returns the miner singleton.
func GetMiner() interface{} { return m }
