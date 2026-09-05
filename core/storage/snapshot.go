package storage

import (
	"math/big"
	"sync"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// AccountSnapshot is a point-in-time copy of in-memory accounts for dry-run rollback.
type AccountSnapshot map[types.Address]*account.StateAccount

func cloneAccount(acc *account.StateAccount) *account.StateAccount {
	if acc == nil {
		return nil
	}
	cp := *acc
	cp.SetBalanceBI(acc.GetBalanceBI())
	if acc.Inputs != nil {
		cp.Inputs = &account.Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int, len(acc.Inputs.M)),
		}
		acc.Inputs.RLock()
		for h, v := range acc.Inputs.M {
			if v != nil {
				cp.Inputs.M[h] = new(big.Int).Set(v)
			} else {
				cp.Inputs.M[h] = big.NewInt(0)
			}
		}
		acc.Inputs.RUnlock()
	}
	return &cp
}

// SnapshotAccounts clones all accounts currently held in memory.
func (v *D5Vault) SnapshotAccounts() AccountSnapshot {
	if v == nil || v.accounts == nil {
		return nil
	}
	v.accounts.mu.RLock()
	defer v.accounts.mu.RUnlock()
	snap := make(AccountSnapshot, len(v.accounts.accounts))
	for addr, acc := range v.accounts.accounts {
		snap[addr] = cloneAccount(acc)
	}
	return snap
}

// RestoreAccounts replaces in-memory accounts from a prior snapshot.
func (v *D5Vault) RestoreAccounts(snap AccountSnapshot) {
	if snap == nil || v == nil || v.accounts == nil {
		return
	}
	v.accounts.ReplaceAll(snap)
}
