package protocol

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/cerera/core/storage"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Binary vault snapshot protocol (one request / one response per stream).
//
// Request (9 bytes, big-endian):
//   uint8  kind   — 0=accounts, 1=contract code, 2=contract storage
//   uint32 offset — index into sorted export list for the selected kind
//   uint32 limit  — max entries to return (server clamps)
//
// Response:
//   uint32 total       — total entries on server for this kind
//   uint32 nextOffset  — offset to pass for the next chunk
//   uint8  more        — 1 if another chunk remains
//   uint32 count       — number of blobs following
//   count × (uint32 blobLen, blobLen bytes)

const (
	storageSnapshotDefaultLimit = 256
	storageSnapshotMaxLimit     = 512
	// MaxStorageAccountBlob caps a single serialized StateAccount on the wire.
	MaxStorageAccountBlob   = 16 * 1024 * 1024
	storageSnapshotReqSize  = 9
	storageSnapshotHdrSize  = 13
	storageSnapshotStreamTO = 5 * time.Minute
)

// VaultSnapshotKind selects which vault rows are exported in a snapshot chunk.
type VaultSnapshotKind uint8

const (
	VaultSnapshotAccounts         VaultSnapshotKind = 0
	VaultSnapshotContractCode       VaultSnapshotKind = 1
	VaultSnapshotContractStorage    VaultSnapshotKind = 2
)

// StorageSnapshotChunk is one page of vault blobs from a peer.
type StorageSnapshotChunk struct {
	Kind       VaultSnapshotKind
	Accounts   [][]byte
	Total      int
	NextOffset int
	More       bool
}

