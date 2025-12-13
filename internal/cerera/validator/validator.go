package validator

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/gigea"

	"github.com/cerera/internal/cerera/types"

	"github.com/prometheus/client_golang/prometheus"
)

const VALIDATOR_SERVICE_NAME = "CERERA_VALIDATOR_54013.10.25"

var (
	EmptyCoinbase    = &decError{"empty hex string"}
	NotEnoughtInputs = &decError{"not enought inputs"}
)

var vlogger = logger.Named("validator")

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
	// CheckAddress(addr types.Address) bool
	GasPrice() *big.Int
	GetVersion() string
	Exec(method string, params []interface{}) interface{}
	ExecuteTransaction(tx types.GTransaction) error
	FindTransaction(hash common.Hash) *types.GTransaction
	CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas float64, message string) (*types.GTransaction, error)
	SetUp(chainId *big.Int)
	ServiceName() string
	Signer() types.Signer
	SignRawTransactionWithKey(tx *types.GTransaction, kStr string) error
	Status() byte
	ValidateRawTransaction(tx *types.GTransaction) bool
	ValidateTransaction(t *types.GTransaction, from types.Address) bool
	ValidateBlock(b block.Block) bool
	// observer methods
	GetID() string
	Update(tx *types.GTransaction)
	ProposeBlock(b *block.Block)
}

type CoreValidator struct {
	*chain.Chain
	signatureKey   *ecdsa.PrivateKey
	signer         types.Signer
	balance        *big.Int
	currentAddress types.Address
	currentVersion string
	minGasPrice    *big.Int
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
	storage.InitTxTable()
	v = &CoreValidator{
		signatureKey:   p,
		signer:         types.NewSimpleSigner(big.NewInt(int64(cfg.Chain.ChainID))),
		balance:        big.NewInt(0), // Initialize balance
		currentVersion: "ALPHA-0.0.1",
		currentAddress: cfg.NetCfg.ADDR,
	}
	// Get chain from registry if not set
	registry, err := service.GetRegistry()
	if err == nil {
		if ch, ok := registry.GetService(chain.CHAIN_SERVICE_NAME); ok {
			if chainPtr, ok := ch.(*chain.Chain); ok {
				v.(*CoreValidator).Chain = chainPtr
			}
		}
	}
	// preconfig chain
	ConfigChain(v)
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

func (v *CoreValidator) FindTransaction(hash common.Hash) *types.GTransaction {
	storage.GetTxTable().Get(hash)
	return nil
}

func (v *CoreValidator) ExecuteTransaction(tx types.GTransaction) error {
	// if send to address not generated - > send only to input
	// executed transaction adds to txs trie struct
	var localVault = storage.GetVault()
	var val = tx.Value()

	// Handle different transaction types first to avoid checking sender for faucet/coinbase
	switch tx.Type() {
	case types.FaucetTxType:
		// Faucet transactions: no sender balance check needed
		if tx.To() == nil {
			return errors.New("faucet transaction missing recipient address")
		}
		if err := localVault.DropFaucet(*tx.To(), val, tx.Hash()); err != nil {
			return err
		}
		// add tx to tx tree
		storage.GetTxTable().Add(&tx)
		valExecuteSuccess.Inc()
		return nil

	case types.CoinbaseTxType:
		// Coinbase transactions: reward goes directly to miner
		// Create shadow account for miner if it doesn't exist
		if tx.To() == nil {
			return errors.New("coinbase transaction missing recipient address")
		}
		if err := localVault.RewardMiner(*tx.To(), val, tx.Hash()); err != nil {
			return err
		}

		// add tx to tx tree
		storage.GetTxTable().Add(&tx)
		valExecuteSuccess.Inc()
		return nil

	case types.LegacyTxType:
		// Regular transactions: check sender balance and deduct gas
		if tx.To() == nil {
			return errors.New("legacy transaction missing recipient address")
		}
		// check if sender has enough balance
		senderAcc := localVault.Get(tx.From())
		if senderAcc == nil {
			return NotEnoughtInputs
		}
		gasCost := tx.Cost()
		totalDebit := new(big.Int).Add(new(big.Int).Set(val), gasCost)
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
		vlogger.Warnw("unknown transaction type",
			"type", tx.Type(),
			"from", tx.From(),
		)
		return fmt.Errorf("unknown transaction type: %d", tx.Type())
	}

	// add tx to tx tree
	storage.GetTxTable().Add(&tx)

	valExecuteSuccess.Inc()
	return nil
}

func (v *CoreValidator) GasPrice() *big.Int {
	return v.minGasPrice
}

func (v *CoreValidator) GetID() string {
	return v.currentAddress.String()
}

func (v *CoreValidator) GetVersion() string {
	return v.currentVersion
}

func (v *CoreValidator) ProposeBlock(b *block.Block) {
	// Проверяем готовность Ice (bootstrap соединение)
	if !v.isIceReady() {
		vlogger.Warnw("Ice not ready - bootstrap connection not established, but adding block locally anyway", "block_hash", b.GetHash())
	} else {
		vlogger.Debugw("Ice is ready - bootstrap connection established", "block_hash", b.GetHash())
	}

	// Проверяем, начался ли консенсус
	if !v.isConsensusStarted() {
		v.printConsensusStatus(b.GetHash())
		vlogger.Warnw("Consensus not started - adding block locally anyway", "block_hash", b.GetHash())
	} else {
		vlogger.Debugw("Consensus started - proceeding with block proposal", "block_hash", b.GetHash())
	}

	for _, btx := range b.Transactions {
		v.ExecuteTransaction(btx)
		v.UpdateTxTree(&btx, int(b.Header().Index))
	}
	v.UpdateChain(b)
}

// isIceReady проверяет, готов ли Ice компонент (bootstrap соединение установлено)
func (v *CoreValidator) isIceReady() bool {
	registry, err := service.GetRegistry()
	if err != nil {
		vlogger.Debugw("Registry not available for Ice check", "err", err)
		return false
	}

	// Пробуем найти Ice сервис по имени "ice" или по полному имени
	iceService, ok := registry.GetService("ice")
	if !ok {
		// Пробуем найти по полному имени сервиса
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			vlogger.Debugw("Ice service not found in registry")
			return false
		}
	}

	// Вызываем метод проверки готовности через Exec
	result := iceService.Exec("isBootstrapReady", nil)
	if ready, ok := result.(bool); ok {
		return ready
	}

	return false
}

