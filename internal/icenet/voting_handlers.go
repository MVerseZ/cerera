package icenet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/consensus"
	"github.com/cerera/internal/icenet/protocol"
)

// votingConsensus is the voting consensus manager
var votingConsensus *consensus.VotingConsensus

// initVotingConsensus initializes the voting consensus
func (i *Ice) initVotingConsensus() {
	if votingConsensus == nil {
		// Minimum votes = majority of known nodes (at least 2)
		minVotes := 2
		timeout := 30 * time.Second
		votingConsensus = consensus.NewVotingConsensus(minVotes, timeout)
		
		// Set mesh network for broadcasting
		votingConsensus.SetMeshNetwork(i.meshNetwork)
		
		// Add current node as voter
		votingConsensus.AddVoter(i.address)
	}
}

// handleProposalMessage handles a PROPOSAL message
func (i *Ice) handleProposalMessage(msg *protocol.ProposalMessage) {
	i.initVotingConsensus()
	
	icelogger().Infow("Received PROPOSAL message",
		"proposal_id", msg.ProposalID,
		"proposal_type", msg.ProposalType,
		"proposer", msg.Proposer.Hex(),
	)
	
	// Process proposal
	if err := votingConsensus.ProcessProposalMessage(msg); err != nil {
		icelogger().Warnw("Failed to process proposal",
			"proposal_id", msg.ProposalID,
			"err", err,
		)
		return
	}
	
	// Broadcast proposal to other nodes via gossip
	if i.meshNetwork != nil {
		if err := i.meshNetwork.BroadcastMessage(msg); err != nil {
			icelogger().Warnw("Failed to broadcast proposal",
				"proposal_id", msg.ProposalID,
				"err", err,
			)
		}
	}
}

// handleVoteMessage handles a VOTE message
func (i *Ice) handleVoteMessage(msg *protocol.VoteMessage) {
	i.initVotingConsensus()
	
	icelogger().Infow("Received VOTE message",
		"proposal_id", msg.ProposalID,
		"voter", msg.Voter.Hex(),
		"vote", msg.Vote,
	)
	
	// Process vote
	if err := votingConsensus.ProcessVoteMessage(msg); err != nil {
		icelogger().Warnw("Failed to process vote",
			"proposal_id", msg.ProposalID,
			"voter", msg.Voter.Hex(),
			"err", err,
		)
		return
	}
	
	// Broadcast vote to other nodes via gossip
	if i.meshNetwork != nil {
		if err := i.meshNetwork.BroadcastMessage(msg); err != nil {
			icelogger().Warnw("Failed to broadcast vote",
				"proposal_id", msg.ProposalID,
				"err", err,
			)
		}
	}
	
	// Check if proposal reached consensus
	proposal, exists := votingConsensus.GetProposal(msg.ProposalID)
	if exists && proposal.Result != nil {
		icelogger().Infow("Proposal reached consensus",
			"proposal_id", msg.ProposalID,
			"result", *proposal.Result,
		)
		
		// Handle proposal result
		i.handleProposalResult(proposal)
	}
}

// handleConsensusResultMessage handles a CONSENSUS_RESULT message
func (i *Ice) handleConsensusResultMessage(msg *protocol.ConsensusResultMessage) {
	i.initVotingConsensus()
	
	icelogger().Infow("Received CONSENSUS_RESULT message",
		"proposal_id", msg.ProposalID,
		"result", msg.Result,
		"votes_for", msg.VotesFor,
		"votes_against", msg.VotesAgainst,
	)
	
	// Process result
	if err := votingConsensus.ProcessConsensusResultMessage(msg); err != nil {
		icelogger().Warnw("Failed to process consensus result",
			"proposal_id", msg.ProposalID,
			"err", err,
		)
		return
	}
	
	// Get proposal and handle result
	proposal, exists := votingConsensus.GetProposal(msg.ProposalID)
	if exists {
		i.handleProposalResult(proposal)
	}
}

// handleProposalResult handles the result of a proposal
func (i *Ice) handleProposalResult(proposal *consensus.Proposal) {
	if proposal.Result == nil {
		return
	}
	
	if !*proposal.Result {
		icelogger().Infow("Proposal rejected",
			"proposal_id", proposal.ProposalID,
			"proposal_type", proposal.ProposalType,
		)
		return
	}
	
	// Handle approved proposal based on type
	switch proposal.ProposalType {
	case consensus.ProposalTypeNonce:
		i.handleNonceProposal(proposal)
	case consensus.ProposalTypeNodeAdd:
		i.handleNodeAddProposal(proposal)
	case consensus.ProposalTypeNodeRemove:
		i.handleNodeRemoveProposal(proposal)
	default:
		icelogger().Warnw("Unknown proposal type",
			"proposal_id", proposal.ProposalID,
			"proposal_type", proposal.ProposalType,
		)
	}
}