func clampStorageSnapshotParams(offset, limit int) (int, int) {
	if limit <= 0 {
		limit = storageSnapshotDefaultLimit
	}
	if limit > storageSnapshotMaxLimit {
		limit = storageSnapshotMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}

func (h *Handler) buildStorageSnapshotChunk(kind VaultSnapshotKind, offset, limit int) (accounts [][]byte, total, next int, more bool) {
	offset, limit = clampStorageSnapshotParams(offset, limit)
	if h.serviceProvider != nil {
		switch kind {
		case VaultSnapshotContractCode:
			accounts, total = h.serviceProvider.ExportStorageContractCodeRange(offset, limit)
		case VaultSnapshotContractStorage:
			accounts, total = h.serviceProvider.ExportStorageContractStorageRange(offset, limit)
		default:
			accounts, total = h.serviceProvider.ExportStorageAccountRange(offset, limit)
		}
	}
	next = offset + limit
	if next > total {
		next = total
	}
	more = next < total
	return accounts, total, next, more
}

func writeStorageSnapshotResponse(w io.Writer, accounts [][]byte, total, next int, more bool) error {
	hdr := make([]byte, storageSnapshotHdrSize)
	binary.BigEndian.PutUint32(hdr[0:4], uint32(total))
	binary.BigEndian.PutUint32(hdr[4:8], uint32(next))
	if more {
		hdr[8] = 1
	}
	binary.BigEndian.PutUint32(hdr[9:13], uint32(len(accounts)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	for _, blob := range accounts {
		if len(blob) > MaxStorageAccountBlob {
			return fmt.Errorf("snapshot blob exceeds max size: %d", len(blob))
		}
		var lb [4]byte
		binary.BigEndian.PutUint32(lb[:], uint32(len(blob)))
		if _, err := w.Write(lb[:]); err != nil {
			return err
		}
		if len(blob) > 0 {
			if _, err := w.Write(blob); err != nil {
				return err
			}
		}
	}
	return nil
}

func readStorageSnapshotResponse(r io.Reader, kind VaultSnapshotKind) (*StorageSnapshotChunk, error) {
	var hdr [storageSnapshotHdrSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read snapshot header: %w", err)
	}
	total := int(binary.BigEndian.Uint32(hdr[0:4]))
	next := int(binary.BigEndian.Uint32(hdr[4:8]))
	more := hdr[8] != 0
	n := int(binary.BigEndian.Uint32(hdr[9:13]))
	if n > storageSnapshotMaxLimit {
		return nil, fmt.Errorf("invalid snapshot blob count: %d", n)
	}
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		var lb [4]byte
		if _, err := io.ReadFull(r, lb[:]); err != nil {
			return nil, fmt.Errorf("read blob %d length: %w", i, err)
		}
		ln := int(binary.BigEndian.Uint32(lb[:]))
		if ln > MaxStorageAccountBlob {
			return nil, fmt.Errorf("invalid blob %d length: %d", i, ln)
		}
		if ln == 0 {
			out = append(out, nil)
			continue
		}
		b := make([]byte, ln)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, fmt.Errorf("read blob %d body: %w", i, err)
		}
		if err := validateSnapshotBlob(kind, b); err != nil {
			return nil, fmt.Errorf("invalid blob %d: %w", i, err)
		}
		out = append(out, b)
	}
	return &StorageSnapshotChunk{
		Kind:       kind,
		Accounts:   out,
		Total:      total,
		NextOffset: next,
		More:       more,
	}, nil
}

func validateSnapshotBlob(kind VaultSnapshotKind, blob []byte) error {
	switch kind {
	case VaultSnapshotContractCode:
		_, _, err := storage.DecodeContractCodeBlob(blob)
		return err
	case VaultSnapshotContractStorage:
		if len(blob) != storage.ContractStorageWireSize() {
			return fmt.Errorf("invalid contract storage blob size: %d", len(blob))
		}
		return nil
	default:
		return nil
	}
}

// handleStorageSnapshotStream serves one binary snapshot chunk per connection.
func (h *Handler) handleStorageSnapshotStream(s network.Stream) {
	defer s.Close()

	remotePeer := s.Conn().RemotePeer()
	protocolLogger().Debugw("Storage snapshot stream opened", "peer", remotePeer)

	if err := s.SetDeadline(time.Now().Add(storageSnapshotStreamTO)); err != nil {
		protocolLogger().Warnw("Failed to set storage snapshot deadline", "error", err)
		return
	}

	var req [storageSnapshotReqSize]byte
	if _, err := io.ReadFull(s, req[:]); err != nil {
		protocolLogger().Warnw("Failed to read storage snapshot request", "peer", remotePeer, "error", err)
		return
	}
	kind := VaultSnapshotKind(req[0])
	offset := int(binary.BigEndian.Uint32(req[1:5]))
	limit := int(binary.BigEndian.Uint32(req[5:9]))

	accounts, total, next, more := h.buildStorageSnapshotChunk(kind, offset, limit)
	if err := writeStorageSnapshotResponse(s, accounts, total, next, more); err != nil {
		protocolLogger().Warnw("Failed to write storage snapshot response", "peer", remotePeer, "error", err)
		return
	}

	protocolLogger().Debugw("Sent storage snapshot chunk (binary)",
		"peer", remotePeer,
		"kind", kind,
		"offset", offset,
		"count", len(accounts),
		"total", total,
		"more", more,
	)
}

// RequestStorageSnapshot fetches one chunk of vault rows over the binary storage-snapshot protocol.
func (h *Handler) RequestStorageSnapshot(ctx context.Context, peerID peer.ID, kind VaultSnapshotKind, offset, limit int) (*StorageSnapshotChunk, error) {
	s, err := h.host.NewStream(ctx, peerID, StorageSnapshotProtocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage snapshot stream: %w", err)
	}
	defer s.Close()

	if err := s.SetDeadline(time.Now().Add(storageSnapshotStreamTO)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	var req [storageSnapshotReqSize]byte
	req[0] = byte(kind)
	binary.BigEndian.PutUint32(req[1:5], uint32(offset))
	binary.BigEndian.PutUint32(req[5:9], uint32(limit))
	if _, err := s.Write(req[:]); err != nil {
		return nil, fmt.Errorf("failed to send storage snapshot request: %w", err)
	}

	return readStorageSnapshotResponse(s, kind)
}
