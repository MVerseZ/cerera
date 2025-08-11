package gigea

import "github.com/cerera/internal/cerera/types"

// PublishConsensus is a function used by consensus algorithms to publish
// messages over the libp2p pubsub transport. It is injected by the net layer.
var PublishConsensus func(msg string)

// SetConsensusPublisher injects a publisher implementation from the net layer.
func SetConsensusPublisher(publisher func(msg string)) {
	PublishConsensus = publisher
}

// NotifyHeartbeat allows the net layer to notify consensus about a received
// heartbeat via pubsub without tight coupling to specific consensus types.
func NotifyHeartbeat(term int64, leader types.Address) {
	if E.ConsensusManager == nil {
		return
	}
	// Access underlying algorithm and call optional method if implemented
	if E.ConsensusManager != nil {
		if E.ConsensusManager.consensus != nil { // same package, can access
			if hb, ok := E.ConsensusManager.consensus.(interface{ NotifyHeartbeat(int64, types.Address) }); ok {
				hb.NotifyHeartbeat(term, leader)
			}
		}
	}
}

// NotifyVoteRequest forwards a vote request to the running consensus if supported
func NotifyVoteRequest(term int64, candidate types.Address) {
	if E.ConsensusManager == nil || E.ConsensusManager.consensus == nil {
		return
	}
	if vr, ok := E.ConsensusManager.consensus.(interface{ NotifyVoteRequest(int64, types.Address) }); ok {
		vr.NotifyVoteRequest(term, candidate)
	}
}

// NotifyVoteResponse forwards a vote response to the running consensus if supported
func NotifyVoteResponse(term int64, voter types.Address, granted bool) {
	if E.ConsensusManager == nil || E.ConsensusManager.consensus == nil {
		return
	}
	if vrs, ok := E.ConsensusManager.consensus.(interface {
		NotifyVoteResponse(int64, types.Address, bool)
	}); ok {
		vrs.NotifyVoteResponse(term, voter, granted)
	}
}

// NotifyLeaderAnnouncement forwards a leader announcement to the running consensus
func NotifyLeaderAnnouncement(term int64, leader types.Address) {
	if E.ConsensusManager == nil || E.ConsensusManager.consensus == nil {
		return
	}
	if la, ok := E.ConsensusManager.consensus.(interface{ NotifyLeaderAnnouncement(int64, types.Address) }); ok {
		la.NotifyLeaderAnnouncement(term, leader)
	}
}