// waitForIceReady блокирует выполнение до готовности Ice
func (v *CoreValidator) waitForIceReady() {
	registry, err := service.GetRegistry()
	if err != nil {
		vlogger.Errorw("Registry not available for Ice wait", "err", err)
		return
	}

	// Пробуем найти Ice сервис по имени "ice" или по полному имени
	iceService, ok := registry.GetService("ice")
	if !ok {
		// Пробуем найти по полному имени сервиса
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			vlogger.Errorw("Ice service not found in registry")
			return
		}
	}

	// Вызываем метод ожидания готовности через Exec (блокирующий вызов)
	iceService.Exec("waitForBootstrapReady", nil)
}

// isConsensusStarted проверяет, начался ли консенсус
func (v *CoreValidator) isConsensusStarted() bool {
	registry, err := service.GetRegistry()
	if err != nil {
		vlogger.Debugw("Registry not available for consensus check", "err", err)
		return false
	}

	// Пробуем найти Ice сервис по имени "ice" или по полному имени
	iceService, ok := registry.GetService("ice")
	if !ok {
		// Пробуем найти по полному имени сервиса
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			vlogger.Debugw("Ice service not found in registry for consensus check")
			return false
		}
	}

	// Вызываем метод проверки консенсуса через Exec
	result := iceService.Exec("isConsensusStarted", nil)
	if started, ok := result.(bool); ok {
		return started
	}

	return false
}

// waitForConsensus блокирует выполнение до начала консенсуса
func (v *CoreValidator) waitForConsensus() {
	registry, err := service.GetRegistry()
	if err != nil {
		vlogger.Errorw("Registry not available for consensus wait", "err", err)
		return
	}

	// Пробуем найти Ice сервис по имени "ice" или по полному имени
	iceService, ok := registry.GetService("ice")
	if !ok {
		// Пробуем найти по полному имени сервиса
		iceService, ok = registry.GetService("ICE_CERERA_001_1_0")
		if !ok {
			vlogger.Errorw("Ice service not found in registry for consensus wait")
			return
		}
	}

	// Вызываем метод ожидания консенсуса через Exec (блокирующий вызов)
	iceService.Exec("waitForConsensus", nil)
}

