package storage

import (
	"github.com/cerera/core/account"
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

// RestoreAccounts replaces in-memory accounts from a prior snapshot (deep-cloned).
func (v *D5Vault) RestoreAccounts(snap AccountSnapshot) {
	if snap == nil || v == nil || v.accounts == nil {
		return
	}
	cloned := make(AccountSnapshot, len(snap))
	for addr, acc := range snap {
		cloned[addr] = cloneAccount(acc)
	}
	v.accounts.ReplaceAll(cloned)
}
