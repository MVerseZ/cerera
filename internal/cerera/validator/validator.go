package validator

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"os"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea"

	"github.com/cerera/internal/cerera/types"

	"github.com/prometheus/client_golang/prometheus"
)

const VALIDATOR_SERVICE_NAME = "CERERA_VALIDATOR_54013.10.25"

var (
	EmptyCoinbase    = &decError{"empty hex string"}
	NotEnoughtInputs = &decError{"not enought inputs"}
)

// vlogger is a dedicated console logger for validator
var vlogger = log.New(os.Stdout, "[validator] ", log.LstdFlags|log.Lmicroseconds)

var (
	valTxCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_tx_created_total",
		Help: "Total number of transactions created",
	})
	valTxValidated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_tx_validated_total",
		Help: "Total number of transactions validated successfully",
	})
	valTxRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_tx_rejected_total",
		Help: "Total number of transactions rejected during validation",
	})
	valExecuteSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_execute_success_total",
		Help: "Total number of executed transactions successfully applied",
	})
	valExecuteError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_execute_error_total",
		Help: "Total number of transaction execution errors",
	})
	valSignSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_sign_success_total",
		Help: "Total number of successfully signed transactions",
	})
	valSignError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "validator_sign_error_total",
		Help: "Total number of transaction signing errors",
	})
)

func init() {
	prometheus.MustRegister(
		valTxCreated,
		valTxValidated,
		valTxRejected,
		valExecuteSuccess,
		valExecuteError,
		valSignSuccess,
		valSignError,
	)
}

type decError struct{ msg string }

func (err decError) Error() string { return err.msg }

var v Validator

type Validator interface {
	CheckAddress(addr types.Address) bool
	GasPrice() *big.Int
	GetVersion() string
	Exec(method string, params []interface{}) interface{}
	ExecuteTransaction(tx types.GTransaction) error
	// Faucet(addrStr string, valFor int) error
	CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas float64, message string) (*types.GTransaction, error)

	SetUp(chainId *big.Int)
	ServiceName() string
	Signer() types.Signer
	SignRawTransactionWithKey(tx *types.GTransaction, kStr string) error
	Status() byte

	ValidateRawTransaction(tx *types.GTransaction) bool
	// validate and execute transaction
	ValidateTransaction(t *types.GTransaction, from types.Address) bool
	ValidateBlock(b block.Block) bool
	// observer methods
	GetID() string
	Update(tx *types.GTransaction)

	// REF
}

type CoreValidator struct {
	currentAddress types.Address
	currentStatus  int
	minGasPrice    *big.Int
	storage        string
	signatureKey   *ecdsa.PrivateKey
	signer         types.Signer
	balance        *big.Int
	currentVersion string
	servChannel    chan []byte
}

// Exec DTOs (typed request objects)
type CreateTxParams struct {
	Key    string
	Nonce  uint64
	To     types.Address
	Amount string // decimal string CER, e.g. "1.23"
	Gas    float64
	Msg    string
}

type SendTxParams struct {
	Key    string
	ToHex  string
	Amount string // decimal string CER
	Gas    float64
	Msg    string
}

func Get() Validator {
	return v
}

func NewValidator(ctx context.Context, cfg config.Config) (Validator, error) {
	var p = types.DecodePrivKey(cfg.NetCfg.PRIV)
	v = &CoreValidator{
		signatureKey:   p,
		signer:         types.NewSimpleSigner(big.NewInt(int64(cfg.Chain.ChainID))),
		balance:        big.NewInt(0), // Initialize balance
		currentVersion: "ALPHA-0.0.1",
		currentAddress: cfg.NetCfg.ADDR,
	}
	// Ensure validator invariants are initialized
	v.SetUp(big.NewInt(int64(cfg.Chain.ChainID)))
	// Configure min gas price from config
	v.(*CoreValidator).minGasPrice = types.FloatToBigInt(cfg.POOL.MinGas)
	return v, nil
}

func (v *CoreValidator) CheckAddress(addr types.Address) bool {
	// move logic to consensus
	return v.currentAddress != addr
}

