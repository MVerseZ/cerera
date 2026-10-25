package service

import (
	"context"
	"fmt"
	"strings"
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

func (r *Registry) Lookup(serviceName, methodName string) (RPCHandler, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[serviceName]
	if !ok {
		return nil, false
	}
	handler, ok := s.Methods[methodName]
	return handler, ok
}

func (r *Registry) GetService(name string) (*Service, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[name]
	if !ok {
		return nil, false
	}
	methods := make(map[string]RPCHandler, len(s.Methods))
	for method, handler := range s.Methods {
		methods[method] = handler
	}
	return &Service{Name: s.Name, Methods: methods}, true
}

func (r *Registry) Register(s *Service) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}
	if s == nil {
		return fmt.Errorf("service is nil")
	}
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("service name is empty")
	}
	methods := make(map[string]RPCHandler, len(s.Methods))
	for name, handler := range s.Methods {
		methods[name] = handler
	}
	registered := &Service{Name: s.Name, Methods: methods}
	registryLogger().Infow("[REGISTRY] Registering service", "name", s.Name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.services[s.Name]; exists {
		return fmt.Errorf("service %q is already registered", s.Name)
	}
	r.services[s.Name] = registered
	return nil
}
