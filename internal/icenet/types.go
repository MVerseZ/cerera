package icenet

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/types"
	"go.uber.org/zap"
)

const ICE_SERVICE_NAME = "ICE_CERERA_001_1_0"

// DefaultBootstrapIP is the default bootstrap IP address if not configured
const DefaultBootstrapIP = "192.168.1.6"

// DefaultBootstrapPort is the default bootstrap port if not configured
const DefaultBootstrapPort = "31100"

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
	bootstrapIP         string                     // IP адрес bootstrap узла
	bootstrapPort       string                     // порт bootstrap узла
	listener            net.Listener               // listener для входящих подключений
	bootstrapConn       net.Conn                   // постоянное соединение с bootstrap
	connectedNodes      map[types.Address]net.Conn // активные соединения с узлами (только для bootstrap)
	confirmedNodes      map[types.Address]int      // количество подтверждений от каждого узла (только для bootstrap)
	mu                  sync.RWMutex               // мьютекс для потокобезопасности (RWMutex для оптимизации чтения)
	lastSentBlockHash   common.Hash                // хеш последнего отправленного блока
	bootstrapReady      bool                       // флаг готовности bootstrap соединения
	readyChan           chan struct{}              // канал для уведомления о готовности
	readyOnce           sync.Once                  // гарантирует однократное закрытие readyChan
	consensusStarted    bool                       // флаг начала консенсуса
	consensusChan       chan struct{}              // канал для уведомления о начале консенсуса
	consensusOnce       sync.Once                  // гарантирует однократное закрытие consensusChan
	consensusManager    ConsensusManager           // менеджер консенсуса (инжектированная зависимость)
	receivedBlockHashes map[common.Hash]bool       // кэш полученных блоков для защиты от дублирования
}

// getBootstrapIP returns the bootstrap IP from config, environment variable, or default
func getBootstrapIP(cfg *config.Config) string {
	// Check environment variable first
	if envIP := os.Getenv("CERERA_BOOTSTRAP_IP"); envIP != "" {
		return envIP
	}
	// Use config value if set
	if cfg != nil && cfg.NetCfg.BootstrapIP != "" {
		return cfg.NetCfg.BootstrapIP
	}
	// Fall back to default
	return DefaultBootstrapIP
}

// getBootstrapPort returns the bootstrap port from config, environment variable, or default
func getBootstrapPort(cfg *config.Config) string {
	// Check environment variable first
	if envPort := os.Getenv("CERERA_BOOTSTRAP_PORT"); envPort != "" {
		return envPort
	}
	// Use config value if set
	if cfg != nil && cfg.NetCfg.BootstrapPort != "" {
		return cfg.NetCfg.BootstrapPort
	}
	// Fall back to default
	return DefaultBootstrapPort
}

// NewIce создаёт новый экземпляр Ice.
// Определяет IP адрес автоматически, инициализирует все необходимые структуры данных
// и настраивает bootstrap адрес из конфигурации или переменных окружения.
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

	bootstrapIP := getBootstrapIP(cfg)
	bootstrapPort := getBootstrapPort(cfg)

	ice := &Ice{
		cfg:                 cfg,
		ctx:                 ctx,
		status:              0x0,
		started:             false,
		address:             cfg.NetCfg.ADDR, // адрес cerera из конфигурации
		ip:                  currentIP,
		port:                port,
		bootstrapIP:         bootstrapIP,
		bootstrapPort:       bootstrapPort,
		bootstrapReady:      false,
		readyChan:           make(chan struct{}),
		consensusStarted:    false,
		consensusChan:       make(chan struct{}),
		connectedNodes:      make(map[types.Address]net.Conn), // инициализируем map для хранения соединений
		confirmedNodes:      make(map[types.Address]int),      // инициализируем map для отслеживания подтверждений (каждый узел должен подтвердить 2 раза)
		consensusManager:    NewGigeaConsensusManager(),       // используем адаптер для gigea по умолчанию
		receivedBlockHashes: make(map[common.Hash]bool),       // кэш для защиты от дублирования блоков
	}

	icelogger().Infow("Ice component created",
		"address", cfg.NetCfg.ADDR.Hex(),
		"ip", currentIP,
		"port", port,
		"bootstrap", fmt.Sprintf("%s:%s", bootstrapIP, bootstrapPort),
		"bootstrap_source", getBootstrapSource(cfg),
	)

	return ice, nil
}

// getBootstrapSource returns a string indicating where the bootstrap config came from
func getBootstrapSource(cfg *config.Config) string {
	if os.Getenv("CERERA_BOOTSTRAP_IP") != "" || os.Getenv("CERERA_BOOTSTRAP_PORT") != "" {
		return "environment"
	}
	if cfg != nil && (cfg.NetCfg.BootstrapIP != "" || cfg.NetCfg.BootstrapPort != "") {
		return "config"
	}
	return "default"
}
