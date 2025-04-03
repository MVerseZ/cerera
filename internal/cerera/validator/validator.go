package validator

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/coinbase"

	"github.com/cerera/internal/cerera/types"
)

var (
	EmptyCoinbase    = &decError{"empty hex string"}
	NotEnoughtInputs = &decError{"not enought inputs"}
)

type decError struct{ msg string }

func (err decError) Error() string { return err.msg }

var v Validator

func Get() Validator {
	return v
}

type Validator interface {
	CheckAddress(addr types.Address) bool
	GasPrice() *big.Int
	GetVersion() string
	ExecuteTransaction(tx types.GTransaction) error
	// Faucet(addrStr string, valFor int) error
	PreSend(to types.Address, value float64, gas uint64, msg string) *types.GTransaction

	SetUp(chainId *big.Int)
	Signer() types.Signer
	SignRawTransactionWithKey(tx *types.GTransaction, kStr string) (*types.GTransaction, error)
	ValidateRawTransaction(tx *types.GTransaction) bool
	// validate and execute transaction
	ValidateTransaction(t *types.GTransaction, from types.Address) bool
	ValidateBlock(b block.Block) bool
}

type DDDDDValidator struct {
	currentAddress types.Address
	currentStatus  int
	minGasPrice    *big.Int
	storage        string
	signatureKey   *ecdsa.PrivateKey
	signer         types.Signer
	balance        *big.Int
	currentVersion string
}

func NewValidator(ctx context.Context, cfg config.Config) (Validator, error) {
	var p = types.DecodePrivKey(cfg.NetCfg.PRIV)
	v = &DDDDDValidator{
		signatureKey:   p,
		signer:         types.NewSimpleSignerWithPen(big.NewInt(int64(cfg.Chain.ChainID))), //, p),
		balance:        big.NewInt(0),                                                      // Initialize balance
		currentVersion: "ALPHA-0.0.1",
		currentAddress: cfg.NetCfg.ADDR,
	}
	return v, nil
}

func (v *DDDDDValidator) CheckAddress(addr types.Address) bool {
	// move logic to consensus
	return v.currentAddress != addr
}

func (v *DDDDDValidator) GetVersion() string {
	return v.currentVersion
}

func (v *DDDDDValidator) GasPrice() *big.Int {
	return v.minGasPrice
}

func (v *DDDDDValidator) ExecuteTransaction(tx types.GTransaction) error {
	// if send to address not generated - > send only to input
	var localVault = storage.GetVault()
	var gas = tx.Gas()
	var val = tx.Value()
	var out = coinbase.GetCoinbaseBalance()
	var delta = big.NewInt(0).Sub(out, val)
	if delta.Cmp(big.NewInt(0)) < 0 {
		return EmptyCoinbase
	} else {
		fmt.Printf(
			"APPROVED\r\n\tSigned transaction with hash=%s\r\n\t gas=%d\r\n\t value=%f\r\n\t  current balance=%d\r\n",
			tx.Hash(),
			gas,
			types.BigIntToFloat(val),
			out,
		)
		fmt.Printf("\t\t reaylly signed?%t\r\n", tx.IsSigned())
		switch tx.Type() {
		case types.LegacyTxType:
			fmt.Printf("\t\t legacy from %s\r\n", tx.From())
			localVault.UpdateBalance(tx.From(), *tx.To(), val, tx.Hash())
		case types.FaucetTxType:
			fmt.Printf("\t\t faucet from %s\r\n", tx.From())
			localVault.DropFaucet(*tx.To(), val, tx.Hash())
		case types.CoinbaseTxType:
			fmt.Printf("\t\t coinbase from %s\r\n", tx.From())
			localVault.UpdateBalance(coinbase.GetCoinbaseAddress(), *tx.To(), val, tx.Hash())
		default:
			fmt.Printf("\t\t unknown from %s\r\n", tx.From())
		}

	}
	return nil
}

// func (v *DDDDDValidator) Faucet(addrStr string, valFor int) error {
// 	if valFor > 0 {
// 		var vault = storage.GetVault()
// 		return vault.FaucetBalance(types.HexToAddress(addrStr), valFor)
// 	}
// 	return errors.New("value < 0")
// }

func (v *DDDDDValidator) PreSend(to types.Address, value float64, gas uint64, msg string) *types.GTransaction {
	// here we create transaction by input values
	var tx = types.NewTransaction(
		1,
		to,
		types.FloatToBigInt(value),
		gas,
		v.GasPrice(),
		[]byte(msg),
	)
	return tx
}

