package miner

import (
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/randomx"
	"github.com/cerera/internal/cerera/types"
)

// PROTOTYPE STRUCTURE

type Miner struct {
	difficulty int64
	status     string
}

var m *Miner
var xvm *randomx.RxVM

func Init() {
	var flags = []randomx.Flag{randomx.FlagDefault}
	var myCache, _ = randomx.AllocCache(flags...)
	var seed = []byte(big.NewInt(114167270716410).Bytes())
	randomx.InitCache(myCache, seed)
	// var dataset, _ = randomx.AllocDataset(flags...)
	var rxDs, _ = randomx.NewRxDataset(flags...)
	xvm, _ = randomx.NewRxVM(rxDs, flags...)
	// randomx.SetVMDataset(xvm, dataset)
	xvm.CalcHashFirst([]byte("FIRST"))
	m = &Miner{
		difficulty: 1,
		status:     "INIT",
	}
	// randomx.CalculateHashFirst(xvm, []byte("FIRST"))

	// var res2 = randomx.CalculateHashNext(xvm, []byte("NEXT"))

	// hash, found, sol := randomx.Search(xvm, []byte("INPUT DATA"), target, maxTimes, jump, nonce)
	// fmt.Println(hash)
	// fmt.Println(common.BytesToHash(hash))
	// fmt.Println(found)
	// fmt.Println(sol)

	// fmt.Println(common.BytesToHash(res2))
	// fmt.Println(res2)
	// randomx.DestroyVM(xvm)
}

func Run() {
}

func Start() {

}

func Stop() {

}

func CalculateHash(b *block.Block) common.Hash {
	var bhash = xvm.CalcHash(b.ToBytes())
	return common.BytesToHash(bhash)
}

func TryToFind(prevBlock *block.Block, chainId *big.Int, difficulty uint64, maxTimes uint64, jump uint32) ([]byte, bool, []byte) {

	// block consume txs from pool or event-call from pool to blockchain/miner/validator/other ???
	var hash []byte
	var find = false
	var sol []byte = []byte{0x0, 0x0, 0x0, 0x0}
	var maxNonce = difficulty
	var newHeight = prevBlock.Header().Height + 1
	var newIndex = prevBlock.Header().Index + 1
	fmt.Printf("curr nonce: %d\r\n", prevBlock.Header().Nonce)
	fmt.Printf("max nonce: %d\r\n", maxNonce)
	fmt.Printf("target difficulty: %d\r\n", difficulty)

	for nonce := prevBlock.Header().Nonce; nonce < maxNonce; nonce++ {
		head := &block.Header{
			Ctx:        prevBlock.Header().Ctx,
			Difficulty: difficulty,
			Extra:      []byte(""),
			Height:     newHeight,
			Index:      newIndex,
			Timestamp:  uint64(time.Now().UnixMilli()),
			ChainId:    chainId,
			PrevHash:   prevBlock.GetHash(),
			// Node:       bc.currentAddress,
			GasLimit: prevBlock.Head.GasLimit, // todo get gas limit dynamically
			Nonce:    nonce,
		}
		var preBlock = block.NewBlockWithHeader(head)
		h, f, sol := xvm.Search(preBlock.ToBytes(), difficulty, 1, 1, preBlock.GetNonceBytes())
		if f {
			preBlock.Hash = common.BytesToHash(h)
			return h, f, sol
		}
		hash = h
	}
	return hash, find, sol
}

func CalculateBlockHash(b block.Block) {

}

func MineBlock(latest *block.Block, addr types.Address) {
	head := &block.Header{
		Ctx:        latest.Header().Ctx,
		Difficulty: latest.Header().Difficulty,
		Extra:      []byte("OP_AUTO_GEN_BLOCK_DAT"),
		Height:     latest.Header().Height + 1,
		Index:      latest.Header().Index + 1,
		Timestamp:  uint64(time.Now().UnixMilli()),
		ChainId:    latest.Header().ChainId,
		// PrevHash:      bc.info.Latest,
		Node:     addr,
		Root:     latest.Header().Root,
		GasLimit: latest.Head.GasLimit, // todo get gas limit dynamically
	}
	fmt.Println(head)
}
