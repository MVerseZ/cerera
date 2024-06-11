package chain

import (
	"errors"
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
	autoGen        bool
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

	if cfg.Chain.Path == "EMPTY" {
		// init with genesis empty cfg
		InitChainVault(genesisBlock)
		dataBlocks = append(dataBlocks, genesisBlock)
		cfg.UpdateChainPath("./chain.dat")
	} else {
		var readBlock, err = SyncVault()
		if err != nil {
			panic(err)
		}
		dataBlocks = append(dataBlocks, readBlock...)
		// validate added blocks
		lastCorrect, errorBlock := ValidateBlocks(dataBlocks)
		if errorBlock != nil {
			fmt.Printf("ERROR BLOCK! %s\r\n", errorBlock)
		}
		dataBlocks = dataBlocks[:lastCorrect]
	}

	stats := BlockChainStatus{
		Total:     0,
		ChainWork: 0,
	}

	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   &dataBlocks[len(dataBlocks)-1],
		blockTicker:    time.NewTicker(time.Duration(5 * time.Second)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
	}
	// genesisBlock.Head.Node = bch.currentAddress
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
		Index:         latest.Head.Index + 1,
		Timestamp:     uint64(time.Now().UnixMilli()),
		Number:        big.NewInt(0).Add(latest.Head.Number, big.NewInt(1)),
		PrevHash:      latest.Hash(),
		Confirmations: 1,
		Node:          bc.currentAddress,
		Root:          latest.Head.Root,
	}
	newBlock := block.NewBlockWithHeader(head)
	// TODO refactor
	newBlock.Transactions = []types.GTransaction{}
	if len(pool.Prepared) > 0 {
		for _, tx := range pool.Prepared {
			if vld.ValidateTransaction(tx, tx.From()) {
				newBlock.Transactions = append(newBlock.Transactions, *tx)
			}
		}
	}

	newBlock.Nonce = latest.Nonce

	var finalSize = unsafe.Sizeof(newBlock)
	newBlock.Head.Size = int(finalSize)
	newBlock.Head.GasUsed += uint64(finalSize)

	bc.data = append(bc.data, *newBlock)

	bc.info.Latest = newBlock.Hash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
	bc.currentBlock = newBlock

	SaveToVault(*newBlock)

	// clear array with included txs
	pool.Prepared = nil
}

// change block generation time
// val multiply by milliseconds (ms)
func (bc *Chain) ChangeBlockInterval(val int) {
	bc.blockTicker.Reset(time.Duration(time.Duration(val) * time.Millisecond))
}

// return lenght of array
func ValidateBlocks(blocks []block.Block) (int, error) {
	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}

	for i, blk := range blocks {
		// Проверка целостности цепочки блоков
		if i > 0 {
			prevBlock := blocks[i-1]
			if blk.Head.PrevHash != prevBlock.Hash() {
				return i - 1, fmt.Errorf("block %d has invalid previous hash", i)
			}
		}
	}
	return len(blocks), nil
}
