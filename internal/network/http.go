package network

import (
	"context"
	"fmt"

	"github.com/cerera/internal/service"
)

func Execute(ctx context.Context, method string, params []any) (any, error) {
	registry, ok := service.GetRegistryFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("registry not in context")
	}

	cmp, m := service.ParseMethod(method)
	cmp = service.ResolveServiceName(cmp)

	methods, ok := registry.GetMethods(cmp)
	if !ok {
		return nil, fmt.Errorf("service %s not found", cmp)
	}

	handler, ok := methods[m]
	if !ok {
		return nil, fmt.Errorf("method %s not found in %s", m, cmp)
	}

	return handler(ctx, params)
}
