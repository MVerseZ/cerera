package gigea

import (
	"fmt"
	"time"
)

type Ring struct {
	Counter    int64
	RoundTimer *time.Ticker
}

var T Ring

func Get() Ring {
	return T
}

func (t Ring) Execute() {
	// var v = validator.Get()
	// for {
	// 	select {
	// 	case <-t.RoundTimer.C:
	// 		// fmt.Printf("Execute txs...\r\n")
	// 		// if len(t.Pool.Prepared) > 0 {
	// 		// 	for _, tx := range t.Pool.Prepared {
	// 		// 		if v.ValidateTransaction(tx, tx.From()) {
	// 		// 			t.Counter++
	// 		// 			t.Pool.Executed = append(t.Pool.Executed, *tx)
	// 		// 		}
	// 		// 	}
	// 		// }
	// 		// t.Pool.Prepared = nil
	// 	}
	// }
}

func ExecuteCtl(code int) int {
	// get status of cerera
	// if not running - do nothing
	// else check some shit and execute with smth conditions
	//
	// may be check service as system service
	fmt.Printf("Execute command: %d\r\n", code)
	return 0
}