// handleNonceProposal handles a nonce change proposal
func (i *Ice) handleNonceProposal(proposal *consensus.Proposal) {
	var newNonce uint64
	if _, err := fmt.Sscanf(proposal.Data, "%d", &newNonce); err != nil {
		icelogger().Warnw("Failed to parse nonce from proposal",
			"proposal_id", proposal.ProposalID,
			"data", proposal.Data,
			"err", err,
		)
		return
	}
	
	// Update nonce in consensus manager
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	
	oldNonce := consensusManager.GetNonce()
	consensusManager.SetNonce(newNonce)
	
	icelogger().Infow("Nonce updated via voting consensus",
		"proposal_id", proposal.ProposalID,
		"old_nonce", oldNonce,
		"new_nonce", newNonce,
	)
}

// handleNodeAddProposal handles a node add proposal
func (i *Ice) handleNodeAddProposal(proposal *consensus.Proposal) {
	// Parse node data from proposal
	var nodeData struct {
		Address     types.Address
		NetworkAddr string
	}
	if err := json.Unmarshal([]byte(proposal.Data), &nodeData); err != nil {
		icelogger().Warnw("Failed to parse node data from proposal",
			"proposal_id", proposal.ProposalID,
			"err", err,
		)
		return
	}
	
	// Add node to consensus
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	
	if err := addNodeToConsensus(nodeData.Address, nodeData.NetworkAddr, consensusManager); err != nil {
		icelogger().Warnw("Failed to add node via voting consensus",
			"proposal_id", proposal.ProposalID,
			"node_address", nodeData.Address.Hex(),
			"err", err,
		)
		return
	}
	
	// Add to peer store
	if i.peerStore != nil {
		i.peerStore.AddOrUpdate(nodeData.Address, nodeData.NetworkAddr)
	}
	
	// Add as voter
	if votingConsensus != nil {
		votingConsensus.AddVoter(nodeData.Address)
	}
	
	icelogger().Infow("Node added via voting consensus",
		"proposal_id", proposal.ProposalID,
		"node_address", nodeData.Address.Hex(),
	)
}

// handleNodeRemoveProposal handles a node remove proposal
func (i *Ice) handleNodeRemoveProposal(proposal *consensus.Proposal) {
	// Parse node address from proposal
	var nodeAddr types.Address
	if err := json.Unmarshal([]byte(proposal.Data), &nodeAddr); err != nil {
		// Try parsing as hex string
		nodeAddr = types.HexToAddress(proposal.Data)
		if nodeAddr == (types.Address{}) {
			icelogger().Warnw("Failed to parse node address from proposal",
				"proposal_id", proposal.ProposalID,
				"data", proposal.Data,
			)
			return
		}
	}
	
	// Remove from peer store
	if i.peerStore != nil {
		i.peerStore.Remove(nodeAddr)
	}
	
	// Remove as voter
	if votingConsensus != nil {
		votingConsensus.RemoveVoter(nodeAddr)
	}
	
	icelogger().Infow("Node removed via voting consensus",
		"proposal_id", proposal.ProposalID,
		"node_address", nodeAddr.Hex(),
	)
}

// handlePeerDiscoveryMessage handles a PEER_DISCOVERY message
func (i *Ice) handlePeerDiscoveryMessage(msg *protocol.PeerDiscoveryMessage, conn *connection.Connection) {
	icelogger().Infow("Received PEER_DISCOVERY message",
		"requester", msg.Requester.Hex(),
		"max_peers", msg.MaxPeers,
	)
	
	// Get best peers from peer store
	if i.peerStore == nil {
		return
	}
	
	peers := i.peerStore.GetBestPeers(msg.MaxPeers)
	if len(peers) == 0 {
		return
	}
	
	// Create NODES message with peer information
	nodeInfos := make([]protocol.NodeInfo, 0, len(peers))
	for _, peer := range peers {
		nodeInfos = append(nodeInfos, protocol.NodeInfo{
			Address:     peer.Address,
			NetworkAddr: peer.NetworkAddr,
		})
	}
	
	nodesMsg := &protocol.NodesMessage{Nodes: nodeInfos}
	
	// Send response
	handler := i.connManager.GetHandler()
	if err := handler.WriteMessage(conn, nodesMsg); err != nil {
		icelogger().Warnw("Failed to send nodes in response to PEER_DISCOVERY",
			"requester", msg.Requester.Hex(),
			"err", err,
		)
	}
}

// ProposeNonceChange proposes a nonce change via voting consensus
func (i *Ice) ProposeNonceChange(newNonce uint64) error {
	i.initVotingConsensus()
	
	proposalID, err := votingConsensus.ProposeNonceChange(newNonce, i.address)
	if err != nil {
		return fmt.Errorf("failed to propose nonce change: %w", err)
	}
	
	// Create and broadcast proposal message
	proposalMsg := &protocol.ProposalMessage{
		ProposalID:   proposalID,
		ProposalType: string(consensus.ProposalTypeNonce),
		Data:         fmt.Sprintf("%d", newNonce),
		Proposer:     i.address,
		Timestamp:    time.Now().Unix(),
	}
	
	if i.meshNetwork != nil {
		if err := i.meshNetwork.BroadcastMessage(proposalMsg); err != nil {
			return fmt.Errorf("failed to broadcast proposal: %w", err)
		}
	}
	
	icelogger().Infow("Proposed nonce change via voting consensus",
		"proposal_id", proposalID,
		"new_nonce", newNonce,
	)
	
	return nil
}
