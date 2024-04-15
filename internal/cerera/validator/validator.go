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
	Faucet(addrStr string, valFor int) (common.Hash, error)
	//	LoadChain() ([]*block.Block, error)
	//	RewardSignature() *ecdsa.PrivateKey
	//	Stamp() *ecdsa.PrivateKey
	PreSend(to types.Address, value float64, gas uint64, msg string) *types.GTransaction
	SetUp(chainId *big.Int)
	//	Status() int
	//	Stop()
	Signer() types.Signer
	SignRawTransactionWithKey(txHash common.Hash, kStr string) (common.Hash, error)
	ValidateRawTransaction(tx *types.GTransaction) bool
	ValidateTransaction(t *types.GTransaction, from types.Address) bool
	ValidateBlock(b block.Block) bool
	//	ValidateGenesis(b *block.Block)
	//	WriteBlock(b block.Block) (common.Hash, error)
}

type DDDDDValidator struct {
	currentStatus int
	minGasPrice   *big.Int
	storage       string
	signatureKey  *ecdsa.PrivateKey
	signer        types.Signer
}

func NewValidator(ctx context.Context, cfg config.Config) Validator {
	var p = types.DecodePrivKey(cfg.NetCfg.PRIV)
	v = &DDDDDValidator{
		signatureKey: p,
		signer:       types.NewSimpleSignerWithPen(cfg.Chain.ChainID, p),
	}
	return v
}

func (v *DDDDDValidator) GasPrice() *big.Int {
	return v.minGasPrice
}
func (v *DDDDDValidator) Faucet(addrStr string, valFor int) (common.Hash, error) {
	var faucetTx = types.NewTransaction(
		1,
		types.HexToAddress(addrStr),
		types.FloatToBigInt(float64(valFor)),
		10000,
		big.NewInt(10000000),
		[]byte("faucet transaction"),
	)

	// Send the signed transaction to the pool
	txHash, err := pool.SendTransaction(*faucetTx)
	if err != nil {
		return common.EmptyHash(), err
	}

	// Sign the transaction
	derBytes, _ := x509.MarshalECPrivateKey(v.signatureKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBytes})
	signedTxHash, err := v.SignRawTransactionWithKey(txHash, string(pemEncoded))
	if err != nil {
		return common.EmptyHash(), err
	}

	if signedTxHash.Compare(txHash) != 0 {
		return common.EmptyHash(), errors.New("different hashes!")
	}
	return txHash, nil
}

func (v *DDDDDValidator) SetUp(chainId *big.Int) {
	v.minGasPrice = big.NewInt(100)
	v.signer = types.NewSimpleSignerWithPen(chainId, v.signatureKey)
}

func (v *DDDDDValidator) PreSend(to types.Address, value float64, gas uint64, msg string) *types.GTransaction {
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
			"APPROVED\r\n\tSigned transaction with hash=%s\r\n\t gas=%d\r\n value=%d\r\n  current balance=%d\r\n",
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
	hash, err := p.SignRawTransaction(txHash, v.Signer(), signKey)
	if err != nil {
		return common.EmptyHash(), err
	}
	return hash, nil
}

func (v *DDDDDValidator) ValidateBlock(b block.Block) bool {
	return true
}

//	func (v *DDDDDValidator) LoadChain() ([]*block.Block, error) {
//		return v.storage.LoadInitialBlocks()
//	}
//
//	func (v *DDDDDValidator) GetLatestBlock() *block.Block {
//		return v.storage.GetLatestBlock()
//	}
//
//	func (v *DDDDDValidator) RewardSignature() *ecdsa.PrivateKey {
//		return v.signatureKey
//	}
//
//	func (v *DDDDDValidator) Start() {
//		v.current_status = 7
//	}
//
//	func (v *DDDDDValidator) Stop() {
//		v.current_status = 13
//	}
//
//	func (v *DDDDDValidator) Status() int {
//		return v.current_status
//	}
//
//	func (v *DDDDDValidator) Stamp() *ecdsa.PrivateKey {
//		// may be autogen if not exist and write???
//		return v.signatureKey
//	}
//
//	func (validator *DDDDDValidator) ValidateRawTransaction(tx *types.GTransaction) bool {
//		// no edit tx here again
//		// TODO
//		return true
//	}

//func (v *DDDDDValidator) ValidateBlock(b block.Block) bool {
//	return true
//}
//func (v *DDDDDValidator) ValidateGenesis(b *block.Block) {
//	var tmpBuf, err = v.storage.Get([]byte("GENESIS"))
//	if err != nil {
//		v.storage.Write([]byte("GENESIS"), b)
//		v.storage.Write([]byte("LATEST"), b)
//	} else {
//		latestB := &block.Block{}
//		err = json.Unmarshal(tmpBuf, latestB)
//		if err != nil {
//			panic(err)
//		}
//		b = latestB
//	}
//
//}
//func (v *DDDDDValidator) WriteBlock(b block.Block) (common.Hash, error) {
//	return v.storage.WriteBlock(b)
//}
