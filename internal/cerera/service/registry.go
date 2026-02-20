package service

import (
	"fmt"
	"strings"
	"sync"
)

var R *Registry

type Service interface {
	Exec(method string, params []interface{}) interface{}
	ServiceName() string
}

type StoppableService interface {
	Service
	Stop() error
}

type Registry struct {
	services map[string]Service
	mu       sync.Mutex
}

func Exec(method string, params []interface{}) interface{} {
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

func ExecTyped(method string, params []interface{}) interface{} {
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
	R = &Registry{
		services: make(map[string]Service),
	}
	return R, nil
}

func (r *Registry) GetService(name string) (Service, bool) {
	if r == nil {
		return nil, false
	}
	var srvName = name
	if name == "account" {
		srvName = "D5_VAULT_CERERA_001_1_7"
	}
	if name == "chain" {
		srvName = "CHAIN_CERERA_001_1_7"
	}
	if name == "pool" {
		srvName = "POOL_CERERA_001_1_3"
	}
	if name == "transaction" || name == "validator" {
		srvName = "CERERA_VALIDATOR_54013.10.25"
	}
	if name == "ice" {
		srvName = "ICE_CERERA_001_1_0"
	}
	if name == "miner" {
		srvName = "CERERA_MINER_001_1_0"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.services[srvName]
	return s, ok
}

func (r *Registry) Register(name string, s Service) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[name] = s
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