// printConsensusStatus выводит текущий статус консенсуса
func (v *CoreValidator) printConsensusStatus(blockHash common.Hash) {
	consensusInfo := gigea.GetConsensusInfo()
	vlogger.Warnw("Consensus not started - current consensus status",
		"block_hash", blockHash.Hex(),
		"status", consensusInfo["status"],
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"nonce", consensusInfo["nonce"],
		"address", consensusInfo["address"],
	)
}

func (v *CoreValidator) ServiceName() string {
	return VALIDATOR_SERVICE_NAME
}

func (v *CoreValidator) SetUp(chainId *big.Int) {
	// default min gas price; can be overridden from config in NewValidator
	v.minGasPrice = types.FloatToBigInt(0.000001)
	v.signer = types.NewSimpleSigner(chainId)
	if v.Chain == nil {
		return
	}
	// config chain
	if v.Chain.GetChainConfigStatus() == 0x0 {
		for _, block := range v.Chain.GetData() {
			for _, tx := range block.Transactions {
				// Skip if transaction was already executed
				if storage.GetTxTable().Get(tx.Hash()) != -1 {
					continue
				}
				err := v.ExecuteTransaction(tx)
				if err != nil {
					vlogger.Errorw("error while executing tx",
						"hash", tx.Hash(),
						"err", err,
					)
					valExecuteError.Inc()
					continue
				}
				v.UpdateTxTree(&tx, int(block.Header().Index))
			}
		}
	}
	v.Chain.SetChainConfigStatus(0x1)
}

func (v *CoreValidator) Signer() types.Signer {
	return v.signer
}

func (v *CoreValidator) SignRawTransactionWithKey(tx *types.GTransaction, signKey string) error {
	// get for tx
	v.balance.Add(v.balance, big.NewInt(int64(tx.Gas())))

	// sign tx
	if signKey == "" {
		valSignError.Inc()
		return errors.New("empty signing key id")
	}
	var vlt = storage.GetVault()
	var signBytes = vlt.GetKey(signKey)
	// fmt.Printf("signBytes: %x\n", signBytes)
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
		vlogger.Errorw("error while sign tx",
			"hash", tx.Hash(),
			"err", err2,
		)
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

func (v *CoreValidator) UpdateTxTree(tx *types.GTransaction, bIndex int) {
	storage.GetTxTable().UpdateIndex(tx, bIndex)
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
	localVault.CheckRunnable(r, s, tx)
	valTxValidated.Inc()
	return true
}

func (v *CoreValidator) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "_create":
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
				nonce := gigea.GetAndIncrementNonce()
				tx, err := types.CreateUnbroadcastTransactionWei(nonce, addrTo, wei, p.Gas, p.Msg)
				if err != nil {
					return err
				}
				if err := v.SignRawTransactionWithKey(tx, p.Key); err != nil {
					return err
				}
				time.Sleep(1 * time.Second) // prefer spam
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
		// if len(msg) > 1024 {
		// 	return errors.New("message too long")
		// }
		if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 {
			return errors.New("parameter type mismatch for send")
		}
		var addrTo = types.HexToAddress(addrStr)
		tx, err := types.CreateUnbroadcastTransaction(gigea.GetAndIncrementNonce(), addrTo, count, gas, msg)
		if err != nil {
			return err.Error()
		}
		if err := v.SignRawTransactionWithKey(tx, spk); err != nil {
			return err.Error()
		}
		pool.Get().QueueTransaction(tx)
		return tx.Hash()
	case "get":
		// params[0]: hash string
		if len(params) == 1 {
			if p, ok := params[0].(string); ok {
				hash := common.HexToHash(p)
				var index = storage.GetTxTable().Get(hash)
				if index != -1 {
					txBlock := v.GetBlockByNumber(index)
					for _, btx := range txBlock.Transactions {
						if btx.Hash() == hash {
							// Return only selected fields
							result := map[string]interface{}{
								"hash":  btx.Hash().Hex(),
								"from":  btx.From().Hex(),
								"value": btx.Value().String(),
								"gas":   btx.Gas(),
								"data":  string(btx.Data()),
							}
							// Handle To() which can be nil
							if to := btx.To(); to != nil {
								result["to"] = to.Hex()
							} else {
								result["to"] = nil
							}
							return result
						}
					}
				}
			}
		}
		return nil
	}
	return nil
}

func ConfigChain(validator Validator) {
}
