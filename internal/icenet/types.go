package icenet

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/types"
	"go.uber.org/zap"
)

const ICE_SERVICE_NAME = "ICE_CERERA_001_1_0"

const (
	bootstrapIP   = "192.168.1.6"
	bootstrapPort = "31100"
)

// icelogger returns a sugared logger for the icenet package.
// It is defined as a function (not a global variable) so that it always
// uses the logger configured by logger.Init(), even if this package is
// imported before logging is set up in main().
func icelogger() *zap.SugaredLogger {
	return logger.Named("icenet")
}

// Ice представляет основной компонент Ice
type Ice struct {
	cfg               *config.Config
	ctx               context.Context
	status            byte
	started           bool
	address           types.Address              // адрес cerera
	ip                string                     // IP адрес узла
	port              string                     // порт узла
	bootstrapIP       string                     // IP адрес bootstrap узла
	bootstrapPort     string                     // порт bootstrap узла
	listener          net.Listener               // listener для входящих подключений
	bootstrapConn     net.Conn                   // постоянное соединение с bootstrap
	connectedNodes    map[types.Address]net.Conn // активные соединения с узлами (только для bootstrap)
	confirmedNodes    map[types.Address]int      // количество подтверждений от каждого узла (только для bootstrap)
	mu                sync.Mutex                 // мьютекс для потокобезопасности
	lastSentBlockHash common.Hash                // хеш последнего отправленного блока
	bootstrapReady    bool                       // флаг готовности bootstrap соединения
	readyChan         chan struct{}              // канал для уведомления о готовности
	consensusStarted  bool                       // флаг начала консенсуса
	consensusChan     chan struct{}              // канал для уведомления о начале консенсуса
}

// NewIce создаёт новый экземпляр Ice
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

	ice := &Ice{
		cfg:              cfg,
		ctx:              ctx,
		status:           0x0,
		started:          false,
		address:          cfg.NetCfg.ADDR, // адрес cerera из конфигурации
		ip:               currentIP,
		port:             port,
		bootstrapIP:      bootstrapIP,
		bootstrapPort:    bootstrapPort,
		bootstrapReady:   false,
		readyChan:        make(chan struct{}),
		consensusStarted: false,
		consensusChan:    make(chan struct{}),
		connectedNodes:   make(map[types.Address]net.Conn), // инициализируем map для хранения соединений
		confirmedNodes:   make(map[types.Address]int),      // инициализируем map для отслеживания подтверждений (каждый узел должен подтвердить 2 раза)
	}

	icelogger().Infow("Ice component created",
		"address", cfg.NetCfg.ADDR.Hex(),
		"ip", currentIP,
		"port", port,
		"bootstrap", fmt.Sprintf("%s:%s", bootstrapIP, bootstrapPort),
	)

	return ice, nil
}
