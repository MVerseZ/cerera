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
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"

	"github.com/cerera/internal/cerera/types"
)

var v Validator

func Get() Validator {
	return v
}

type Validator interface {
	GasPrice() *big.Int
	GetVersion() string
	Faucet(addrStr string, valFor int) error
	PreSend(to types.Address, value float64, gas uint64, msg string) *types.GTransaction
	SetUp(chainId *big.Int)
	Signer() types.Signer
	SignRawTransactionWithKey(txHash common.Hash, kStr string) (common.Hash, error)
	ValidateRawTransaction(tx *types.GTransaction) bool
	// validate and execute transaction
	ValidateTransaction(t *types.GTransaction, from types.Address) bool
	ValidateBlock(b block.Block) bool
}

type DDDDDValidator struct {
	currentStatus  int
	minGasPrice    *big.Int
	storage        string
	signatureKey   *ecdsa.PrivateKey
	signer         types.Signer
	balance        *big.Int
	currentVersion string
}

func NewValidator(ctx context.Context, cfg config.Config) Validator {
	var p = types.DecodePrivKey(cfg.NetCfg.PRIV)
	v = &DDDDDValidator{
		signatureKey:   p,
		signer:         types.NewSimpleSignerWithPen(cfg.Chain.ChainID, p),
		balance:        big.NewInt(0), // Initialize balance
		currentVersion: "ALPHA-0.0.1",
	}
	return v
}

func (v *DDDDDValidator) GetVersion() string {
	return v.currentVersion
}

func (v *DDDDDValidator) GasPrice() *big.Int {
	return v.minGasPrice
}

func (v *DDDDDValidator) Faucet(addrStr string, valFor int) error {
	if valFor > 0 {
		var vault = storage.GetVault()
		vault.FaucetBalance(types.HexToAddress(addrStr), valFor)
		return nil
	}
	return errors.New("value < 0")
}

func (v *DDDDDValidator) SetUp(chainId *big.Int) {
	v.minGasPrice = big.NewInt(100)
	v.signer = types.NewSimpleSignerWithPen(chainId, v.signatureKey)
}

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

func (v *DDDDDValidator) Signer() types.Signer {
	return v.signer
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

func (v *DDDDDValidator) SignRawTransactionWithKey(txHash common.Hash, signKey string) (common.Hash, error) {
	p := pool.Get()
	fmt.Println(txHash)
	fmt.Println(signKey)
	var tx = p.GetTransaction(txHash)
	fmt.Println(tx.IsSigned())

	// get for tx
	v.balance.Add(v.balance, big.NewInt(int64(tx.Gas())))

	// sign tx
	var vlt = storage.GetVault()
	var signBytes = vlt.GetKey(signKey)

	pemBlock, _ := pem.Decode([]byte(signBytes))
	aKey, err1 := x509.ParseECPrivateKey(pemBlock.Bytes)
	if err1 != nil {
		return common.EmptyHash(), errors.New("error ParsePKC58 key")
	}

	signTx, err2 := types.SignTx(tx, v.signer, aKey)
	if err2 != nil {
		fmt.Printf("Error while sign tx: %s\r\n", tx.Hash())
		return common.EmptyHash(), errors.New("error while sign tx")
	}
	var r, vv, s = signTx.RawSignatureValues()
	fmt.Printf("Raw values:%d %d %d\r\n", r, s, vv)
	// update tx in mempool
	p.UpdateTx(*signTx)

	// p.memPool[i] = *signTx
	// network.PublishData("OP_TX_SIGNED", tx)
	fmt.Printf("Now tx %s is %t\r\n", signTx.Hash(), signTx.IsSigned())
	return signTx.Hash(), nil
}

func (v *DDDDDValidator) ValidateBlock(b block.Block) bool {
	// when validator says that block is correct, node get reward for it
	// it should be automatic as same level with autogen alogrythm of chain
	// if block.Confirmations > 2 then node gets reward
	return true
}
