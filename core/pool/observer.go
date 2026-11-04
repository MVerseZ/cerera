package pool

import "github.com/cerera/core/types"

// TxObserver receives notifications when transactions enter the pool.
type TxObserver interface {
	Update(tx *types.GTransaction)
	GetID() string
}

// TxSource pushes transactions into the pool from an external origin.
type TxSource interface {
	Notify(tx *types.GTransaction)
	GetID() string
}
