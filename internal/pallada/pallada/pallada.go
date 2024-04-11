package pallada

import (
	"fmt"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
)

var pld Pallada

type Pallada struct {
	Data interface{}
}

func GetData() interface{} {
	return pld.Data
}

func Prepare() {
	pld = Pallada{}
}

func Execute(method string, params []interface{}) interface{} {
	if &pld == nil {
		Prepare()
	}

	var vlt = storage.GetVault()
	var bc = chain.GetBlockChain()
	var vldtr = validator.Get()
	var p = pool.Get()
	fmt.Println(p.Status)

	switch method {
	case "accounts":
		pld.Data = vlt.GetAll()
	case "create_account":
		walletName, ok1 := params[0].(string)
		passphraseStr, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return 0xf
		}
		pk, pb, addr, err := vlt.Create(walletName, passphraseStr)
		if err != nil {
			pld.Data = "Error"
			return 0xf
		}
		type res struct {
			Address *types.Address `json:"address,omitempty"`
			Priv    string         `json:"priv,omitempty"`
			Pub     string         `json:"pub,omitempty"`
		}
		pld.Data = &res{
			Address: addr,
			Priv:    pk,
			Pub:     pb,
		}
	case "get_minimum_gas_value":
		pld.Data = p.GetMinimalGasValue()
	case "get_balance":
		addressStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		var addr = types.HexToAddress(addressStr)
		pld.Data = types.BigIntToFloat(vlt.Get(addr).Balance)
	case "faucet":
		to, ok1 := params[0].(string)
		count, ok2 := params[1].(float64)
		if !ok1 || !ok2 {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = vldtr.Faucet(to, int(count))
	case "getblockchaininfo":
		pld.Data = bc.GetInfo()
	case "getblockcount":
		pld.Data = bc.GetLatestBlock().Header().Number
	case "getblockhash":
		number, ok := params[0].(float64)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlockHash(int(number))
	case "getblock":
		blockHashStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlock(blockHashStr)
	case "getblockheader":
		blockHashStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlockHeader(blockHashStr)
	case "getmempoolinfo":
		pld.Data = p.GetInfo()
	case "signrawtransactionwithkey":
		if len(params) > 1 {
			txHashStr, ok1 := params[0].(string)
			kStr, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				pld.Data = "Error"
				return 0xf
			}
			var txHash = common.HexToHash(txHashStr)
			pld.Data = vldtr.SignRawTransactionWithKey(txHash, kStr)
		} else {
			pld.Data = "Wrong count of params"
			return 0xf
		}
	case "send_tx":
		if len(params) < 2 {
			pld.Data = "Wrong count of params"
		} else {
			addrStr, ok1 := params[0].(string)
			count, ok2 := params[1].(float64)
			gas, ok3 := params[2].(float64)
			msg, ok4 := params[3].(string)
			if !ok1 || !ok2 || !ok3 || !ok4 {
				pld.Data = "Error parse params"
				return 0xf
			} else {
				var addrTo = types.HexToAddress(addrStr)
				var gasInt = int(gas)
				var tx = vldtr.PreSend(addrTo, count, uint64(gasInt), msg)
				if vldtr.ValidateRawTransaction(tx) {
					go func() { p.Funnel <- []*types.GTransaction{tx} }()
					// p.AddRawTransaction(tx)
					pld.Data = tx.Hash()
				} else {
					pld.Data = types.EmptyCodeHash
				}
			}
		}
	default:
		pld.Data = "Method not supperted"
	}
	return pld.Data
}
