package network

import (
	"fmt"
	"strings"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea/gigea"
)

var Result interface{}

func Execute(method string, params []interface{}) interface{} {
	// workaround
	// https://stackoverflow.com/questions/28447297/how-to-check-for-an-empty-struct
	// if (Pallada{}) == pld {
	// 	Prepare()
	// }
	fmt.Printf("Request: %s\r\n", method)
	// get inner components
	// there is singletons and when call Get* returns struct of component
	var vlt = storage.GetVault()
	var bc = chain.GetBlockChain()
	var vldtr = validator.Get()
	var p = pool.Get()

	// rpc methods
	// these methods should not only using at rpc
	switch method {
	case "network":
		Result = "_SINGLE_NODE_"
	case "accounts", "account.getAll":
		// get all accounts of system
		Result = vlt.GetAll()
	case "accounts_cnt", "account.getCntAll":
		// get all accounts of system
		Result = vlt.GetCount()
	case "coinbase":
		Result = types.BigIntToFloat(coinbase.TotalValue)
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
		mk, pk, m, addr, err := vlt.Create(walletName, passphraseStr)
		if err != nil {
			Result = "Error"
			return 0xf
		}
		// network broadcast
		N.BroadcastAcc(vlt.Get(*addr))
		type res struct {
			Address  *types.Address `json:"address,omitempty"`
			Priv     string         `json:"priv,omitempty"`
			Pub      string         `json:"pub,omitempty"`
			Mnemonic string         `json:"mnemonic,omitempty"`
		}
		Result = &res{
			Address:  addr,
			Priv:     mk,
			Pub:      pk,
			Mnemonic: m,
		}
	case "restore_account", "account.restore":
		// restore account by wordlist
		mnemonic, ok1 := params[0].(string)
		pass, ok2 := params[1].(string)
		if !ok1 || !ok2 {
			return 0xf
		}
		if strings.Count(mnemonic, " ") != 23 {
			Result = "Wrong words count!"
			return 0xf
		}
		addr, mk, pk, err := vlt.Restore(mnemonic, pass)
		if err != nil {
			Result = "Error while restore"
			return 0xf
		}
		type res struct {
			Addr types.Address `json:"address,omitempty"`
			Priv string        `json:"priv,omitempty"`
			Pub  string        `json:"pub,omitempty"`
		}
		Result = &res{
			Priv: mk,
			Pub:  pk,
			Addr: addr,
		}
	case "get_minimum_gas_value", "chain.getMinimumGasValue":
		// get min gas value
		Result = p.GetMinimalGasValue()
	case "get_balance", "account.getBalance":
		// get balance of address of account
		addressStr, ok := params[0].(string)
		if !ok {
			Result = "Error"
			return 0xf
		}
		var addr = types.HexToAddress(addressStr)
		Result = types.BigIntToFloat(vlt.Get(addr).Balance)
	case "faucet":
		// faucet
		to, ok1 := params[0].(string)
		count, ok2 := params[1].(float64)
		if !ok1 || !ok2 {
			Result = "Error"
			return 0xf
		}
		// var txHash, err = vldtr.Faucet(to, int(count))
		var addrTo = types.HexToAddress(to)
		var coinbaseTx = coinbase.FaucetTransaction(gigea.C.Nonce, addrTo, count)
		p.Funnel <- []*types.GTransaction{coinbaseTx}
		go N.BroadcastTx(*coinbaseTx)
		Result = coinbaseTx
	case "getblockchaininfo", "cerera.getInfo":
		// get info of (block)chain
		Result = bc.GetInfo()
	case "getblockcount", "cerera.getBlockCount":
		// get latest block of chain
		Result = bc.GetLatestBlock().Header().Number
	case "getblockhash", "cerera.getBlockHash":
		number, ok := params[0].(float64)
		if !ok {
			Result = "Error"
			return 0xf
		}
		Result = bc.GetBlockByNumber(int(number))
	case "get_block", "cerera.getBlock":
		// get block by hash
		blockHashStr, ok := params[0].(string)
		if !ok {
			Result = "Error"
			return 0xf
		}
		Result = bc.GetBlock(common.HexToHash(blockHashStr))
	case "getblockheader", "cerera.getBlockHeader":
		// get header by block hash
		blockHashStr, ok := params[0].(string)
		if !ok {
			Result = "Error"
			return 0xf
		}
		Result = bc.GetBlockHeader(blockHashStr)
	case "getmempoolinfo", "cerera.getMemPool":
		// get pool info
		Result = p.GetInfo()
	case "signrawtransactionwithkey", "cerera.signTransaction":
		// sign transaction with key (signer will pay fees and value for transfer)
		Result = "Method not supported"
		return 0xe
		// if len(params) > 1 {
		// 	txHashStr, ok1 := params[0].(string)
		// 	kStr, ok2 := params[1].(string)
		// 	if !ok1 || !ok2 {
		// 		Result = "Error"
		// 		return 0xf
		// 	}
		// 	var txHash = common.HexToHash(txHashStr)
		// 	resHash, err := vldtr.SignRawTransactionWithKey(txHash, kStr)
		// 	if err != nil {
		// 		Result = "Error while sign tx"
		// 		return 0xf
		// 	}
		// 	Result = resHash
		// } else {
		// 	Result = "Wrong count of params"
		// 	return 0xf
		// }
	case "send_tx", "cerera.sendTransaction":
		// send transaction to address

		// signer
		// address
		// value
		// gas
		// message
		if len(params) < 3 {
			Result = "Wrong count of params"
		} else {
			_, ok0 := params[0].(string)
			addrStr, ok1 := params[1].(string)
			count, ok2 := params[2].(float64)
			gas, ok3 := params[3].(float64)
			msg, ok4 := params[4].(string)
			if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 {
				Result = "Error parse params"
				return 0xf
			} else {
				var addrTo = types.HexToAddress(addrStr)
				var gasInt = int(gas)
				tx, err := types.CreateUnbroadcastTransaction(gigea.C.Nonce, addrTo, count, uint64(gasInt), msg)
				if err != nil {
					Result = "Error while create transaction!"
					return 0xf
				}
				go N.BroadcastTx(*tx)
				p.Funnel <- []*types.GTransaction{tx}
				Result = tx.Hash()
				// // var tx = vldtr.PreSend(addrTo, count, uint64(gasInt), msg)
				// if vldtr.ValidateRawTransaction(tx) {
				// 	resTx, err := vldtr.SignRawTransactionWithKey(tx, kStr)
				// 	if err != nil {
				// 		Result = "Error while signing!"
				// 		return 0xf
				// 	}
				// 	// p.AddRawTransaction(tx)
				// 	p.Funnel <- []*types.GTransaction{resTx}
				// 	Result = resTx.Hash()
				// } else {
				// 	Result = types.EmptyCodeHash
				// }
			}
		}
	case "info", "cerera.getVersion":
		Result = vldtr.GetVersion()

		// complexity of components very huge
	case "cerera.control.config":
		Result = "Cerera configuration: "
	case "cerera.control.ipconfig":
		Result = "Cerera network configuration: "
	case "cerera.consensus.join":
		// guest use latest block for sync
		if len(params) != 1 {
			Result = "Wrong count of params"
		} else {
			addrStr, ok1 := params[0].(string)
			if !ok1 {
				Result = "Error parse params"
				return 0xf
			} else {
				var cereraClientAddress = types.HexToAddress(addrStr)
				if vldtr.CheckAddress(cereraClientAddress) {
					fmt.Printf("Address: %s\r\n", cereraClientAddress)
					bc.Idle()
					Result = fmt.Sprintf("LATEST#%d", bc.GetLatestBlock().Head.Index)
				} else {
					Result = "DONE"
				}
			}
		}
	case "cerera.consensus.sync":
		var result = make([]*block.Block, 0)
		for i := 0; i < bc.GetLatestBlock().Head.Height+1; i++ {
			var h = bc.GetBlockHash(i)
			var b = bc.GetBlock(h)
			result = append(result, b)
		}
		Result = result
	case "cerera.consensus.done":
		Result = "DONE"
	case "cerera.consensus.ready":
		bc.Resume()
		// guest use latest block for sync
		Result = bc.GetLatestBlock().Hash()
	case "cerera.consensus.block":

	default:
		Result = "Method not supported"
	}
	return Result
}
