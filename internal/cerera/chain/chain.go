package chain

import (
	"fmt"
	"math/big"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
)

type BlockChainStatus struct {
	Total     int         `json:"total,omitempty"`
	ChainWork int         `json:"chainWork,omitempty"`
	Latest    common.Hash `json:"latest,omitempty"`
}

type Chain struct {
	autoGen bool
	// buf            []*types.GTransaction
	chainId        *big.Int
	chainWork      *big.Int
	currentAddress types.Address
	currentBlock   *block.Block
	rootHash       common.Hash

	// mu sync.Mutex
	info BlockChainStatus
	data []block.Block

	// tickers
	maintainTicker *time.Ticker
	blockTicker    *time.Ticker
	DataChannel    chan []byte
}

var bch Chain

func GetBlockChain() Chain {
	return bch
}
func InitBlockChain(cfg *config.Config) Chain {

	genesisBlock := block.Genesis()

	dataBlocks := make([]block.Block, 0)
	dataBlocks = append(dataBlocks, genesisBlock)
	stats := BlockChainStatus{
		Total:     1,
		ChainWork: genesisBlock.Head.Size,
	}

	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   &genesisBlock,
		blockTicker:    time.NewTicker(time.Duration(10 * time.Second)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
	}
	genesisBlock.Head.Node = bch.currentAddress
	go bch.BlockGenerator()
	return bch
}

func (bc *Chain) GetInfo() interface{} {
	bc.info.Total = len(bc.data)
	bc.info.Latest = bc.data[len(bc.data)-1].Hash()
	return bc.info
}

func (bc Chain) GetLatestBlock() *block.Block {
	return bc.currentBlock
}

func (bc Chain) GetBlockHash(number int) common.Hash {
	for _, b := range bc.data {
		if b.Head.Number.Cmp(big.NewInt(int64(number))) == 0 {
			return b.Hash()
		}
	}
	return common.EmptyHash()
}

func (bc Chain) GetBlock(blockHash string) *block.Block {
	var bHash = common.HexToHash(blockHash)
	for _, b := range bc.data {
		if b.Hash().Compare(bHash) == 0 {
			var cfrm = b.Head.Confirmations
			b.Confirmations = cfrm
			return &b
		}
	}
	return &block.Block{}
}

func (bc Chain) GetBlockHeader(blockHash string) *block.Header {
	var bHash = common.HexToHash(blockHash)
	for _, b := range bc.data {
		if b.Hash().Compare(bHash) == 0 {
			return b.Head
		}
	}
	return &block.Header{}
}

func (bc *Chain) BlockGenerator() {
	for {
		select {
		case <-bc.blockTicker.C:
			fmt.Printf("Block ticker\r\n")
			for _, b := range bc.data {
				b.Head.Confirmations = b.Header().Confirmations + 1
			}

			var latest = bc.GetLatestBlock()
			if bc.autoGen {
				bc.G(latest)
			}
		}
	}
}

func (bc *Chain) G(latest *block.Block) {
	var vld = validator.Get()
	var pool = pool.Get()
	head := &block.Header{
		Ctx: latest.Head.Ctx,
		Difficulty: big.NewInt(0).Add(
			latest.Head.Difficulty,
			big.NewInt(int64(latest.Head.Ctx)),
		),
		Extra:         []byte("OP_AUTO_GEN_BLOCK_DAT"),
		Height:        latest.Head.Height + 1,
		Timestamp:     uint64(time.Now().UnixMilli()),
		Number:        big.NewInt(0).Add(latest.Head.Number, big.NewInt(1)),
		PrevHash:      latest.Hash(),
		Confirmations: 1,
		Node:          bc.currentAddress,
	}
	newBlock := block.NewBlockWithHeader(head)
	// TODO refactor
	if len(pool.Prepared) > 0 {
		for _, tx := range pool.Prepared {
			if vld.ValidateTransaction(tx, tx.From()) {
				newBlock.Transactions = append(newBlock.Transactions, *tx)
			}
		}
	}

	var finalSize = unsafe.Sizeof(newBlock)
	newBlock.Head.Size = int(finalSize)
	newBlock.Head.GasUsed += uint64(finalSize)

	bc.data = append(bc.data, *newBlock)

	bc.info.Latest = newBlock.Hash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	bc.currentBlock = newBlock

	// clear array with included txs
	pool.Prepared = nil
}

// func (bc *Chain) AddApprovedTx(tx *types.GTransaction) {
// 	bc.buf = append(bc.buf, tx)
// }
