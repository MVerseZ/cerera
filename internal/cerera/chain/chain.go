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
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type BlockChainStatus struct {
	Total     int         `json:"total,omitempty"`
	ChainWork int         `json:"chainWork,omitempty"`
	Latest    common.Hash `json:"latest,omitempty"`
	Size      int64       `json:"size,omitempty"`
}

type Chain struct {
	autoGen        bool
	chainId        *big.Int
	chainWork      *big.Int
	currentAddress types.Address
	currentBlock   *block.Block
	// rootHash       common.Hash

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
		Latest:    dataBlocks[len(dataBlocks)-1].Hash(),
		Size:      0,
	}

	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   &dataBlocks[len(dataBlocks)-1],
		blockTicker:    time.NewTicker(time.Duration(1984 * time.Microsecond)),
		maintainTicker: time.NewTicker(time.Duration(5 * time.Minute)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
	}
	// genesisBlock.Head.Node = bch.currentAddress
	go bch.BlockGenerator()
	return bch
}

func (bc *Chain) GetInfo() interface{} {
	var bcs, err = GetChainSourceSize()
	if err != nil {
		bc.info.Size = -1
	} else {
		bc.info.Size = bcs
	}
	bc.info.Total = len(bc.data)
	bc.info.Latest = bc.data[len(bc.data)-1].Hash()

	return bc.info
}

func (bc Chain) GetLatestBlock() *block.Block {
	return bc.currentBlock
}

func (bc Chain) GetBlockHash(number int) common.Hash {
	for _, b := range bc.data {
		if b.Header().Number.Cmp(big.NewInt(int64(number))) == 0 {
			return b.Hash()
		}
	}
	return common.EmptyHash()
}

func (bc Chain) GetBlock(blockHash string) *block.Block {
	var bHash = common.HexToHash(blockHash)
	for _, b := range bc.data {
		if b.Hash().Compare(bHash) == 0 {
			return &b
		}
	}
	return &block.Block{}
}

func (bc Chain) GetBlockHeader(blockHash string) *block.Header {
	var bHash = common.HexToHash(blockHash)
	for _, b := range bc.data {
		if b.Hash().Compare(bHash) == 0 {
			return b.Header()
		}
	}
	return &block.Header{}
}

func (bc *Chain) BlockGenerator() {
	for {
		select {
		case <-bc.blockTicker.C:
			var latest = bc.GetLatestBlock()
			if bc.autoGen {
				bc.G(latest)
			}
		case <-bc.maintainTicker.C:
			continue
		}
	}
}

func (bc *Chain) G(latest *block.Block) {
	var vld = validator.Get()
	var pool = pool.Get()
	head := &block.Header{
		Ctx:           latest.Header().Ctx,
		Difficulty:    latest.Header().Difficulty,
		Extra:         []byte("OP_AUTO_GEN_BLOCK_DAT"),
		Height:        latest.Header().Height + 1,
		Index:         latest.Header().Index + 1,
		Timestamp:     uint64(time.Now().UnixMilli()),
		Number:        big.NewInt(0).Add(latest.Header().Number, big.NewInt(1)),
		PrevHash:      bc.info.Latest,
		Confirmations: 1,
		Node:          bc.currentAddress,
		Root:          latest.Header().Root,
	}
	newBlock := block.NewBlockWithHeader(head)
	// TODO refactor
	if len(pool.Prepared) > 0 {
		for _, tx := range pool.Prepared {
			if vld.ValidateTransaction(tx, tx.From()) {
				newBlock.Transactions = append(newBlock.Transactions, *tx)
				newBlock.Head.GasUsed += tx.Gas()
				// newBlock.SetTransaction(tx)
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

	// progress
	doneCh := make(chan struct{})

	bar := progressbar.NewOptions(len(blocks),
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(25),
		progressbar.OptionSetDescription("[cyan][0/1][reset] Validate blocks..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			doneCh <- struct{}{}
		}),
	)
	// go func() {
	// 	for i := 0; i < 1000; i++ {
	// 		bar.Add(1)
	// 		time.Sleep(1 * time.Millisecond)
	// 	}
	// }()

	for i, blk := range blocks {
		bar.Add(1)
		// Проверка целостности цепочки блоков
		if i > 0 {
			prevBlock := blocks[i-1]
			// fmt.Printf("%d-%d: %s - %s\r\n", i-1, i, blk.Head.PrevHash, prevBlock.Hash())
			if blk.Head.PrevHash.String() != prevBlock.Hash().String() {
				bar.Describe("[red][1/1][break] Validating blocks break!")
				<-doneCh
				return i - 1, fmt.Errorf("block %d has invalid previous hash", i)
			}
		}
	}

	bar.Describe("[cyan][1/1][done] Validate blocks done!")
	// got notified that progress bar is complete.
	<-doneCh
	fmt.Println()
	return len(blocks), nil
}
