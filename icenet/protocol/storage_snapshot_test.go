package protocol

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
)

func TestStorageSnapshotWireRoundtrip(t *testing.T) {
	priv, _ := types.GenerateAccount()
	addr := types.PubkeyToAddress(&priv.PublicKey)

	codeBlob, err := storage.EncodeContractCodeBlob(addr, []byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("EncodeContractCodeBlob: %v", err)
	}
	storageBlob := storage.EncodeContractStorageBlob(addr, big.NewInt(1), big.NewInt(2))

	var buf bytes.Buffer
	if err := writeStorageSnapshotResponse(&buf, [][]byte{codeBlob, storageBlob}, 2, 2, false); err != nil {
		t.Fatalf("writeStorageSnapshotResponse: %v", err)
	}

	chunk, err := readStorageSnapshotResponse(&buf, VaultSnapshotContractCode)
	if err != nil {
		t.Fatalf("readStorageSnapshotResponse code kind: %v", err)
	}
	if chunk.Total != 2 || len(chunk.Accounts) != 2 {
		t.Fatalf("unexpected chunk: %+v", chunk)
	}

	buf.Reset()
	if err := writeStorageSnapshotResponse(&buf, [][]byte{storageBlob}, 1, 1, false); err != nil {
		t.Fatalf("write storage chunk: %v", err)
	}
	chunk2, err := readStorageSnapshotResponse(&buf, VaultSnapshotContractStorage)
	if err != nil {
		t.Fatalf("readStorageSnapshotResponse storage kind: %v", err)
	}
	if len(chunk2.Accounts) != 1 || len(chunk2.Accounts[0]) != storage.ContractStorageWireSize() {
		t.Fatalf("unexpected storage chunk: %+v", chunk2)
	}
}

func TestVaultSyncNeeded(t *testing.T) {
	local := storage.VaultSyncStats{Accounts: 1, ContractCodes: 1, ContractSlots: 1}
	peer := Status{StorageData: 1, ContractCodes: 1, ContractSlots: 2}
	if !needsVaultSyncForTest(local, peer) {
		t.Fatal("expected vault sync needed when contract slots differ")
	}
}

func needsVaultSyncForTest(local storage.VaultSyncStats, peer Status) bool {
	return peer.StorageData > local.Accounts ||
		peer.ContractCodes > local.ContractCodes ||
		peer.ContractSlots > local.ContractSlots
}
