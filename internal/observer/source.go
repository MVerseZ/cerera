package observer

import "github.com/cerera/core/types"

type Source interface {
	Notify(tx *types.GTransaction)
	GetID() string
}