func (v *CoreValidator) CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas float64, message string) (*types.GTransaction, error) {
	// here we create transaction by input values
	tx, err := types.CreateUnbroadcastTransaction(nonce, addressTo, count, gas, message)
	if err != nil {
		return nil, err
	}
	// calculate fee and add to value
	valTxCreated.Inc()
	return tx, nil
}

func (v *CoreValidator) ExecuteTransaction(tx types.GTransaction) error {
	// if send to address not generated - > send only to input
	// executed transaction adds to txs trie struct
	var localVault = storage.GetVault()
	var val = tx.Value()

	// check if sender has enough balance
	senderAcc := localVault.Get(tx.From())
	if senderAcc == nil {
		return NotEnoughtInputs
	}

	// Handle different transaction types
	switch tx.Type() {
	case types.CoinbaseTxType:
		// Coinbase transactions: reward goes directly to miner
		// Create shadow account for miner if it doesn't exist
		minerAcc := localVault.Get(*tx.To())
		if minerAcc == nil {
			minerAcc = types.NewStateAccount(*tx.To(), 0, common.Hash{})
			localVault.Put(*tx.To(), minerAcc)
		}
		// Update coinbase balance (debit from coinbase)
		coinbaseAcc := localVault.Get(coinbase.GetCoinbaseAddress())
		if coinbaseAcc != nil {
			if big.NewInt(0).Sub(coinbaseAcc.GetBalanceBI(), val).Cmp(val) == 1 {
				newCoinbaseBal := new(big.Int).Sub(coinbaseAcc.GetBalanceBI(), val)
				coinbaseAcc.SetBalanceBI(newCoinbaseBal)

				// Credit reward to miner
				newMinerBal := new(big.Int).Add(minerAcc.GetBalanceBI(), val)
				minerAcc.SetBalanceBI(newMinerBal)
				minerAcc.AddInput(tx.Hash(), val)
			}
		}

		// Persist changes if not in memory mode
		// Note: vault persistence is handled internally by the vault

	case types.FaucetTxType:
		// Faucet transactions: no sender balance check needed
		localVault.DropFaucet(*tx.To(), val, tx.Hash())

	case types.LegacyTxType:
		// Regular transactions: check sender balance and deduct gas
		gasCost := tx.Cost()
		totalDebit := new(big.Int).Add(new(big.Int).Set(val), gasCost)

		// Check sender balance using big.Int - ensure sender exists
		senderAcc := localVault.Get(tx.From())
		if senderAcc == nil {
			return NotEnoughtInputs
		}
		senderBal := senderAcc.GetBalanceBI()
		if senderBal.Cmp(totalDebit) < 0 {
			return NotEnoughtInputs
		}

		// Validate gas cost
		if v.minGasPrice != nil && gasCost.Sign() > 0 && gasCost.Cmp(v.minGasPrice) < 0 {
			return errors.New("gas cost below minimum")
		}

		// Deduct gas from sender (gas is burned)
		senderAcc.SetBalanceBI(new(big.Int).Sub(senderBal, gasCost))

		// Transfer value to recipient (UpdateBalance will deduct value from sender and add to recipient)
		localVault.UpdateBalance(tx.From(), *tx.To(), val, tx.Hash())

	default:
		vlogger.Printf("unknown tx type: %d from %s", tx.Type(), tx.From())
	}

	valExecuteSuccess.Inc()
	return nil
}

// func (v *DDDDDValidator) Faucet(addrStr string, valFor int) error {
// 	if valFor > 0 {
// 		var vault = storage.GetVault()
// 		return vault.FaucetBalance(types.HexToAddress(addrStr), valFor)
// 	}
// 	return errors.New("value < 0")
// }

func (v *CoreValidator) GasPrice() *big.Int {
	return v.minGasPrice
}

func (v *CoreValidator) GetID() string {
	return v.currentAddress.String()
}

func (v *CoreValidator) GetVersion() string {
	return v.currentVersion
}

