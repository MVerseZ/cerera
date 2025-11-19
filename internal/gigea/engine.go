package gigea

import (
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
)

type TxTree struct {
}

var E Engine

type Engine struct {
	TxFunnel       chan *types.GTransaction // input tx funnel
	BlockFunnel    chan *block.Block        // input block funnel
	BlockPipe      chan *block.Block
	Txs            TxTree
	Owner          types.Address
	Transactions   *TxMerkleTree
	List           []types.GTransaction
	MaintainTicker *time.Ticker
	stopCh         chan struct{} // Channel to signal shutdown

	// Consensus manager

	Port int
}

func (e *Engine) Start(lAddr types.Address) {
	// pipes
	e.BlockFunnel = make(chan *block.Block)
	e.TxFunnel = make(chan *types.GTransaction)
	e.BlockPipe = make(chan *block.Block)
	e.stopCh = make(chan struct{})

	e.List = make([]types.GTransaction, 0)

	e.Owner = lAddr
	e.MaintainTicker = time.NewTicker(1 * time.Minute)

	fmt.Println("Engine started")
	go e.Listen()
}

func (e *Engine) Listen() {
	for {
		select {
		case b := <-e.BlockFunnel:
			fmt.Printf("New block arrived to GIGEA: %s\r\n", b.GetHash())
			C.Notify(b)
		case <-e.MaintainTicker.C:
			// Maintain timer, utility methods
			// fmt.Printf("Maintain GIGEA\r\n\tCSP:[address: %s, state: %s]\r\n", G.address, G.state)

			// if e.ConsensusManager != nil {
			// 	consensusInfo := e.ConsensusManager.GetConsensusInfo()
			// 	fmt.Printf("\tConsensus Info: \r\n\t%+v\r\n", consensusInfo)
			// } else {
			// 	fmt.Printf("\tConsensus Info: \r\n\t%s\r\n", "Consensus manager not initialized or not running")
			// }

		case <-e.stopCh:
			fmt.Println("Engine stopping...")
			return
		}
	}
}

func (e *Engine) Register(a interface{}) {

}
