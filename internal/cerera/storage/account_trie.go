package storage

import (
	"github.com/cerera/internal/cerera/types"
	"github.com/tyler-smith/go-bip32"
)

// structure stores account and other accounting stuff
// in smth like merkle-b-tree (cool data structure)
type AccountsTrie struct {
	accounts map[types.Address]types.StateAccount
}

func GetAccountsTrie() *AccountsTrie {
	// this smth like init function
	return &AccountsTrie{
		accounts: make(map[types.Address]types.StateAccount),
	}
}

// add account with address to Account Tree
func (at *AccountsTrie) Append(addr types.Address, sa types.StateAccount) {
	at.accounts[addr] = sa
}

func (at *AccountsTrie) GetAccount(addr types.Address) types.StateAccount {
	return at.accounts[addr]
}

func (at *AccountsTrie) GetKBytes(pubKey *bip32.Key) []byte {
	// for _, account := range at.accounts {
	// 	var fp = pubKey.FingerPrint
	// 	var cfp = account.MPub.FingerPrint
	// 	if bytes.Equal(fp, cfp) {
	// 		return account.CodeHash
	// 	}
	// }
	return nil
}

func (at *AccountsTrie) Size() int {
	return len(at.accounts)
}
