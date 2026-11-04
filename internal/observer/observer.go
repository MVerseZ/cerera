package observer

import "github.com/cerera/core/pool"

// Observer notifies subscribers about pool transaction events.
type Observer = pool.TxObserver

// Source pushes transactions into the pool.
type Source = pool.TxSource
