package validator

import (
	"context"
	"crypto/ecdsa"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"

	"github.com/cerera/config"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/gigea"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validation"
	"github.com/cerera/pallada"

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

type Validator interface {
	GetVersion() string
	ExecuteTransaction(tx types.GTransaction) error
	FindTransaction(hash common.Hash) *types.GTransaction
	CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas uint64, message string) (*types.GTransaction, error)
	SetUp(chainId *big.Int)
	ServiceName() string
	Signer() types.Signer
	SignRawTransactionWithKey(tx *types.GTransaction, kStr string) error
	Status() byte
	ValidateRawTransaction(tx types.GTransaction) bool
	ValidateBlockContent(b *block.Block) error
	ValidateBlockPoW(b *block.Block) bool
	ComputeBlockStateRoot(b *block.Block) (common.Hash, error)
	GetBlockValidator() *validation.BlockValidator
	GetID() string
	Update(tx *types.GTransaction)
	UpdateTxTree(tx *types.GTransaction, bIndex int)
	Methods() map[string]service.RPCHandler

	ReplayChain(blocks []*block.Block) error
	SetPool(pool pool.TxPool)
	SetChain(chain *chain.Chain)
}

type CoreValidator struct {
	*chain.Chain
	signatureKey   *ecdsa.PrivateKey
	signer         types.Signer
	balance        *big.Int
	currentAddress types.Address
	currentVersion string
	chainID        int

	pool    pool.TxPool
	vault   *storage.D5Vault
	txTable *storage.TxTable

	blockVal *validation.BlockValidator
}

func (v *CoreValidator) SetPool(pool pool.TxPool) {
	v.pool = pool
}

func (v *CoreValidator) SetChain(bc *chain.Chain) {
	v.Chain = bc
}

func NewValidator(ctx context.Context, cfg config.Config, vault *storage.D5Vault, txTable *storage.TxTable) (Validator, error) {
	var p, err = crypto.DecodePrivKey(cfg.NetCfg.PRIV)
	if err != nil {
		return nil, err
	}
	if txTable == nil {
		txTable = storage.NewTxTable()
	}
	v := &CoreValidator{
		signatureKey:   p,
		signer:         types.NewSimpleSigner(big.NewInt(int64(cfg.Chain.ChainID))),
		balance:        big.NewInt(0),
		currentVersion: "ALPHA-0.0.1",
		currentAddress: cfg.NetCfg.ADDR,
		chainID:        cfg.Chain.ChainID,
		vault:          vault,
		txTable:        txTable,
	}
	v.SetUp(big.NewInt(int64(cfg.Chain.ChainID)))
	v.initBlockValidator()
	return v, nil
}

func (v *CoreValidator) initBlockValidator() {
	v.blockVal = &validation.BlockValidator{
		ChainID:          v.chainID,
		Signer:           v.signer,
		Vault:            v.vault,
		TxTable:          v.txTable,
		StateRootAfter:   v.computeStateRootAfterBlock,
		SkipPoWAtGenesis: true,
	}
	chain.BlockContentValidator = v.ValidateBlockContent
}

func (v *CoreValidator) CheckAddress(addr types.Address) bool {
	// move logic to consensus
	return v.currentAddress != addr
}

