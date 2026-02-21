package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cerera/internal/logger"
	"go.uber.org/zap"
)

func registryLogger() *zap.SugaredLogger {
	return logger.Named("registry")
}

var R *Registry

type Service interface {
	Exec(method string, params []any) any
	ServiceName() string
}

type StoppableService interface {
	Service
	Stop() error
}

type Registry struct {
	services map[string]Service
	status   [32]byte
	mu       sync.Mutex
}

func Exec(method string, params []any) any {
	registry, err := GetRegistry()
	if err != nil {
		return err
	}
	var cmp, m = ParseMethod(method)
	service, ok := registry.GetService(cmp)
	if !ok {
		return fmt.Errorf("service %s not found", cmp)
	}
	return service.Exec(m, params)
}

func ExecTyped(method string, params []any) any {
	registry, err := GetRegistry()
	if err != nil {
		return err
	}
	var cmp, m = ParseMethod(method)
	service, ok := registry.GetService(cmp)
	if !ok {
		return fmt.Errorf("service %s not found", cmp)
	}
	return service.Exec(m, params)
}

func GetRegistry() (*Registry, error) {
	if R == nil {
		return nil, fmt.Errorf("service registry not initialized")
	}
	return R, nil
}

func NewRegistry() (*Registry, error) {
	registryLogger().Info("[REGISTRY] Creating new registry")
	R = &Registry{
		services: make(map[string]Service),
		status:   [32]byte{},
	}
	return R, nil
}

func (r *Registry) GetService(name string) (Service, bool) {
	if r == nil {
		return nil, false
	}
	var srvName = name
	if name == "account" {
		srvName = VAULT_SERVICE_NAME
	}
	if name == "chain" {
		srvName = CHAIN_SERVICE_NAME
	}
	if name == "pool" {
		srvName = POOL_SERVICE_NAME
	}
	if name == "transaction" || name == "validator" {
		srvName = VALIDATOR_SERVICE_NAME
	}
	if name == "ice" {
		srvName = ICE_SERVICE_NAME
	}
	if name == "miner" {
		srvName = MINER_SERVICE_NAME
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.services[srvName]
	return s, ok
}

func (r *Registry) Register(name string, s Service) {
	registryLogger().Infow("[REGISTRY] Registering service", "name", name)
	if r == nil {
		registryLogger().Error("[REGISTRY] Registry is nil")
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	registryLogger().Infow("[REGISTRY] Service registered", "name", name)
	r.services[name] = s
	r.UpdateStatus()
}

func (r *Registry) UpdateStatus() {
	for i, s := range r.status {
		if s == 0x0 {
			r.status[i] = 0x1
			break
		}
	}
}

// StopAllServices останавливает все сервисы, которые поддерживают остановку
func (r *Registry) StopAllServices() error {
	r.mu.Lock()
	services := make([]Service, 0, len(r.services))
	for _, s := range r.services {
		services = append(services, s)
	}
	r.mu.Unlock()

	var lastErr error
	for _, service := range services {
		if stoppable, ok := service.(StoppableService); ok {
			if err := stoppable.Stop(); err != nil {
				lastErr = err
				// Логируем ошибку, но продолжаем останавливать другие сервисы
			}
		}
	}
	return lastErr
}

func ParseMethod(method string) (string, string) {
	// EX: cerera.account.getAll or miner.status
	parts := strings.Split(method, ".")
	if parts[0] == "cerera" && len(parts) == 3 {
		// EX: cerera.account.getAll -> return account, getAll
		return parts[1], parts[2]
	}
	if len(parts) == 2 {
		// EX: miner.status -> return miner, status
		return parts[0], parts[1]
	}
	return method, method
}
