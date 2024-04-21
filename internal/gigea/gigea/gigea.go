package gigea

import (
	"time"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/pool"
)

type Ring struct {
	Pool       *pool.Pool
	Chain      *chain.Chain
	Counter    int64
	RoundTimer *time.Ticker
}

var T Ring

func Get() Ring {
	return T
}
func (t Ring) Execute() {
	// var v = validator.Get()
	for {
		select {
		case <-t.RoundTimer.C:
			// fmt.Printf("Execute txs...\r\n")
			// if len(t.Pool.Prepared) > 0 {
			// 	for _, tx := range t.Pool.Prepared {
			// 		if v.ValidateTransaction(tx, tx.From()) {
			// 			t.Counter++
			// 			t.Pool.Executed = append(t.Pool.Executed, *tx)
			// 		}
			// 	}
			// }
			// t.Pool.Prepared = nil
		}
	}
}
