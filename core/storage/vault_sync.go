package storage

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/akrylysov/pogreb"
	"github.com/cerera/core/types"
)

const (
	codeKeyPrefix    = "code:"
	storageKeyPrefix = "storage:"

	addressWireSize           = 32
	contractStorageKeyWireSize = 32
	contractStorageValueWireSize = 32
	contractStorageWireSize     = addressWireSize + contractStorageKeyWireSize + contractStorageValueWireSize
	maxContractCodeBlobSize     = 16 * 1024 * 1024
)

// ContractStorageWireSize is the on-wire size of a contract storage slot blob.
func ContractStorageWireSize() int { return contractStorageWireSize }

const (
	storageDBKeyLen = len(storageKeyPrefix) + addressWireSize + 1 + contractStorageKeyWireSize
)

// VaultSyncStats summarizes vault rows relevant for P2P sync.
type VaultSyncStats struct {
	Accounts      int
	ContractCodes int
	ContractSlots int
}

type contractStorageSlot struct {
	address types.Address
	key     [32]byte
}

// VaultSyncStats returns current vault row counts for status exchange.
func (v *D5Vault) VaultSyncStats() VaultSyncStats {
	if v == nil {
		return VaultSyncStats{}
	}
	return VaultSyncStats{
		Accounts:      v.GetCount(),
		ContractCodes: v.contractCodeCount(),
		ContractSlots: v.contractStorageSlotCount(),
	}
}

func (v *D5Vault) contractCodeCount() int {
	return len(v.sortedContractCodeAddresses())
}

func (v *D5Vault) contractStorageSlotCount() int {
	return len(v.sortedContractStorageSlots())
}

func (v *D5Vault) sortedContractCodeAddresses() []types.Address {
	seen := make(map[types.Address]struct{})
	var addrs []types.Address

	v.contractMu.RLock()
	for addr := range v.contractCode {
		addrs = append(addrs, addr)
		seen[addr] = struct{}{}
	}
	v.contractMu.RUnlock()

	if !v.inMem && v.db != nil {
		v.dbMu.RLock()
		it := v.db.Items()
		for {
			key, _, err := it.Next()
			if err == pogreb.ErrIterationDone {
				break
			}
			if err != nil {
				break
			}
			addr, ok := parseContractCodeDBKey(key)
			if !ok {
				continue
			}
			if _, exists := seen[addr]; exists {
				continue
			}
			addrs = append(addrs, addr)
			seen[addr] = struct{}{}
		}
		v.dbMu.RUnlock()
	}

	sort.Slice(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i].Bytes(), addrs[j].Bytes()) < 0
	})
	return addrs
}

func (v *D5Vault) sortedContractStorageSlots() []contractStorageSlot {
	seen := make(map[string]struct{})
	var slots []contractStorageSlot

	v.contractMu.RLock()
	for addr, kv := range v.contractStorage {
		for keyStr, val := range kv {
			if val == nil {
				continue
			}
			var keyBytes [32]byte
			copy(keyBytes[:], []byte(keyStr))
			slots = append(slots, contractStorageSlot{address: addr, key: keyBytes})
			seen[storageSlotSeenKey(addr, keyBytes)] = struct{}{}
		}
	}
	v.contractMu.RUnlock()

	if !v.inMem && v.db != nil {
		v.dbMu.RLock()
		it := v.db.Items()
		for {
			key, _, err := it.Next()
			if err == pogreb.ErrIterationDone {
				break
			}
			if err != nil {
				break
			}
			addr, slotKey, ok := parseContractStorageDBKey(key)
			if !ok {
				continue
			}
			seenKey := storageSlotSeenKey(addr, slotKey)
			if _, exists := seen[seenKey]; exists {
				continue
			}
			slots = append(slots, contractStorageSlot{address: addr, key: slotKey})
			seen[seenKey] = struct{}{}
		}
		v.dbMu.RUnlock()
	}

	sort.Slice(slots, func(i, j int) bool {
		cmp := bytes.Compare(slots[i].address.Bytes(), slots[j].address.Bytes())
		if cmp != 0 {
			return cmp < 0
		}
		return bytes.Compare(slots[i].key[:], slots[j].key[:]) < 0
	})
	return slots
}

func storageSlotSeenKey(addr types.Address, key [32]byte) string {
	return addr.Hex() + string(key[:])
}

func parseContractCodeDBKey(key []byte) (types.Address, bool) {
	prefix := []byte(codeKeyPrefix)
	if !bytes.HasPrefix(key, prefix) || len(key) != len(prefix)+addressWireSize {
		return types.Address{}, false
	}
	return types.BytesToAddress(key[len(prefix):]), true
}

func parseContractStorageDBKey(key []byte) (types.Address, [32]byte, bool) {
	prefix := []byte(storageKeyPrefix)
	if !bytes.HasPrefix(key, prefix) || len(key) != storageDBKeyLen {
		return types.Address{}, [32]byte{}, false
	}
	rest := key[len(prefix):]
	if rest[addressWireSize] != ':' {
		return types.Address{}, [32]byte{}, false
	}
	addr := types.BytesToAddress(rest[:addressWireSize])
	var slotKey [32]byte
	copy(slotKey[:], rest[addressWireSize+1:])
	return addr, slotKey, true
}

