package observer

import "github.com/cerera/internal/cerera/types"

type Observer interface {
	Update(tx *types.GTransaction)
	GetID() string
}
