package network

import (
	"context"
	"encoding/json"

	"github.com/cerera/internal/service"
)

func Execute(ctx context.Context, method string, params json.RawMessage) (any, error) {
	registry, ok := service.GetRegistryFromContext(ctx)
	if !ok {
		return nil, &rpcError{code: -32603, msg: "Internal error"}
	}

	cmp, m := service.ParseMethod(method)
	cmp = service.ResolveServiceName(cmp)

	handler, ok := registry.Lookup(cmp, m)
	if !ok {
		return nil, &rpcError{code: -32601, msg: "Method not found"}
	}
	var args []any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, &rpcError{code: -32602, msg: "Invalid params"}
		}
	}
	return handler(ctx, args)
}