func (v *DDDDDValidator) SetUp(chainId *big.Int) {
	v.minGasPrice = big.NewInt(100)
	v.signer = types.NewSimpleSignerWithPen(chainId) //, v.signatureKey)
}

func (v *DDDDDValidator) Signer() types.Signer {
	return v.signer
}

func (v *DDDDDValidator) SignRawTransactionWithKey(tx *types.GTransaction, signKey string) (*types.GTransaction, error) {
	fmt.Printf("Start signing tx\r\n")
	// p := pool.Get()
	// fmt.Println(txHash)
	// fmt.Println(signKey)
	// var tx = p.GetTransaction(txHash)
	fmt.Println(tx.IsSigned())

	// get for tx
	v.balance.Add(v.balance, big.NewInt(int64(tx.Gas())))

	// sign tx
	var vlt = storage.GetVault()
	var signBytes = vlt.GetKey(signKey)

	pemBlock, _ := pem.Decode([]byte(signBytes))
	aKey, err1 := x509.ParseECPrivateKey(pemBlock.Bytes)
	if err1 != nil {
		return nil, errors.New("error ParsePKC58 key")
	}
	fmt.Printf("Sing tx: %s\r\n", tx.Hash())

	signTx, err2 := types.SignTx(tx, v.signer, aKey)
	if err2 != nil {
		fmt.Printf("Error while sign tx: %s\r\n", tx.Hash())
		return nil, errors.New("error while sign tx")
	}
	var r, vv, s = signTx.RawSignatureValues()
	fmt.Printf("Raw values:%d %d %d\r\n", r, s, vv)
	// update tx in mempool WHY ???
	// p.UpdateTx(signTx)

	// p.memPool[i] = *signTx
	// network.PublishData("OP_TX_SIGNED", tx)
	fmt.Printf("Now tx %s is %t\r\n", signTx.Hash(), signTx.IsSigned())
	// check existing inputs

	fmt.Printf("\tcheck tx: %s\r\n", tx.Hash())
	var bFrom, bTo, bVal = vlt.Get(tx.From()).Balance, vlt.Get(*tx.To()).Balance, tx.Value()
	fmt.Printf("\tbalance src %s\r\n", bFrom)
	fmt.Printf("\tbalance dest %s\r\n", bTo)
	fmt.Printf("\tamount to transfer: %d\r\n", bVal)
	// fmt.Printf("\tsrc after transfer: %d\r\n", big.NewInt(0).Sub(bFrom, bVal))
	// fmt.Printf("\tsrc after transfer: %f\r\n", types.BigIntToFloat(big.NewInt(0).Sub(bFrom, bVal)))
	// fmt.Printf("\tsrc after transfer: %t\r\n", types.BigIntToFloat(big.NewInt(0).Sub(bFrom, bVal)) < 0.0)

	if types.BigIntToFloat(big.NewInt(0).Sub(bFrom, bVal)) < 0.0 {
		return nil, NotEnoughtInputs
	}
	return signTx, nil
}

func (v *DDDDDValidator) ValidateBlock(b block.Block) bool {
	// move logic to consensus
	// return consensus.ConfirmBlock(b)
	return true
}

// Validate and execute transaction
func (validator *DDDDDValidator) ValidateTransaction(tx *types.GTransaction, from types.Address) bool {
	// no edit tx here !!!
	// check user can send signed tx
	// this function should be rewriting and simplified by refactoring onto n functions
	// logic now semicorrect, false only arythmetic errors
	var localVault = storage.GetVault()
	var r, s, _ = tx.RawSignatureValues()
	fmt.Printf("Sender is: %s\r\n", from)
	var gas = tx.Gas()
	var val = tx.Value()
	var out = localVault.Get(from).Balance
	var delta = big.NewInt(0).Sub(out, val)
	if delta.Cmp(big.NewInt(0)) < 0 {
		return false
	} else {
		fmt.Printf(
			"APPROVED\r\n\tSigned transaction with hash=%s\r\n\t gas=%d\r\n\t value=%d\r\n\t  current balance=%d\r\n",
			tx.Hash(),
			gas,
			val,
			out,
		)
		localVault.UpdateBalance(from, *tx.To(), val, tx.Hash())
	}
	localVault.CheckRunnable(r, s, tx)
	return true
}

func (validator *DDDDDValidator) ValidateRawTransaction(tx *types.GTransaction) bool {
	return true
}