func (v *CoreValidator) CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas uint64, message string) (*types.GTransaction, error) {
	// here we create transaction by input values
	tx, err := types.CreateUnbroadcastTransaction(nonce, addressTo, count, gas, v.GasPrice(), message)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (v *CoreValidator) FindTransaction(hash common.Hash) *types.GTransaction {
	if v.txTable != nil {
		v.txTable.Get(hash)
	}
	return nil
}

func (v *CoreValidator) ExecuteTransaction(tx types.GTransaction) error {
	err := v.applyTransaction(tx, true)
	if err == nil && v.vault != nil {
		if flushErr := v.vault.Flush(); flushErr != nil {
			return flushErr
		}
	}
	return err
}

func (v *CoreValidator) applyTransaction(tx types.GTransaction, recordInChain bool) error {
	if recordInChain && v.txTable != nil && v.txTable.Get(tx.Hash()) != -1 {
		return nil
	}
	localVault := v.vault
	if localVault == nil {
		return errors.New("vault not initialized")
	}
	var val = tx.Value()

	switch tx.Type() {
	case types.FaucetTxType:
		if tx.To() == nil {
			valExecuteError.Inc()
			return errors.New("faucet transaction missing recipient address")
		}
		if err := localVault.DropFaucet(*tx.To(), val, tx.Hash()); err != nil {
			valExecuteError.Inc()
			return err
		}

	case types.CoinbaseTxType:
		if tx.To() == nil {
			valExecuteError.Inc()
			return errors.New("coinbase transaction missing recipient address")
		}
		if err := localVault.RewardMiner(*tx.To(), val, tx.Hash()); err != nil {
			valExecuteError.Inc()
			return err
		}

	case types.LegacyTxType:
		if tx.To() == nil {
			valExecuteError.Inc()
			return errors.New("legacy transaction missing recipient address")
		}
		sender, err := validation.RecoverSender(v.signer, &tx)
		if err == nil {
			tx.SetFrom(sender)
		}
		senderAcc := localVault.Get(tx.From())
		if senderAcc == nil {
			valExecuteError.Inc()
			return NotEnoughtInputs
		}
		gasCost := tx.Cost()
		totalDebit := new(big.Int).Add(new(big.Int).Set(val), gasCost)
		senderBal := senderAcc.GetBalanceBI()
		if senderBal.Cmp(totalDebit) < 0 {
			valExecuteError.Inc()
			return NotEnoughtInputs
		}

		gasLimit := uint64(tx.Gas())
		if gasLimit > 0 && gasLimit < pallada.MinTransferGas {
			valExecuteError.Inc()
			return fmt.Errorf("gas limit below minimum: got %d, need %d", gasLimit, pallada.MinTransferGas)
		}

		senderAcc.SetBalanceBI(new(big.Int).Sub(senderBal, gasCost))
		if err := localVault.UpdateBalance(tx.From(), *tx.To(), val, tx.Hash()); err != nil {
			valExecuteError.Inc()
			return err
		}
		senderAcc.Nonce++

	case types.AppTxType:
		return nil

	default:
		vlogger.Warnw("unknown transaction type",
			"type", tx.Type(),
			"from", tx.From(),
		)
		valExecuteError.Inc()
		return fmt.Errorf("unknown transaction type: %d", tx.Type())
	}

	if recordInChain && v.txTable != nil {
		v.txTable.Add(&tx)
	}
	valExecuteSuccess.Inc()
	if recordInChain && v.pool != nil {
		v.pool.RemoveFromPool(tx.Hash())
	}
	return nil
}

func (v *CoreValidator) GasPrice() *big.Int {
	// Default minimal gas price is defined by the Pallada VM
	return common.FloatToBigInt(pallada.MinGasPrice())
}

func (v *CoreValidator) GetID() string {
	return v.currentAddress.String()
}

func (v *CoreValidator) GetVersion() string {
	return v.currentVersion
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
	// v.minGasPrice = common.FloatToBigInt(0.000001)
	v.signer = types.NewSimpleSigner(chainId)
	if v.Chain == nil {
		return
	}
	// config chain
	if v.Chain.GetChainConfigStatus() == 0x0 {
		for _, block := range v.Chain.GetData() {
			for _, tx := range block.Transactions {
				// Skip if transaction was already executed
				if v.txTable != nil && v.txTable.Get(tx.Hash()) != -1 {
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

// signkey is string representation of ecdsa private key
func (v *CoreValidator) SignRawTransactionWithKey(tx *types.GTransaction, signKey string) error {
	// get for tx
	v.balance.Add(v.balance, big.NewInt(int64(tx.Gas())))

	// sign tx
	if signKey == "" {
		valSignError.Inc()
		return errors.New("empty signing key id")
	}

	pemBlock, _ := pem.Decode([]byte(signKey))
	if pemBlock == nil || len(pemBlock.Bytes) == 0 {
		valSignError.Inc()
		return errors.New("invalid PEM block for private key")
	}

	var aKey *ecdsa.PrivateKey
	aKey, err := crypto.DecodePrivKey(signKey)
	if err != nil {
		vlogger.Errorw("error while decode key",
			"hash", tx.Hash(),
			"err", err,
		)
		valSignError.Inc()
		return errors.New("error while sign tx")
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
	*tx = *signTx
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
	if v.txTable != nil {
		v.txTable.UpdateIndex(tx, bIndex)
	}
}

func (v *CoreValidator) ValidateRawTransaction(tx types.GTransaction) bool {
	if err := validation.ValidateMempoolTx(v.signer, tx, v.vault, v.txTable); err != nil {
		valTxRejected.Inc()
		vlogger.Debugw("mempool tx rejected", "hash", tx.Hash(), "err", err)
		return false
	}
	valTxValidated.Inc()
	return true
}

func (v *CoreValidator) ValidateBlockContent(b *block.Block) error {
	if v.blockVal == nil {
		return nil
	}
	return v.blockVal.ValidateContent(b)
}

func (v *CoreValidator) ValidateBlockPoW(b *block.Block) bool {
	if v.blockVal == nil {
		return b != nil && b.Head != nil
	}
	return v.blockVal.ValidatePoW(b)
}

func (v *CoreValidator) GetBlockValidator() *validation.BlockValidator {
	return v.blockVal
}

func (v *CoreValidator) ComputeBlockStateRoot(b *block.Block) (common.Hash, error) {
	return v.computeStateRootAfterBlock(b)
}

func (v *CoreValidator) computeStateRootAfterBlock(b *block.Block) (common.Hash, error) {
	vault := v.vault
	if vault == nil || b == nil || b.Head == nil {
		return common.EmptyRootHash, fmt.Errorf("vault or block missing")
	}
	snap := vault.SnapshotAccounts()
	defer vault.RestoreAccounts(snap)

	vault.RestoreBaseline()
	if v.Chain != nil {
		for _, blk := range v.Chain.GetData() {
			if blk == nil || blk.Head == nil || blk.Head.Height == 0 {
				continue
			}
			if blk.Head.Height >= b.Head.Height {
				break
			}
			for i := range blk.Transactions {
				tx := blk.Transactions[i]
				if tx.Type() == types.FaucetTxType {
					continue
				}
				if err := v.applyTransaction(tx, false); err != nil {
					return common.Hash{}, fmt.Errorf("replay block %d: %w", blk.Head.Height, err)
				}
			}
		}
	}

	for i := range b.Transactions {
		tx := b.Transactions[i]
		if tx.Type() == types.FaucetTxType {
			continue
		}
		if err := v.applyTransaction(tx, false); err != nil {
			return common.Hash{}, err
		}
	}
	return vault.ComputeStateRoot(), nil
}

// ReplayChain resets vault/tx state to baseline and re-applies canonical blocks.
func (v *CoreValidator) ReplayChain(blocks []*block.Block) error {
	vault := v.vault
	if vault == nil {
		return errors.New("vault not initialized")
	}
	vault.RestoreBaseline()
	if v.txTable != nil {
		v.txTable.Reset()
	}
	for _, b := range blocks {
		if b == nil || b.Head == nil || b.Head.Height == 0 {
			continue
		}
		idx := int(b.Head.Index)
		for i := range b.Transactions {
			tx := b.Transactions[i]
			if tx.Type() == types.FaucetTxType {
				continue
			}
			if err := v.applyTransaction(tx, true); err != nil {
				return fmt.Errorf("replay block %d tx %s: %w", b.Head.Height, tx.Hash().Hex(), err)
			}
			v.UpdateTxTree(&b.Transactions[i], idx)
		}
	}
	return vault.Flush()
}

func (v *CoreValidator) Methods() map[string]service.RPCHandler {
	return map[string]service.RPCHandler{
		"_create": func(ctx context.Context, params []any) (any, error) {
			if len(params) == 1 {
				if p, ok := params[0].(CreateTxParams); ok {
					if p.Gas < 0 {
						return nil, errors.New("negative gas or value")
					}
					dust, err := types.DecimalStringToDust(p.Amount)
					if err != nil {
						return nil, err
					}
					tx, err := types.CreateUnbroadcastTransactionDust(p.Nonce, p.To, dust, uint64(p.Gas), v.GasPrice(), p.Msg)
					if err != nil {
						return nil, err
					}
					if err := v.SignRawTransactionWithKey(tx, p.Key); err != nil {
						return nil, err
					}
					return tx, nil
				}
			}
			if len(params) < 6 {
				return nil, errors.New("invalid parameters for create")
			}
			key, ok0 := params[0].(string)
			nonce, ok1 := params[1].(uint64)
			to, ok2 := params[2].(types.Address)
			count, ok3 := params[3].(float64)
			gas, ok4 := params[4].(float64)
			msg, ok5 := params[5].(string)
			if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
				return nil, errors.New("parameter type mismatch for create")
			}
			tx, err := types.CreateUnbroadcastTransaction(nonce, to, count, uint64(gas), v.GasPrice(), msg)
			if err != nil {
				return nil, err
			}
			if err := v.SignRawTransactionWithKey(tx, key); err != nil {
				return nil, err
			}
			return tx, nil
		},
		"send": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 5 {
				return nil, errors.New("invalid parameters for send")
			}
			spk, ok0 := params[0].(string)
			addrStr, ok1 := params[1].(string)
			count, ok2 := params[2].(float64)
			gas, ok3 := params[3].(float64)
			msg, ok4 := params[4].(string)
			if !ok0 || !ok1 || !ok2 || !ok3 || !ok4 {
				return nil, errors.New("parameter type mismatch for send")
			}
			var addrTo = types.HexToAddress(addrStr)
			aKey, err := crypto.DecodePrivKey(spk)
			if err != nil {
				return nil, errors.New("invalid signing key")
			}
			from := crypto.PrivKeyToAddress(*aKey)
			nonce := uint64(1)
			if acc := v.vault.Get(from); acc != nil {
				nonce = acc.Nonce
			}
			tx, err := types.CreateUnbroadcastTransaction(nonce, addrTo, count, uint64(gas), v.GasPrice(), msg)
			if err != nil {
				return nil, err
			}
			if err := v.SignRawTransactionWithKey(tx, spk); err != nil {
				return nil, err
			}
			err = v.pool.AddRawTransaction(tx)
			if err != nil {
				return nil, err
			}
			return tx.Hash(), nil
		},
		"get": func(ctx context.Context, params []any) (any, error) {
			if len(params) == 1 {
				if p, ok := params[0].(string); ok {
					hash := common.HexToHash(p)
					var index = -1
					if v.txTable != nil {
						index = v.txTable.Get(hash)
					}
					if index != -1 {
						txBlock := v.GetBlockByNumber(index)
						for _, btx := range txBlock.Transactions {
							if btx.Hash() == hash {
								result := map[string]interface{}{
									"hash":  btx.Hash().Hex(),
									"from":  btx.From().Hex(),
									"value": btx.Value().String(),
									"gas":   uint64(btx.Gas()),
									"data":  common.Bytes(btx.Data()).String(),
								}
								if to := btx.To(); to != nil {
									result["to"] = to.Hex()
								} else {
									result["to"] = nil
								}
								return result, nil
							}
						}
					}
				}
			}
			return nil, nil
		},
		"stake": func(ctx context.Context, params []any) (any, error) {
			return nil, nil
		},
	}
}
