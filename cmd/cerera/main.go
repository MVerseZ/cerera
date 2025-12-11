package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/miner"
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/gigea"
	"github.com/cerera/internal/icenet"
)

var appLog = logger.Named("cmd.cerera")

// Cerera объединяет основные компоненты приложения.
type Cerera struct {
	bc *chain.Chain
	g  *validator.Validator
	// h        *network.Node
	p        pool.TxPool // CHANGE TO INTERFACE BUT WHY?
	v        *storage.Vault
	registry *service.Registry
	ice      *icenet.Ice
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

	// Инициализация хранилища
	vault, err := storage.NewD5Vault(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}
	registry.Register(vault.ServiceName(), vault)

	//  цепочки
	chain, err := chain.Mold(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to mold blockchain parts: %w", err)
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

	// Инициализация http сервера
	if err := network.SetUpHttp(ctx, cfg, httpPort); err != nil {
		appLog.Warnw("HTTP server error", "err", err)
	}

	// Инициализация майнера
	miner, err := miner.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init miner: %w", err)
	}
	if mine {
		if err := miner.Start(); err != nil {
			appLog.Errorw("Failed to start miner", "err", err)
			return nil, fmt.Errorf("failed to start miner: %w", err)
		}
	}

	// gigea.E.Register(chain)
	// gigea.E.Register(miner.GetMiner())
	mempool.Register(miner)

	// Инициализация Ice компонента
	ice, err := icenet.Start(cfg, ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ice: %w", err)
	}
	registry.Register(ice.ServiceName(), ice)
	appLog.Infow("Ice component initialized",
		"address", ice.GetAddress().Hex(),
		"network_addr", ice.GetNetworkAddr(),
	)

	return &Cerera{
		bc:       chain,
		g:        &validator,
		p:        mempool,
		v:        &vault,
		registry: registry,
		ice:      ice,
		status:   [8]byte{0xf, validator.Status(), 0x4, vault.Status(), 0x0, 0x3, 0x1, 0x7},
	}, nil
}

// setupLogging настраивает логирование в файл.
func setupLogging() error {
	_, err := logger.Init(logger.Config{
		Path:    "logfile",
		Level:   "info",
		Console: true,
	})
	return err
}

// twin live change famous blue aspect control edge choose dragon sleep tissue drip match predict leopard weekend orient clap aim fluid toy fall nuclear
// parseFlags разбирает аргументы командной строки.
func parseFlags() (config.Config, string, string, int, bool, bool) {
	port := flag.String("port", "31000", "p2p port for connection")
	keyPath := flag.String("key", "", "path to pem key")
	mode := flag.String("mode", "server", "Режим работы: server, client, p2p")
	// address := flag.String("address", "127.0.0.1:10001", "Адрес для подключения или прослушивания")
	http := flag.Int("http", 8080, "Порт для http сервера")
	mine := flag.Bool("miner", true, "Флаг для добычи новых блоков")
	inMem := flag.Bool("mem", true, "Хранение данных память/диск")
	tls := flag.Bool("s", false, "Включить HTTPS (TLS)")
	flag.Parse()

	cfg := config.GenerageConfig()
	cfg.SetNodeKey(*keyPath)
	cfg.SetAutoGen(true)
	cfg.SetInMem(*inMem)
	cfg.SEC.HTTP.TLS = *tls

	return *cfg, *mode, *port, *http, *mine, *inMem
}

func main() {
	// Настройка логирования
	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Парсинг флагов и создание конфигурации
	cfg, mode, port, httpPort, mine, _ := parseFlags()

	// Создание контекста с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Инициализация приложения
	app, err := NewCerera(&cfg, ctx, mode, port, httpPort, mine)
	if err != nil {
		appLog.Errorw("Failed to initialize Cerera", "err", err)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		sig := <-sigs
		fmt.Println("Received signal: ", sig)
		// Закрываем vault (закрывает bitcask базу данных)
		if app != nil && app.v != nil {
			if err := (*app.v).Close(); err != nil {
				appLog.Errorw("Ошибка при закрытии vault", "err", err)
			} else {
				appLog.Infow("Vault успешно закрыт")
			}
		}
		// Закрываем Ice компонент
		if app != nil && app.ice != nil {
			appLog.Infow("Shutting down Ice component...")
			if err := app.ice.Close(); err != nil {
				appLog.Errorw("Ошибка при закрытии Ice", "err", err)
			} else {
				appLog.Infow("Ice component успешно закрыт")
			}
		}
		time.Sleep(2 * time.Second)
		done <- true
	}()

	<-done

	// // Ожидание сигнала завершения
	// <-ctx.Done()

	// log.Println("Получен сигнал завершения, начинаем graceful shutdown...")

	// // Останавливаем другие компоненты через registry, если они поддерживают остановку
	// if app.registry != nil {
	// 	if err := app.registry.StopAllServices(); err != nil {
	// 		log.Printf("Ошибка при остановке сервисов: %v", err)
	// 	}
	// }

	// // Даем время на завершение операций записи в базу данных
	// time.Sleep(100 * time.Millisecond)

	// log.Println("Приложение корректно завершено")

}
