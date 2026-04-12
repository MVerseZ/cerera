package storage

import (
	"sync"

	"github.com/tyler-smith/go-bip32"
)

type Keys struct {
	Priv *bip32.Key
	Pub  *bip32.Key
}

var (
	keysMu sync.RWMutex
	k      Keys
)

func SetKeys(priv *bip32.Key, pub *bip32.Key) error {
	keysMu.Lock()
	defer keysMu.Unlock()
	k = Keys{
		Priv: priv,
		Pub:  pub,
	}
	return nil
}

func GetKeys() (*bip32.Key, *bip32.Key, error) {
	keysMu.RLock()
	defer keysMu.RUnlock()
	return k.Priv, k.Pub, nil
}
