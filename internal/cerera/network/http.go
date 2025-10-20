package network

import (
	"github.com/cerera/internal/cerera/service"
)

// faucet
// to, ok1 := params[0].(string)
// if !ok1 {
// 	Result = "Error"
// 	return 0xf
// }
// // parse and validate address
// var addrTo = types.HexToAddress(to)
// if (addrTo == types.Address{}) {
// 	Result = "Invalid address"
// 	return 0xf
// }
// // construct faucet tx (state change will happen on application)
// var coinbaseTx = coinbase.FaucetTransaction(gigea.GetAndIncrementNonce(), addrTo)
// // enqueue to mempool
// go func() { p.Funnel <- []*types.GTransaction{coinbaseTx} }()
// Result = coinbaseTx

// case "send_tx", "cerera.sendTransaction":
// 	// send transaction to address

// 	// signer
// 	// address
// 	// value
// 	// gas
// 	// message
// 	if len(params) < 3 {
// 		Result = "Wrong count of params"
// 	} else {
// 		spk, ok0 := params[0].(string)
// 		addrStr, ok1 := params[1].(string)
// 		count, ok2 := params[2].(float64)
// 		gas, ok3 := params[3].(float64)
// 		msg, ok4 := params[4].(string)
// 		if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 {
// 			Result = "Error parse params"
// 			return 0xf
// 		} else {
// 			var addrTo = types.HexToAddress(addrStr)
// 			var gasInt = int(gas)
// 			tx, err := types.CreateUnbroadcastTransaction(gigea.GetAndIncrementNonce(), addrTo, count, uint64(gasInt), msg)
// 			if err != nil {
// 				Result = err
// 				return 0xf
// 			}
// 			tx, err = vldtr.SignRawTransactionWithKey(tx, spk)
// 			if err != nil {
// 				Result = err
// 				return 0xf
// 			}
// 			// go N.BroadcastTx(*tx)
// 			p.Funnel <- []*types.GTransaction{tx}
// 			Result = tx.Hash()
// 		}
// 	}

var Result interface{}

func Execute(method string, params []interface{}) interface{} {
	Result = service.Exec(method, params)
	return Result
}
