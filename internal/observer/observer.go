package observer

import "github.com/cerera/core/types"

type Observer interface {
	Update(tx *types.GTransaction)
	GetID() string
}
