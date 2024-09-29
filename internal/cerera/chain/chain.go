package chain

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"
	"unsafe"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/trie"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
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
	t    *trie.MerkleTree

	// tickers
	maintainTicker *time.Ticker
	blockTicker    *time.Ticker
	DataChannel    chan []byte
}

var (
	bch        Chain
	BLOCKTIMER = time.Duration(1 * time.Second)
)

func GetBlockChain() *Chain {
	return &bch
}
func InitBlockChain(cfg *config.Config) Chain {

	genesisBlock := block.Genesis()
	dataBlocks := make([]block.Block, 0)

	var (
		t         *trie.MerkleTree
		chainWork = 0
		total     = 0
	)
	if cfg.Chain.Path == "EMPTY" {
		var list []trie.Content
		list = append(list, genesisBlock)
		t, _ = trie.NewTree(list)
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
			log.Printf("ERROR BLOCK! %s\r\n", errorBlock)
		}
		dataBlocks = dataBlocks[:lastCorrect]

		var list []trie.Content
		for _, v := range dataBlocks {
			list = append(list, v)
			total += 1
			chainWork += v.Head.Size
			// 		bc.info.Total = bc.info.Total + 1
			// bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
		}

		t, _ = trie.NewTree(list)
		t.VerifyTree()
	}

	stats := BlockChainStatus{
		Total:     total,
		ChainWork: chainWork,
		Latest:    dataBlocks[len(dataBlocks)-1].Hash(),
		Size:      0,
	}
	//	0xb51551C31419695B703aD37a2c04A765AB9A6B4a183041354a6D392ce438Aec47eBb16495E84F18ef492B50f652342dE
	bch = Chain{
		autoGen:        cfg.AUTOGEN,
		chainId:        cfg.Chain.ChainID,
		chainWork:      big.NewInt(1),
		currentBlock:   &dataBlocks[len(dataBlocks)-1],
		blockTicker:    time.NewTicker(BLOCKTIMER),
		maintainTicker: time.NewTicker(time.Duration(5 * time.Minute)),
		info:           stats,
		data:           dataBlocks,
		currentAddress: cfg.NetCfg.ADDR,
		t:              t,
		DataChannel:    make(chan []byte),
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

func (bc Chain) GetBlock(blockHash common.Hash) *block.Block {
	for _, b := range bc.data {
		if b.Hash().Compare(blockHash) == 0 {
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
		Ctx:        latest.Header().Ctx,
		Difficulty: latest.Header().Difficulty,
		Extra:      []byte("OP_AUTO_GEN_BLOCK_DAT"),
		Height:     latest.Header().Height + 1,
		Index:      latest.Header().Index + 1,
		Timestamp:  uint64(time.Now().UnixMilli()),
		Number:     big.NewInt(0).Add(latest.Header().Number, big.NewInt(1)),
		PrevHash:   bc.info.Latest,
		Node:       bc.currentAddress,
		Root:       latest.Header().Root,
		GasLimit:   latest.Head.GasLimit, // todo get gas limit dynamically
	}
	// cpy version, should store elsewhere
	head.V = latest.Head.V
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

	if vld.ValidateBlock(*newBlock) {
		bc.data = append(bc.data, *newBlock)
		bc.t.Add(newBlock)
		var t, err = bc.t.VerifyTree()
		if err != nil || !t {
			log.Printf("Verifying trie error: %s\r\n", err)
		} else {
			bc.info.Latest = newBlock.Hash()
			bc.info.Total = bc.info.Total + 1
			bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
			bc.currentBlock = newBlock
			err := SaveToVault(*newBlock)
			if err == nil {
				var rewardAddress = newBlock.Head.Node
				fmt.Printf("Reward to: %s\r\n", rewardAddress)
			}
		}
		// clear array with included txs
		pool.Prepared = nil
	} else {
		fmt.Printf("Block unconfirmed: %s\r\n", newBlock.Hash())
		return
	}
}

// change block generation time
// val multiply by milliseconds (ms)
func (bc *Chain) ChangeBlockInterval(val int) {
	bc.blockTicker.Reset(time.Duration(time.Duration(val) * time.Millisecond))
}

func (bc *Chain) UpdateChain(newBlock *block.Block) {
	fmt.Printf("Current index: %d with hash: %s\r\n", bc.currentBlock.Head.Number, bc.currentBlock.Hash())
	fmt.Printf("Incoming index: %d with hash: %s\r\n", newBlock.Head.Number, newBlock.Hash())

	if newBlock.Head.Number.Cmp(big.NewInt(0)) == 0 {
		// replace all
		ClearVault()
		bc.data = nil
	}
	bc.data = append(bc.data, *newBlock)
	fmt.Printf("Update index: %d with hash: %s\r\n", newBlock.Head.Number, newBlock.Hash())

	// bc.currentBlock = newBlock
	err := SaveToVault(*newBlock)
	if err == nil {
		var rewardAddress = newBlock.Head.Node
		fmt.Printf("Reward to: %s\r\n", rewardAddress)
	}
	bc.info.Latest = newBlock.Hash()
	bc.info.Total = bc.info.Total + 1
	bc.info.ChainWork = bc.info.ChainWork + newBlock.Head.Size
}

func (bc *Chain) Idle() {
	bc.autoGen = false
}

func (bc *Chain) Resume() {
	bc.autoGen = true
}

// return lenght of array
func ValidateBlocks(blocks []block.Block) (int, error) {
	var vld = validator.Get()

	if len(blocks) == 0 {
		return -1, errors.New("no blocks to validate")
	}

	for i, blk := range blocks {
		// check version chain
		if vld.GetVersion() != blocks[i].Head.V {
			return i, errors.New("wrong chain version")
		}
		if blocks[i].Head.GasUsed > blocks[i].Head.GasLimit {
			return i, errors.New("wrong gas data in block")
		}
		// Проверка целостности цепочки блоков
		if i > 0 {
			prevBlock := blocks[i-1]
			//log.Printf("%d-%d: %s - %s\r\n", i-1, i, blk.Head.PrevHash, prevBlock.Hash())
			if blk.Head.PrevHash.String() != prevBlock.Hash().String() {
				return i - 1, fmt.Errorf("block %d has invalid previous hash", i)
			}
		}
	}

	return len(blocks), nil
}
