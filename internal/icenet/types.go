package icenet

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/discovery"
	"github.com/cerera/internal/icenet/mesh"
	"go.uber.org/zap"
)

const ICE_SERVICE_NAME = "ICE_CERERA_001_1_0"

// getSeedNodes returns seed nodes from config, environment variable, or empty list
func getSeedNodes(cfg *config.Config) []string {
	// Check environment variable first
	if envSeeds := os.Getenv("CERERA_SEED_NODES"); envSeeds != "" {
		// Parse comma-separated list
		seeds := strings.Split(envSeeds, ",")
		result := make([]string, 0, len(seeds))
		for _, seed := range seeds {
			seed = strings.TrimSpace(seed)
			if seed != "" {
				result = append(result, seed)
			}
		}
		return result
	}
	
	// Use config value if set
	if cfg != nil && len(cfg.NetCfg.SeedNodes) > 0 {
		return cfg.NetCfg.SeedNodes
	}
	
	// Fall back to empty list (no seed nodes)
	return []string{}
}

// icelogger returns a sugared logger for the icenet package.
// It is defined as a function (not a global variable) so that it always
// uses the logger configured by logger.Init(), even if this package is
// imported before logging is set up in main().
func icelogger() *zap.SugaredLogger {
	return logger.Named("icenet")
}

// Ice представляет основной компонент Ice
type Ice struct {
	cfg                 *config.Config
	ctx                 context.Context
	status              byte
	started             bool
	address             types.Address              // адрес cerera
	ip                  string                     // IP адрес узла
	port                string                     // порт узла
	listener            net.Listener               // listener для входящих подключений
	mu                  sync.RWMutex               // мьютекс для потокобезопасности (RWMutex для оптимизации чтения)
	lastSentBlockHash   common.Hash                // хеш последнего отправленного блока
	networkReady        bool                       // флаг готовности сети
	readyChan           chan struct{}              // канал для уведомления о готовности
	readyOnce           sync.Once                  // гарантирует однократное закрытие readyChan
	consensusStarted    bool                       // флаг начала консенсуса
	consensusChan       chan struct{}              // канал для уведомления о начале консенсуса
	consensusOnce       sync.Once                  // гарантирует однократное закрытие consensusChan
	consensusManager    ConsensusManager           // менеджер консенсуса (инжектированная зависимость)
	receivedBlockHashes map[common.Hash]bool       // кэш полученных блоков для защиты от дублирования
	connManager         *connection.Manager        // менеджер соединений
	meshNetwork         *mesh.Network              // mesh сеть для P2P
	peerStore           *mesh.PeerStore            // хранилище информации о пирах
	discovery           *mesh.Discovery            // обнаружение пиров
	seedDiscovery       *discovery.SeedDiscovery   // seed nodes discovery
}


// NewIce создаёт новый экземпляр Ice.
// Определяет IP адрес автоматически, инициализирует все необходимые структуры данных.
// Возвращает ошибку, если не удалось определить IP адрес или создать компонент.
//
// Параметры:
//   - cfg: конфигурация приложения
//   - ctx: контекст для управления жизненным циклом
//   - port: порт для прослушивания входящих подключений
func NewIce(cfg *config.Config, ctx context.Context, port string) (*Ice, error) {
	// Определяем IP адрес автоматически
	currentIP := ""
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	for _, iface := range interfaces {
		ip, _, err := net.ParseCIDR(iface.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			currentIP = ip.String()
			break
		}
	}
	if currentIP == "" {
		return nil, fmt.Errorf("no IPv4 interface found")
	}

	// Инициализируем компоненты mesh сети
	connManager := connection.NewManager(ctx, nil)
	peerStore := mesh.NewPeerStore()
	seedNodes := getSeedNodes(cfg)
	meshDiscovery := mesh.NewDiscovery(ctx, peerStore, seedNodes, connManager)
	meshNetwork := mesh.NewNetwork(ctx, mesh.DefaultNetworkConfig(), peerStore, meshDiscovery, connManager)
	seedDiscovery := discovery.NewSeedDiscovery(ctx, seedNodes, connManager)

	ice := &Ice{
		cfg:                 cfg,
		ctx:                 ctx,
		status:              0x0,
		started:             false,
		address:             cfg.NetCfg.ADDR, // адрес cerera из конфигурации
		ip:                  currentIP,
		port:                port,
		networkReady:        false,
		readyChan:           make(chan struct{}),
		consensusStarted:    false,
		consensusChan:       make(chan struct{}),
		consensusManager:    NewGigeaConsensusManager(),       // используем адаптер для gigea по умолчанию
		receivedBlockHashes: make(map[common.Hash]bool),       // кэш для защиты от дублирования блоков
		connManager:         connManager,
		meshNetwork:         meshNetwork,
		peerStore:           peerStore,
		discovery:           meshDiscovery,
		seedDiscovery:       seedDiscovery,
	}

	icelogger().Infow("Ice component created",
		"address", cfg.NetCfg.ADDR.Hex(),
		"ip", currentIP,
		"port", port,
	)

	return ice, nil
}
