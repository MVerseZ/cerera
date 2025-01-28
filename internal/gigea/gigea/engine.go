package gigea

import (
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

type TxTree struct {
}

type Engine struct {
	TxFunnel     chan *types.GTransaction // input tx funnel
	BlockFunnel  chan *block.Block        // input block funnel
	BlockPipe    chan block.Block
	Transaions   TxTree
	Owner        types.Address
	Transactions *TxMerkleTree
	List         []types.GTransaction
}

func (e *Engine) Start(lAddr types.Address) {
	// pipes
	e.BlockFunnel = make(chan *block.Block)
	e.TxFunnel = make(chan *types.GTransaction)
	e.BlockPipe = make(chan block.Block)

	e.List = make([]types.GTransaction, 0)

	e.Owner = lAddr
	// var firstTx = coinbase.CreateCoinBaseTransation(C.Nonce, e.Owner)
	// var list []types.Content
	// list = append(list, firstTx)
	// var err error
	// fmt.Printf("Coinbase tx hash: %s\r\n", firstTx.Hash())
	// var b, _ = firstTx.CalculateHash()
	// fmt.Printf("Coinbase tx hash: %s\r\n", common.BytesToHash(b))
	// e.Transactions, err = NewTree(list)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Root hash: %s\r\n", common.BytesToHash(e.Transactions.MerkleRoot()))
	go e.Listen()
}

func (e *Engine) Mine() {

	var newDifficulty = big.NewInt(11)
	var newHeight = 11
	var newNumber = big.NewInt(11)
	var prevHash = types.EmptyCodeRootHash
	var gasLimit = uint64(11)
	newHeader := &block.Header{
		Ctx:        int32(C.Nonce),
		Difficulty: newDifficulty,
		Extra:      []byte("OP_MINE"),
		Height:     newHeight,
		Index:      C.Nonce,
		Timestamp:  uint64(time.Now().UnixMilli()),
		Number:     newNumber,
		PrevHash:   prevHash,
		Node:       e.Owner,
		Root:       common.EmptyHash(), //firstTx.Hash(),
		GasLimit:   gasLimit,           // todo get gas limit dynamically
	}
	var b = block.NewBlock(newHeader)

	fmt.Printf("block hash: %s\r\n", b.Hash())
}

func (e *Engine) Validate(b *block.Block) {
	var err error
	if len(e.List) == 0 {
		var firstTx = coinbase.CreateCoinBaseTransation(C.Nonce, e.Owner)
		fmt.Printf("Coinbase tx hash: %s\r\n", firstTx.Hash())
		e.List = append(e.List, firstTx)
	}
	e.Transactions, err = NewTree(e.List)
	if err != nil {
		panic(err)
	}
	txsStatus, err := e.Transactions.VerifyTree()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Root hash: %s\r\n", common.BytesToHash(e.Transactions.Root.Hash))
	// fmt.Printf("Root hash: %s\r\n", e.Transactions.Root.tx.Hash())
	if txsStatus {
		b.Head.Root = common.Hash(e.Transactions.Root.Hash)
		for _, l := range e.Transactions.Leafs {
			var tx = l.tx
			if !l.dup {
				b.Head.Size += int(tx.Size())
				b.Transactions = append(b.Transactions, &tx)
			}
		}
	}

	var nBits = b.Head.Ctx
	var dif = b.Head.Difficulty

	var work = uint64(0)
	for {
		work++
		if dif.Cmp(b.Head.Hash().Big()) > 0 {
			b.Head.GasUsed += work
			break
		} else {
			b.Head.Difficulty = dif.Mul(dif, big.NewInt(int64(nBits)))
		}
	}

	e.List = nil

	// fmt.Printf("Final dif: %d\r\n", b.Head.Difficulty)
	// fmt.Printf("Final hash: %s\r\n", b.Hash())
	e.BlockPipe <- *b
}

func (e *Engine) Listen() {
	var errc chan error
	for errc == nil {
		select {
		case tx := <-e.TxFunnel:
			e.Pack(tx)
			// e.Transaions.
			// fmt.Println(tx.Hash())
		case b := <-e.BlockFunnel:
			fmt.Printf("New block arrived %s\r\n", b.Hash())
			e.Validate(b)
		}
	}
	errc <- nil
}

func (e *Engine) Pack(tx *types.GTransaction) {
	var err error
	// fmt.Printf("Rebuild with hash: %s\r\n", tx.Hash())
	if len(e.List) == 0 {
		var firstTx = coinbase.CreateCoinBaseTransation(C.Nonce, e.Owner)
		e.List = append(e.List, firstTx)
	}
	e.List = append(e.List, *tx)
	e.Transactions, err = NewTree(e.List)
	if err != nil {
		panic(err)
	}

	// var v, err = e.Transactions.VerifyTree()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Verify after pack: %t\r\n", v)

	// vv, err := e.Transactions.VerifyContent(tx)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Verify tx after pack: %t\r\n", vv)
	// fmt.Printf("Root node: %s\r\n", common.BytesToHash(e.Transactions.MerkleRoot()))

	// var traverse func(node *Node)
	// traverse = func(node *Node) {
	// 	if node == nil {
	// 		return
	// 	}
	// 	fmt.Println(node.String()) // Process the node (e.g., print it)
	// 	traverse(node.Left)
	// 	traverse(node.Right)
	// }

	// traverse(e.Transactions.Root)

}