// ExportContractCodeRange returns encoded contract code blobs sorted by address.
func (v *D5Vault) ExportContractCodeRange(offset, limit int) ([][]byte, int) {
	if v == nil || limit <= 0 {
		return nil, 0
	}
	addrs := v.sortedContractCodeAddresses()
	total := len(addrs)
	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	out := make([][]byte, 0, end-offset)
	for _, addr := range addrs[offset:end] {
		code, err := v.GetContractCode(addr)
		if err != nil || len(code) == 0 {
			continue
		}
		blob, err := EncodeContractCodeBlob(addr, code)
		if err != nil {
			continue
		}
		out = append(out, blob)
	}
	return out, total
}

// ExportContractStorageRange returns encoded contract storage slot blobs.
func (v *D5Vault) ExportContractStorageRange(offset, limit int) ([][]byte, int) {
	if v == nil || limit <= 0 {
		return nil, 0
	}
	slots := v.sortedContractStorageSlots()
	total := len(slots)
	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	out := make([][]byte, 0, end-offset)
	for _, slot := range slots[offset:end] {
		key := new(big.Int).SetBytes(slot.key[:])
		value, err := v.GetStorage(slot.address, key)
		if err != nil {
			continue
		}
		out = append(out, EncodeContractStorageBlob(slot.address, key, value))
	}
	return out, total
}

// EncodeContractCodeBlob encodes address || code for P2P transfer.
func EncodeContractCodeBlob(addr types.Address, code []byte) ([]byte, error) {
	if len(code) == 0 {
		return nil, fmt.Errorf("contract code cannot be empty")
	}
	if len(code) > maxContractCodeBlobSize {
		return nil, fmt.Errorf("contract code exceeds max size: %d", len(code))
	}
	out := make([]byte, addressWireSize+len(code))
	copy(out[:addressWireSize], addr.Bytes())
	copy(out[addressWireSize:], code)
	return out, nil
}

// DecodeContractCodeBlob decodes a contract code blob from P2P transfer.
func DecodeContractCodeBlob(blob []byte) (types.Address, []byte, error) {
	if len(blob) <= addressWireSize {
		return types.Address{}, nil, fmt.Errorf("contract code blob too short: %d", len(blob))
	}
	codeLen := len(blob) - addressWireSize
	if codeLen > maxContractCodeBlobSize {
		return types.Address{}, nil, fmt.Errorf("contract code blob too large: %d", codeLen)
	}
	addr := types.BytesToAddress(blob[:addressWireSize])
	code := append([]byte(nil), blob[addressWireSize:]...)
	return addr, code, nil
}

// EncodeContractStorageBlob encodes address || key || value for P2P transfer.
func EncodeContractStorageBlob(addr types.Address, key, value *big.Int) []byte {
	out := make([]byte, contractStorageWireSize)
	copy(out[:addressWireSize], addr.Bytes())
	if key != nil {
		key.FillBytes(out[addressWireSize : addressWireSize+contractStorageKeyWireSize])
	}
	if value != nil {
		value.FillBytes(out[addressWireSize+contractStorageKeyWireSize:])
	}
	return out
}

// DecodeContractStorageBlob decodes a contract storage blob from P2P transfer.
func DecodeContractStorageBlob(blob []byte) (types.Address, *big.Int, *big.Int, error) {
	if len(blob) != contractStorageWireSize {
		return types.Address{}, nil, nil, fmt.Errorf("invalid contract storage blob size: %d", len(blob))
	}
	addr := types.BytesToAddress(blob[:addressWireSize])
	key := new(big.Int).SetBytes(blob[addressWireSize : addressWireSize+contractStorageKeyWireSize])
	value := new(big.Int).SetBytes(blob[addressWireSize+contractStorageKeyWireSize:])
	return addr, key, value, nil
}

// SyncContractCodeBlob applies a contract code blob from a peer.
func (v *D5Vault) SyncContractCodeBlob(blob []byte) error {
	addr, code, err := DecodeContractCodeBlob(blob)
	if err != nil {
		return err
	}
	return v.StoreContractCode(addr, code)
}

// SyncContractStorageBlob applies a contract storage slot blob from a peer.
func (v *D5Vault) SyncContractStorageBlob(blob []byte) error {
	addr, key, value, err := DecodeContractStorageBlob(blob)
	if err != nil {
		return err
	}
	return v.SetStorage(addr, key, value)
}

// loadContractsFromDB populates in-memory contract maps from pogreb.
func (v *D5Vault) loadContractsFromDB() {
	if v.db == nil {
		return
	}

	v.contractMu.Lock()
	defer v.contractMu.Unlock()

	if v.contractCode == nil {
		v.contractCode = make(map[types.Address][]byte)
	}
	if v.contractStorage == nil {
		v.contractStorage = make(map[types.Address]map[string]*big.Int)
	}

	it := v.db.Items()
	for {
		key, val, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			vltlogger().Warnw("loadContractsFromDB: iteration failed", "err", err)
			break
		}

		if addr, ok := parseContractCodeDBKey(key); ok {
			v.contractCode[addr] = append([]byte(nil), val...)
			continue
		}

		if addr, slotKey, ok := parseContractStorageDBKey(key); ok {
			if v.contractStorage[addr] == nil {
				v.contractStorage[addr] = make(map[string]*big.Int)
			}
			keyStr := string(slotKey[:])
			v.contractStorage[addr][keyStr] = new(big.Int).SetBytes(val)
		}
	}
}
