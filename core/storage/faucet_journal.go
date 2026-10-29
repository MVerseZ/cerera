package storage

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/akrylysov/pogreb"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

const (
	faucetJournalKeyPrefix = "faucet:"
	faucetMintWireSize     = 32 + 32 + 32 // address + amount + tx hash
)

func isFaucetJournalKey(key []byte) bool {
	return bytes.HasPrefix(key, []byte(faucetJournalKeyPrefix))
}

func isNonAccountVaultKey(key []byte) bool {
	return isContractOrStorageKey(key) || isFaucetJournalKey(key)
}

// EncodeFaucetMintWire serializes a faucet mint for P2P replication.
func EncodeFaucetMintWire(to types.Address, amount *big.Int, hash common.Hash) []byte {
	if amount == nil {
		amount = new(big.Int)
	}
	out := make([]byte, faucetMintWireSize)
	copy(out[0:32], to.Bytes())
	bal := amount.Bytes()
	padded := make([]byte, 32)
	if len(bal) > 32 {
		copy(padded, bal[len(bal)-32:])
	} else {
		copy(padded[32-len(bal):], bal)
	}
	copy(out[32:64], padded)
	copy(out[64:96], hash.Bytes())
	return out
}

func decodeFaucetMintWire(wire []byte) (faucetMint, error) {
	if len(wire) != faucetMintWireSize {
		return faucetMint{}, fmt.Errorf("faucet mint wire size %d, want %d", len(wire), faucetMintWireSize)
	}
	addr := types.BytesToAddress(wire[0:32])
	amount := new(big.Int).SetBytes(wire[32:64])
	hash := common.BytesToHash(wire[64:96])
	return faucetMint{To: addr, Amount: amount, Hash: hash}, nil
}

func faucetJournalDBKey(hash common.Hash) []byte {
	return []byte(faucetJournalKeyPrefix + hash.Hex())
}

// FaucetMintHook is invoked with EncodeFaucetMintWire payload after a new journal entry is stored.
var FaucetMintHook func(wire []byte)

func (v *D5Vault) hasFaucetMintHash(hash common.Hash) bool {
	if hash == (common.Hash{}) {
		return false
	}
	v.faucetMu.RLock()
	for _, m := range v.faucetMints {
		if m.Hash == hash {
			v.faucetMu.RUnlock()
			return true
		}
	}
	v.faucetMu.RUnlock()

	if v.inMem || v.db == nil {
		return false
	}
	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	has, err := v.db.Has(faucetJournalDBKey(hash))
	return err == nil && has
}

// AddFaucetMintRecord appends a mint to the replicated journal (dedup by tx hash).
func (v *D5Vault) AddFaucetMintRecord(m faucetMint) (added bool, err error) {
	return v.addFaucetMintRecord(m, true)
}

func (v *D5Vault) addFaucetMintRecord(m faucetMint, broadcast bool) (added bool, err error) {
	if m.Amount == nil || m.Amount.Sign() <= 0 {
		return false, ErrInvalidMintAmount
	}
	if v.hasFaucetMintHash(m.Hash) {
		return false, nil
	}

	v.faucetMu.Lock()
	v.faucetMints = append(v.faucetMints, faucetMint{
		To:     m.To,
		Amount: new(big.Int).Set(m.Amount),
		Hash:   m.Hash,
	})
	v.faucetMu.Unlock()

	if err := v.persistFaucetMint(m); err != nil {
		return false, err
	}

	if broadcast && FaucetMintHook != nil {
		FaucetMintHook(EncodeFaucetMintWire(m.To, m.Amount, m.Hash))
	}
	return true, nil
}

// ImportFaucetMintWire merges a peer faucet mint into the local journal without crediting until replay.
func (v *D5Vault) ImportFaucetMintWire(wire []byte) (added bool, err error) {
	m, err := decodeFaucetMintWire(wire)
	if err != nil {
		return false, err
	}
	return v.addFaucetMintRecord(m, false)
}

func (v *D5Vault) persistFaucetMint(m faucetMint) error {
	if v.inMem || v.db == nil {
		return nil
	}
	wire := EncodeFaucetMintWire(m.To, m.Amount, m.Hash)
	v.dbMu.Lock()
	defer v.dbMu.Unlock()
	return v.db.Put(faucetJournalDBKey(m.Hash), wire)
}

func (v *D5Vault) loadFaucetJournalFromDB() error {
	if v.inMem || v.db == nil {
		return nil
	}
	v.dbMu.RLock()
	defer v.dbMu.RUnlock()
	it := v.db.Items()
	var loaded []faucetMint
	for {
		key, val, err := it.Next()
		if err == pogreb.ErrIterationDone {
			break
		}
		if err != nil {
			return err
		}
		if !isFaucetJournalKey(key) {
			continue
		}
		m, err := decodeFaucetMintWire(val)
		if err != nil {
			continue
		}
		loaded = append(loaded, m)
	}
	if len(loaded) == 0 {
		return nil
	}
	v.faucetMu.Lock()
	v.faucetMints = append(v.faucetMints, loaded...)
	v.faucetMu.Unlock()
	return nil
}
