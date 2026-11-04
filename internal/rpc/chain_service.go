package rpc

import (
	"context"
	"fmt"

	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/internal/service"
)

// ChainMethods returns JSON-RPC handlers for the chain service.
func ChainMethods(bc *chain.Chain) map[string]service.RPCHandler {
	return map[string]service.RPCHandler{
		"getLatestBlock": func(ctx context.Context, params []any) (any, error) {
			return bc.GetLatestBlock(), nil
		},
		"getBlockByIndex": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("height required")
			}
			height, ok := params[0].(float64)
			if !ok {
				return nil, fmt.Errorf("height must be int")
			}
			return bc.GetBlockByNumber(int(height)), nil
		},
		"getBlockByHash": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("hash required")
			}
			hashStr, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("hash must be string")
			}
			return bc.GetBlock(common.HexToHash(hashStr)), nil
		},
		"getCurrentHeight": func(ctx context.Context, params []any) (any, error) {
			latest := bc.GetLatestBlock()
			if latest == nil || latest.Head == nil {
				return 0, nil
			}
			return latest.Header().Height, nil
		},
		"getChainId": func(ctx context.Context, params []any) (any, error) {
			return bc.GetChainId(), nil
		},
		"getInfo": func(ctx context.Context, params []any) (any, error) {
			return bc.GetInfo(), nil
		},
		"height": func(ctx context.Context, params []any) (any, error) {
			latest := bc.GetLatestBlock()
			if latest == nil || latest.Head == nil {
				return 0, nil
			}
			return latest.Header().Height, nil
		},
		"updateChain": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("block required")
			}
			b, ok := params[0].(*block.Block)
			if !ok || b == nil {
				return nil, fmt.Errorf("block must be *block.Block")
			}
			return nil, bc.UpdateChain(b)
		},
	}
}

// RegisterChain registers chain RPC handlers on the service registry.
func RegisterChain(reg *service.Registry, bc *chain.Chain) error {
	return reg.Register(service.NewService(service.CHAIN_SERVICE_NAME, ChainMethods(bc)))
}