func (v *CoreValidator) ServiceName() string {
	return VALIDATOR_SERVICE_NAME
}

func (v *CoreValidator) SetUp(chainId *big.Int) {
	// default min gas price; can be overridden from config in NewValidator
	v.minGasPrice = types.FloatToBigInt(0.000001)
	v.signer = types.NewSimpleSigner(chainId)
}

func (v *CoreValidator) Signer() types.Signer {
	return v.signer
}

func (v *CoreValidator) SignRawTransactionWithKey(tx *types.GTransaction, signKey string) error {
	// fmt.Printf("Start signing tx\r\n")
	// p := pool.Get()
	// fmt.Println(txHash)
	// fmt.Println(signKey)
	// var tx = p.GetTransaction(txHash)
	// fmt.Println(tx.IsSigned())

	// get for tx
	v.balance.Add(v.balance, big.NewInt(int64(tx.Gas())))

	// sign tx
	if signKey == "" {
		valSignError.Inc()
		return errors.New("empty signing key id")
	}
	var vlt = storage.GetVault()
	var signBytes = vlt.GetKey(signKey)
	if len(signBytes) == 0 {
		valSignError.Inc()
		return errors.New("signing key not found in vault")
	}
	pemBlock, _ := pem.Decode([]byte(signBytes))
	if pemBlock == nil || len(pemBlock.Bytes) == 0 {
		valSignError.Inc()
		return errors.New("invalid PEM block for private key")
	}
	var aKey *ecdsa.PrivateKey
	if k, err := x509.ParseECPrivateKey(pemBlock.Bytes); err == nil {
		aKey = k
	} else {
		if anyKey, err2 := x509.ParsePKCS8PrivateKey(pemBlock.Bytes); err2 == nil {
			if ecKey, ok := anyKey.(*ecdsa.PrivateKey); ok {
				aKey = ecKey
			} else {
				valSignError.Inc()
				return errors.New("PKCS8 key is not ECDSA private key")
			}
		} else {
			valSignError.Inc()
			return errors.New("unable to parse ECDSA private key: not EC or PKCS8 ECDSA")
		}
	}
	// fmt.Printf("Sing tx: %s\r\n", tx.Hash())
	signTx, err2 := types.SignTx(tx, v.signer, aKey)
	if err2 != nil {
		vlogger.Printf("error while sign tx: %s", tx.Hash())
		valSignError.Inc()
		return errors.New("error while sign tx")
	}
	//var r, vv, s =
	signTx.RawSignatureValues()
	// fmt.Printf("Raw values:%d %d %d\r\n", r, s, vv)
	// update tx in mempool if needed
	// p.UpdateTx(signTx)

	// p.memPool[i] = *signTx
	// network.PublishData("OP_TX_SIGNED", tx)
	// fmt.Printf("Now tx %s isSigned status: %t\r\n", signTx.Hash(), signTx.IsSigned())
	// check existing inputs

	// Signing does not perform balance/gas affordability checks.
	// Validation is handled separately in ValidateTransaction.
	valSignSuccess.Inc()
	return nil
}

func (v *CoreValidator) Status() byte {
	return 0xa
}

func (v *CoreValidator) Update(tx *types.GTransaction) {
	// update validator state
}

func (v *CoreValidator) ValidateBlock(b block.Block) bool {
	// move logic to consensus
	// return consensus.ConfirmBlock(b)
	return true
}

func (validator *CoreValidator) ValidateRawTransaction(tx *types.GTransaction) bool {
	return true
}

