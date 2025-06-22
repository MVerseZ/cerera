package gigea

import (
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/coinbase"
)

type TxTree struct {
}

var E Engine

type Engine struct {
	TxFunnel       chan *types.GTransaction // input tx funnel
	BlockFunnel    chan *block.Block        // input block funnel
	BlockPipe      chan *block.Block
	Transaions     TxTree
	Owner          types.Address
	Transactions   *TxMerkleTree
	List           []types.GTransaction
	MaintainTicker *time.Ticker

	// Consensus manager
	ConsensusManager *ConsensusManager
}

func (e *Engine) Start(lAddr types.Address) {
	// pipes
	e.BlockFunnel = make(chan *block.Block)
	e.TxFunnel = make(chan *types.GTransaction)
	e.BlockPipe = make(chan *block.Block)

	e.List = make([]types.GTransaction, 0)

	e.Owner = lAddr
	e.MaintainTicker = time.NewTicker(1 * time.Second)

	// Initialize consensus manager with default peers
	peers := []types.Address{lAddr} // Start with just this node
	e.ConsensusManager = NewConsensusManager(ConsensusTypePBFT, lAddr, peers, e)
	e.ConsensusManager.Start()

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

func (e *Engine) Listen() {
	var errc chan error
	for errc == nil {
		select {
		case tx := <-e.TxFunnel:
			e.Pack(tx)
			// Submit transaction to consensus
			if e.ConsensusManager != nil {
				e.ConsensusManager.SubmitRequest(fmt.Sprintf("transaction:%s", tx.Hash().Hex()))
			}
			// e.Transaions.
			// fmt.Println(tx.Hash())
			// case b := <-e.BlockFunnel:
			// fmt.Printf("New block arrived %s\r\n", b.Hash())
			// e.Validate(b)
		case b := <-e.BlockFunnel:
			fmt.Printf("New block arrived to GIGEA: %s\r\n", b.GetHash())
			C.Notify(b)
			continue
		case <-e.MaintainTicker.C:
			fmt.Printf("Maintain GIGEA\r\n\tCSP:[address: %s, state: %s]\r\n", G.address, G.state)

			// Update consensus state based on consensus manager
			if e.ConsensusManager != nil {
				consensusInfo := e.ConsensusManager.GetConsensusInfo()
				fmt.Printf("Consensus Info: %+v\r\n", consensusInfo)

				// Update GIGEA state based on consensus
				if e.ConsensusManager.IsLeader() {
					if G.state != Leader {
						G.state = Leader
						fmt.Printf("Consensus manager indicates this node is leader, changing state to Leader\r\n")
					}
				} else {
					if len(C.Voters) <= 1 {
						if G.state != Candidate {
							G.state = Candidate
							fmt.Printf("No connections detected, changing state to Candidate\r\n")
						}
						if len(C.Voters) <= 1 && G.state == Candidate {
							G.state = Leader
							fmt.Printf("No connections detected, changing state to Leader\r\n")
						}
					} else {
						if G.state != Follower {
							G.state = Follower
							fmt.Printf("Consensus manager indicates this node is not leader, changing state to Follower\r\n")
						}
					}
				}
			} else {
				// Fallback to original logic
				if G.state != Leader {
					if len(C.Voters) <= 1 {
						G.state = Candidate
						fmt.Printf("No connections detected, changing state to Candidate\r\n")
					}
					if len(C.Voters) <= 1 && G.state == Candidate {
						G.state = Leader
						fmt.Printf("No connections detected, changing state to Leader\r\n")
					}
				}
			}
			continue
		}
		errc <- nil
	}
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

func (e *Engine) Register(a interface{}) {

}

// SetConsensusType sets the consensus algorithm type
func (e *Engine) SetConsensusType(consensusType ConsensusType) {
	if e.ConsensusManager != nil {
		e.ConsensusManager.SwitchConsensus(consensusType)
	}
}

// GetConsensusInfo returns consensus information
func (e *Engine) GetConsensusInfo() map[string]interface{} {
	if e.ConsensusManager != nil {
		return e.ConsensusManager.GetConsensusInfo()
	}
	return map[string]interface{}{
		"error": "Consensus manager not initialized",
	}
}

// AddPeer adds a peer to the consensus
func (e *Engine) AddPeer(peer types.Address) {
	if e.ConsensusManager != nil {
		e.ConsensusManager.AddPeer(peer)
	}
	// Also add to existing C.Voters for backward compatibility
	C.Voters = append(C.Voters, peer)
}

// RemovePeer removes a peer from the consensus
func (e *Engine) RemovePeer(peer types.Address) {
	if e.ConsensusManager != nil {
		e.ConsensusManager.RemovePeer(peer)
	}
	// Also remove from existing C.Voters for backward compatibility
	for i, voter := range C.Voters {
		if voter == peer {
			C.Voters = append(C.Voters[:i], C.Voters[i+1:]...)
			break
		}
	}
}
