package storage

import (
	"github.com/tyler-smith/go-bip32"
)

type Keys struct {
	Priv *bip32.Key
	Pub  *bip32.Key
}

var k Keys

func SetKeys(priv *bip32.Key, pub *bip32.Key) error {
	k = Keys{
		Priv: priv,
		Pub:  pub,
	}
	return nil
}

func GetKeys() (*bip32.Key, *bip32.Key, error) {
	return k.Priv, k.Pub, nil
}