// Validate and execute transaction
// TODO GAS
func (validator *CoreValidator) ValidateTransaction(tx *types.GTransaction, from types.Address) bool {
	// no edit tx here !!!
	// check user can send signed tx
	// this function should be rewriting and simplified by refactoring onto n functions
	// logic now semicorrect, false only arythmetic errors
	var localVault = storage.GetVault()
	var r, s, _ = tx.RawSignatureValues()
	// fmt.Printf("Sender is: %s\r\n", from)
	// var gas = tx.Gas()
	var val = tx.Value()
	gasCost := tx.Cost()
	need := new(big.Int).Add(new(big.Int).Set(val), gasCost)
	senderAcc := localVault.Get(from)
	if senderAcc == nil {
		valTxRejected.Inc()
		return false
	}
	senderBal := senderAcc.GetBalanceBI()
	if senderBal.Cmp(need) < 0 {
		valTxRejected.Inc()
		return false
	}
	//else {
	// fmt.Printf(
	// 	"APPROVED_TX_TYPE_%d\r\n\tSigned transaction with hash=%s\r\n\t gas=%d\r\n\t value=%d\r\n\t  current balance=%d\r\n",
	// 	tx.Type(),
	// 	tx.Hash(),
	// 	gas,
	// 	val,
	// 	out,
	// )
	// localVault.UpdateBalance(from, *tx.To(), val, tx.Hash())
	//}
	localVault.CheckRunnable(r, s, tx)
	valTxValidated.Inc()
	return true
}

func (v *CoreValidator) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "create":
		// Prefer typed DTO in params[0]
		if len(params) == 1 {
			if p, ok := params[0].(CreateTxParams); ok {
				if p.Gas < 0 {
					return errors.New("negative gas or value")
				}
				wei, err := types.DecimalStringToWei(p.Amount)
				if err != nil {
					return err
				}
				tx, err := types.CreateUnbroadcastTransactionWei(p.Nonce, p.To, wei, p.Gas, p.Msg)
				if err != nil {
					return err
				}
				if err := v.SignRawTransactionWithKey(tx, p.Key); err != nil {
					return err
				}
				return tx
			}
		}
		// Fallback: legacy positional parameters
		if len(params) < 6 {
			return errors.New("invalid parameters for create")
		}
		key, ok0 := params[0].(string)
		nonce, ok1 := params[1].(uint64)
		to, ok2 := params[2].(types.Address)
		count, ok3 := params[3].(float64)
		gas, ok4 := params[4].(float64)
		msg, ok5 := params[5].(string)
		if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
			return errors.New("parameter type mismatch for create")
		}
		tx, err := types.CreateUnbroadcastTransaction(nonce, to, count, gas, msg)
		if err != nil {
			return err
		}
		if err := v.SignRawTransactionWithKey(tx, key); err != nil {
			return err
		}
		return tx
	case "send":
		// Prefer typed DTO in params[0]
		if len(params) == 1 {
			if p, ok := params[0].(SendTxParams); ok {
				if p.Gas < 0 {
					return errors.New("negative gas or value")
				}
				addrTo := types.HexToAddress(p.ToHex)
				wei, err := types.DecimalStringToWei(p.Amount)
				if err != nil {
					return err
				}
				tx, err := types.CreateUnbroadcastTransactionWei(gigea.GetAndIncrementNonce(), addrTo, wei, p.Gas, p.Msg)
				if err != nil {
					return err
				}
				if err := v.SignRawTransactionWithKey(tx, p.Key); err != nil {
					return err
				}
				pool.Get().QueueTransaction(tx)
				return tx.Hash()
			}
		}
		// Fallback: legacy positional parameters
		if len(params) < 5 {
			return errors.New("invalid parameters for send")
		}
		spk, ok0 := params[0].(string)
		addrStr, ok1 := params[1].(string)
		count, ok2 := params[2].(float64)
		gas, ok3 := params[3].(float64)
		msg, ok4 := params[4].(string)
		if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 {
			return errors.New("parameter type mismatch for send")
		}
		var addrTo = types.HexToAddress(addrStr)
		tx, err := types.CreateUnbroadcastTransaction(gigea.GetAndIncrementNonce(), addrTo, count, gas, msg)
		if err != nil {
			return err
		}
		if err := v.SignRawTransactionWithKey(tx, spk); err != nil {
			return err
		}
		pool.Get().QueueTransaction(tx)
		return tx.Hash()
	case "get":
		// params[0]: hash string
		return nil
	}
	return nil
}
