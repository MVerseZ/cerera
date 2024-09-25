package pallada

import (
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
	// workaround
	// https://stackoverflow.com/questions/28447297/how-to-check-for-an-empty-struct
	if (Pallada{}) == pld {
		Prepare()
	}

	// get inner components
	// there is singletons and when call Get* returns struct of component
	var vlt = storage.GetVault()
	var bc = chain.GetBlockChain()
	var vldtr = validator.Get()
	var p = pool.Get()

	// rpc methods
	// these methods should not only using at rpc
	switch method {
	case "accounts", "account.getAll":
		// get all accounts of system
		pld.Data = vlt.GetAll()
	case "create_account", "account.create":
		// get all accounts of system
		//
		// name - just a name
		// passphrase - like a pass but optional now
		walletName, ok1 := params[0].(string)
		passphraseStr, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return 0xf
		}
		pk, m, addr, err := vlt.Create(walletName, passphraseStr)
		if err != nil {
			pld.Data = "Error"
			return 0xf
		}
		type res struct {
			Address  *types.Address `json:"address,omitempty"`
			Pub      string         `json:"pub,omitempty"`
			Mnemonic string         `json:"mnemonic,omitempty"`
		}
		pld.Data = &res{
			Address:  addr,
			Pub:      pk,
			Mnemonic: m,
		}
	case "get_minimum_gas_value", "chain.getMinimumGasValue":
		// get min gas value
		pld.Data = p.GetMinimalGasValue()
	case "get_balance", "account.getBalance":
		// get balance of address of account
		addressStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		var addr = types.HexToAddress(addressStr)
		pld.Data = types.BigIntToFloat(vlt.Get(addr).Balance)
	case "faucet":
		// faucet
		to, ok1 := params[0].(string)
		count, ok2 := params[1].(float64)
		if !ok1 || !ok2 {
			pld.Data = "Error"
			return 0xf
		}
		// var txHash, err = vldtr.Faucet(to, int(count))
		var err = vldtr.Faucet(to, int(count))
		if err != nil {
			pld.Data = err
			return 0xf
		}
		pld.Data = "SUCCESS"
	case "getblockchaininfo", "cerera.getInfo":
		// get info of (block)chain
		pld.Data = bc.GetInfo()
	case "getblockcount", "cerera.getBlockCount":
		// get latest block of chain
		pld.Data = bc.GetLatestBlock().Header().Number
	case "getblockhash", "cerera.getBlockHash":
		number, ok := params[0].(float64)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlockHash(int(number))
	case "getblock", "cerera.getBlock":
		// get block by hash
		blockHashStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlock(common.HexToHash(blockHashStr))
	case "getblockheader", "cerera.getBlockHeader":
		// get header by block hash
		blockHashStr, ok := params[0].(string)
		if !ok {
			pld.Data = "Error"
			return 0xf
		}
		pld.Data = bc.GetBlockHeader(blockHashStr)
	case "getmempoolinfo", "cerera.getMemPool":
		// get pool info
		pld.Data = p.GetInfo()
	case "signrawtransactionwithkey", "cerera.signTransaction":
		// sign transaction with key (signer will pay fees and value for transfer)
		if len(params) > 1 {
			txHashStr, ok1 := params[0].(string)
			kStr, ok2 := params[1].(string)
			if !ok1 || !ok2 {
				pld.Data = "Error"
				return 0xf
			}
			var txHash = common.HexToHash(txHashStr)
			resHash, err := vldtr.SignRawTransactionWithKey(txHash, kStr)
			if err != nil {
				pld.Data = "Error while sign tx"
				return 0xf
			}
			pld.Data = resHash
		} else {
			pld.Data = "Wrong count of params"
			return 0xf
		}
	case "send_tx", "cerera.sendTransaction":
		// send transaction to address

		// address
		// value
		// gas
		// message
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
	case "info", "cerera.getVersion":
		pld.Data = vldtr.GetVersion()

		// complexity of components very huge
	case "cerera.control.config":
		pld.Data = "Cerera configuration: "
	case "cerera.control.ipconfig":
		pld.Data = "Cerera network configuration: "
	default:
		pld.Data = "Method not supported"
	}
	return pld.Data
}
