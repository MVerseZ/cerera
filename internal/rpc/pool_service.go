package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cerera/core/common"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/service"
)

// PoolMethods returns JSON-RPC handlers for the transaction pool service.
func PoolMethods(p *pool.Pool) map[string]service.RPCHandler {
	return map[string]service.RPCHandler{
		"getInfo": func(ctx context.Context, params []any) (any, error) {
			return p.GetInfo(), nil
		},
		"minGas": func(ctx context.Context, params []any) (any, error) {
			return p.GetMinimalGasValue(), nil
		},
		"getPendingTransactions": func(ctx context.Context, params []any) (any, error) {
			return p.GetPendingTransactions(), nil
		},
		"getTransaction": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("transaction hash required")
			}
			hashStr, ok := params[0].(string)
			if !ok {
				return nil, errors.New("transaction hash must be string")
			}
			return p.GetTransaction(common.HexToHash(hashStr)), nil
		},
		"sendRawTransaction": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("signed transaction JSON required")
			}
			raw, ok := params[0].(string)
			if !ok {
				return nil, errors.New("signed transaction must be a JSON string")
			}
			var tx types.GTransaction
			if err := json.Unmarshal([]byte(raw), &tx); err != nil {
				return nil, errors.New("invalid signed transaction JSON: " + err.Error())
			}
			if !tx.IsSigned() || tx.Type() != types.LegacyTxType {
				return nil, errors.New("expected a signed legacy transaction")
			}
			if !p.TxValidatorReady() {
				return nil, errors.New("transaction validator is not configured")
			}
			if err := p.AddRawTransaction(&tx); err != nil {
				return nil, err
			}
			return tx.Hash().Hex(), nil
		},
		"getTopN": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("n required")
			}
			n, ok := params[0].(int)
			if !ok {
				return nil, errors.New("n must be int")
			}
			return p.GetTopN(n), nil
		},
		"getMiningPackage": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("n required")
			}
			n, ok := params[0].(int)
			if !ok {
				return nil, errors.New("n must be int")
			}
			return p.GetMiningPackage(n), nil
		},
		"removeFromPool": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("transaction hash required")
			}
			hashStr, ok := params[0].(string)
			if !ok {
				return nil, errors.New("transaction hash must be string")
			}
			err := p.RemoveFromPool(common.HexToHash(hashStr))
			return err == nil, err
		},
	}
}

// RegisterPool registers pool RPC handlers on the service registry.
func RegisterPool(reg *service.Registry, p *pool.Pool) error {
	return reg.Register(service.NewService(service.POOL_SERVICE_NAME, PoolMethods(p)))
}
