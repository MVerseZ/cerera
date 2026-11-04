package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cerera/core/common"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/service"
)

// VaultMethods returns JSON-RPC handlers for the vault service.
func VaultMethods(v *storage.D5Vault) map[string]service.RPCHandler {
	return map[string]service.RPCHandler{
		"getAll": func(ctx context.Context, params []any) (any, error) {
			return v.GetAll(), nil
		},
		"getCount": func(ctx context.Context, params []any) (any, error) {
			return v.GetCount(), nil
		},
		"create": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("passphrase required")
			}
			passphrase, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("passphrase must be string")
			}
			mk, pk, m, addr, err := v.Create(passphrase)
			if err != nil {
				return nil, err
			}
			type res struct {
				Address  *types.Address `json:"address,omitempty"`
				Priv     string         `json:"priv,omitempty"`
				Pub      string         `json:"pub,omitempty"`
				Mnemonic string         `json:"mnemonic,omitempty"`
			}
			return &res{
				Address:  addr,
				Priv:     mk,
				Pub:      pk,
				Mnemonic: m,
			}, nil
		},
		"restore": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("mnemonic and passphrase required")
			}
			mnemonic, ok0 := params[0].(string)
			pass, ok1 := params[1].(string)
			if !ok0 || !ok1 {
				return nil, fmt.Errorf("mnemonic and passphrase must be strings")
			}
			addr, pk, err := v.Restore(mnemonic, pass)
			if err != nil {
				return nil, err
			}
			type res struct {
				Addr types.Address `json:"address,omitempty"`
				Priv string        `json:"priv,omitempty"`
				Pub  string        `json:"pub,omitempty"`
			}
			return &res{
				Priv: pk,
				Addr: addr,
			}, nil
		},
		"verify": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("address and passphrase required")
			}
			addrHex, ok0 := params[0].(string)
			pass, ok1 := params[1].(string)
			if !ok0 || !ok1 {
				return nil, fmt.Errorf("address and passphrase must be strings")
			}
			rAddr, err := v.VerifyAccount(types.HexToAddress(addrHex), pass)
			if err != nil {
				return false, nil
			}
			return rAddr.Hex() == addrHex, nil
		},
		"getBalance": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("address required")
			}
			addrHex, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("address must be string")
			}
			acc := v.Get(types.HexToAddress(addrHex))
			if acc == nil {
				return big.NewInt(0), nil
			}
			return acc.GetBalance(), nil
		},
		"faucet": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 2 {
				return nil, fmt.Errorf("address and amount required")
			}
			addrStr, ok0 := params[0].(string)
			if !ok0 {
				return nil, fmt.Errorf("address must be string")
			}
			addr := types.HexToAddress(addrStr)
			var amountBigInt *big.Int
			if s, ok := params[1].(string); ok {
				dust, err := types.DecimalStringToDust(s)
				if err != nil {
					return nil, err
				}
				amountBigInt = dust
			} else if f, ok := params[1].(float64); ok {
				amountBigInt = common.FloatToBigInt(f)
			} else {
				return nil, fmt.Errorf("amount must be string or number")
			}
			txHash := common.BytesToHash([]byte("faucet_" + addrStr))
			err := v.DropFaucet(addr, amountBigInt, txHash)
			if err != nil {
				return nil, err
			}
			return "Faucet successful", nil
		},
		"inputs": func(ctx context.Context, params []any) (any, error) {
			if len(params) < 1 {
				return nil, fmt.Errorf("address required")
			}
			addrHex, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("address must be string")
			}
			account := v.Get(types.HexToAddress(addrHex))
			if account == nil {
				return make(map[common.Hash]*big.Int), nil
			}
			return make(map[common.Hash]*big.Int), nil
		},
		"getTotalSupply": func(ctx context.Context, params []any) (any, error) {
			return v.GetTotalSupply(), nil
		},
	}
}

// RegisterVault registers vault RPC handlers on the service registry.
func RegisterVault(reg *service.Registry, v *storage.D5Vault) error {
	return reg.Register(service.NewService(service.VAULT_SERVICE_NAME, VaultMethods(v)))
}
