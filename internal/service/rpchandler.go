package service

import "context"

type RPCHandler func(ctx context.Context, params []any) (any, error)
