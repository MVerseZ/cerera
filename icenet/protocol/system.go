package protocol

import (
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

// GetListenAddresses returns full multiaddrs for the local host (including /p2p/PeerID).
func GetListenAddresses(h host.Host) []string {
	if h == nil {
		return nil
	}
	addrs := h.Addrs()
	out := make([]string, 0, len(addrs))
	pid := h.ID()
	for _, a := range addrs {
		if a == nil {
			continue
		}
		out = append(out, a.String()+"/p2p/"+pid.String())
	}
	return out
}

func (h *Handler) handleSystemStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()
	if err := s.SetDeadline(time.Now().Add(StreamTimeout)); err != nil {
		return
	}

	decoder := NewDecoder(s)
	msg, err := decoder.Decode()
	if err != nil {
		if err != io.EOF {
			protocolLogger().Debugw("System stream ended", "peer", remotePeer, "error", err)
		}
		return
	}

	req, ok := msg.(*SystemPeerListRequest)
	if !ok {
		protocolLogger().Warnw("Unexpected system message", "peer", remotePeer, "type", msg.Type())
		return
	}
	if req.MsgType != MsgTypeSystemPeerList {
		return
	}

	var records []PeerRecordDTO
	if h.peerManager != nil {
		for _, rec := range h.peerManager.ConfirmedPeerRecords() {
			records = append(records, PeerRecordDTO{
				CereraAddress: rec.CereraAddress,
				PeerID:        rec.PeerID,
				ListenAddrs:   rec.ListenAddrs,
				Height:        rec.Height,
				IsReady:       rec.IsReady,
			})
		}
	}

	resp := NewSystemPeerListResponse(records)
	encoder := NewEncoder(s)
	if err := encoder.Encode(resp); err != nil {
		protocolLogger().Warnw("Failed to encode system peer list", "peer", remotePeer, "error", err)
	}
}
