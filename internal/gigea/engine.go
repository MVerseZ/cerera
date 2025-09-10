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
	Txs            TxTree
	Owner          types.Address
	Transactions   *TxMerkleTree
	List           []types.GTransaction
	MaintainTicker *time.Ticker
	stopCh         chan struct{} // Channel to signal shutdown

	// Consensus manager
	ConsensusManager *ConsensusManager

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

	// Initialize consensus manager with default peers
	peers := []*PeerInfo{{
		CereraAddress: lAddr,
		NetworkAddr:   fmt.Sprintf("localhost:%d", e.Port),
	}} // Start with just this node
	e.ConsensusManager = NewConsensusManager(ConsensusTypeGigea, lAddr, peers, e)
	e.ConsensusManager.Start()
	fmt.Println("Engine started")
	go e.Listen()
}

func (e *Engine) Listen() {
	for {
		select {
		case tx := <-e.TxFunnel:
			if err := e.Pack(tx); err != nil {
				fmt.Printf("Error packing transaction: %v\r\n", err)
				continue
			}
			// Submit transaction to consensus
			if e.ConsensusManager != nil {
				e.ConsensusManager.SubmitRequest(fmt.Sprintf("transaction:%s", tx.Hash().Hex()))
			}
		case b := <-e.BlockFunnel:
			fmt.Printf("New block arrived to GIGEA: %s\r\n", b.GetHash())
			C.Notify(b)
		case <-e.MaintainTicker.C:
			fmt.Printf("Maintain GIGEA\r\n\tCSP:[address: %s, state: %s]\r\n", G.address, G.state)

			if e.ConsensusManager != nil {
				consensusInfo := e.ConsensusManager.GetConsensusInfo()
				fmt.Printf("\tConsensus Info: \r\n\t%+v\r\n", consensusInfo)
			} else {
				fmt.Printf("\tConsensus Info: \r\n\t%s\r\n", "Consensus manager not initialized or not running")
			}

		case <-e.stopCh:
			fmt.Println("Engine stopping...")
			return
		}
	}
}

func (e *Engine) Pack(tx *types.GTransaction) error {
	var err error
	// fmt.Printf("Rebuild with hash: %s\r\n", tx.Hash())
	if len(e.List) == 0 {
		var firstTx = coinbase.CreateCoinBaseTransation(C.Nonce, e.Owner)
		e.List = append(e.List, firstTx)
	}
	e.List = append(e.List, *tx)
	e.Transactions, err = NewTree(e.List)
	if err != nil {
		fmt.Printf("Error creating transaction tree: %v\r\n", err)
		return err
	}

	return nil
}

func (e *Engine) Register(a interface{}) {

}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() {
	if e.MaintainTicker != nil {
		e.MaintainTicker.Stop()
	}
	if e.ConsensusManager != nil && e.ConsensusManager.NetworkManager != nil {
		e.ConsensusManager.NetworkManager.Stop()
	}
	close(e.stopCh)
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

// GetNetworkInfo returns network information
func (e *Engine) GetNetworkInfo() map[string]interface{} {
	if e.ConsensusManager != nil {
		return e.ConsensusManager.GetNetworkInfo()
	}
	return map[string]interface{}{
		"error": "Consensus manager not initialized",
	}
}

// AddPeer adds a peer to the consensus
func (e *Engine) AddPeer(peer types.Address) {
	if e.ConsensusManager != nil {
		peerInfo := &PeerInfo{
			CereraAddress: peer,
			NetworkAddr:   fmt.Sprintf(":%d", e.Port), // Use current node's network address
		}
		e.ConsensusManager.AddPeer(peerInfo)
	}
	// Also update in existing C.Voters for backward compatibility
	isPresent := false
	for i, voter := range C.Voters {
		if voter == peer {
			C.Voters[i] = peer
			isPresent = true
			break
		}
	}
	if !isPresent {
		C.Voters = append(C.Voters, peer)
	}
}

func (e *Engine) UpdatePeer(peer types.Address) {
	if e.ConsensusManager != nil {
		peerInfo := &PeerInfo{
			CereraAddress: peer,
			NetworkAddr:   fmt.Sprintf("localhost:%d", e.Port), // Use current node's port
		}
		e.ConsensusManager.AddPeer(peerInfo)
	}
	// Also update in existing C.Voters for backward compatibility
	isPresent := false
	for i, voter := range C.Voters {
		if voter == peer {
			C.Voters[i] = peer
			isPresent = true
			break
		}
	}
	if !isPresent {
		C.Voters = append(C.Voters, peer)
	}
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

// GetConnectedPeers returns connected peers
func (e *Engine) GetConnectedPeers() []types.Address {
	if e.ConsensusManager != nil && e.ConsensusManager.NetworkManager != nil {
		return e.ConsensusManager.NetworkManager.GetConnectedPeers()
	}
	return []types.Address{}
}

// GetTotalPeers returns total peers
func (e *Engine) GetTotalPeers() []types.Address {
	if e.ConsensusManager != nil && e.ConsensusManager.NetworkManager != nil {
		return e.ConsensusManager.NetworkManager.GetPeers()
	}
	return []types.Address{}
}
