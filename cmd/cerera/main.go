package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/miner"
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea"
)

// Cerera объединяет основные компоненты приложения.
type Cerera struct {
	bc *chain.Chain
	g  *validator.Validator
	// h        *network.Node
	p        pool.TxPool // CHANGE TO INTERFACE BUT WHY?
	v        *storage.Vault
	registry *service.Registry
	status   [8]byte
}

// NewCerera создаёт и инициализирует экземпляр Cerera.
func NewCerera(cfg *config.Config, ctx context.Context, mode, address string, httpPort int, mine bool) (*Cerera, error) {
	registry, err := service.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create service registry: %w", err)
	}

	// Инициализация внутренних компонентов
	if err := gigea.Init(ctx, cfg.NetCfg.ADDR); err != nil {
		return nil, fmt.Errorf("failed to init gigea: %w", err)
	}

	// coinbase
	if err := coinbase.InitOperationData(); err != nil {
		return nil, fmt.Errorf("failed to init coinbase: %w", err)
	}

	// Инициализация хранилища
	vault, err := storage.NewD5Vault(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}
	registry.Register(vault.ServiceName(), vault)

	// Инициализация цепочки
	chain, err := chain.InitBlockChain(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init blockchain: %w", err)
	}
	registry.Register(chain.ServiceName(), chain)

	// инициализация валидатора
	validator, err := validator.NewValidator(ctx, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init validator: %w", err)
	}
	registry.Register(validator.ServiceName(), validator)

	// инициализация пула
	mempool, err := pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to init pool: %w", err)
	}
	// register pool in registry
	registry.Register(mempool.ServiceName(), mempool)

	// Инициализация сети
	// if mode == "p2p" {
	// 	go net.StartNode(address, cfg.NetCfg.ADDR)
	// } else {
	// 	if err := network.NewServer(cfg, mode, address); err != nil {
	// 		return nil, fmt.Errorf("failed to start network server: %w", err)
	// 	}
	// }
	if err := network.SetUpHttp(ctx, cfg, httpPort); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
	// network.NewWsManager().Start(ctx)

	// Инициализация майнера
	miner, err := miner.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init miner: %w", err)
	}
	if mine {
		miner.Start()
	}

	// gigea.E.Register(chain)
	// gigea.E.Register(miner.GetMiner())
	mempool.Register(miner)

	return &Cerera{
		bc:       chain,
		g:        &validator,
		p:        mempool,
		v:        &vault,
		registry: registry,
		status:   [8]byte{0xf, validator.Status(), 0x4, vault.Status(), 0x0, 0x3, 0x1, 0x7},
	}, nil
}

// setupLogging настраивает логирование в файл.
func setupLogging() error {
	f, err := os.OpenFile("logfile", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}
	log.SetOutput(f)
	return nil
}

// parseFlags разбирает аргументы командной строки.
func parseFlags() (config.Config, string, string, int, bool, bool) {
	addr := flag.String("addr", "31000", "p2p address for connection")
	keyPath := flag.String("key", "", "path to pem key")
	mode := flag.String("mode", "server", "Режим работы: server, client, p2p")
	// address := flag.String("address", "127.0.0.1:10001", "Адрес для подключения или прослушивания")
	http := flag.Int("http", 8080, "Порт для http сервера")
	mine := flag.Bool("miner", true, "Флаг для добычи новых блоков")
	inMem := flag.Bool("mem", true, "Хранение данных память/диск")
	flag.Parse()

	cfg := config.GenerageConfig()
	cfg.SetNodeKey(*keyPath)
	cfg.SetAutoGen(true)
	cfg.SetInMem(*inMem)

	return *cfg, *mode, *addr, *http, *mine, *inMem
}

func main() {
	// Настройка логирования
	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	// Парсинг флагов и создание конфигурации
	cfg, mode, address, httpPort, mine, _ := parseFlags()

	// Создание контекста с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Инициализация приложения
	app, err := NewCerera(&cfg, ctx, mode, address, httpPort, mine)
	if err != nil {
		log.Printf("Failed to initialize Cerera: %v", err)
		os.Exit(1)
	}
	fmt.Printf("\t<--------Cerera Status: %x-------->\r\n", app.status)

	// Ожидание сигнала завершения
	<-ctx.Done()

	// Создаем контекст с таймаутом для graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Ждем завершения или таймаута
	select {
	case <-shutdownCtx.Done():
		log.Println("Таймаут graceful shutdown, принудительное завершение")
		os.Exit(1)
	default:
		log.Println("Приложение корректно завершено")
	}
}
