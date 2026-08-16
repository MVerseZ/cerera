package service

import (
	"context"
	"sync"

	"github.com/cerera/internal/logger"
	"go.uber.org/zap"
)

func registryLogger() *zap.SugaredLogger {
	return logger.Named("registry")
}

type Registry struct {
	services map[string]*Service
	mu       sync.RWMutex
}

func NewRegistry() (*Registry, error) {
	registryLogger().Info("[REGISTRY] Creating new registry")
	return &Registry{
		services: make(map[string]*Service),
	}, nil
}

type ctxKey string
const registryCtxKey ctxKey = "service_registry"

func WithRegistry(ctx context.Context, r *Registry) context.Context {
	return context.WithValue(ctx, registryCtxKey, r)
}

func GetRegistryFromContext(ctx context.Context) (*Registry, bool) {
	r, ok := ctx.Value(registryCtxKey).(*Registry)
	return r, ok
}

func (r *Registry) GetMethods(serviceName string) (map[string]RPCHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[serviceName]
	if !ok {
		return nil, false
	}
	return s.Methods, true
}

func (r *Registry) GetService(name string) (*Service, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[name]
	return s, ok
}

func (r *Registry) Register(s *Service) {
	if s == nil {
		return
	}
	registryLogger().Infow("[REGISTRY] Registering service", "name", s.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[s.Name] = s
}
