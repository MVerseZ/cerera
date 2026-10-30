package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
	"github.com/cerera/gigea"
	"github.com/cerera/icenet"
	"github.com/cerera/icenet/fork"
	"github.com/cerera/internal/logger"
	"github.com/cerera/internal/network"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validator"
	"github.com/cerera/miner"
)

var appLog = logger.Named("cmd.cerera")

// Cerera объединяет основные компоненты приложения.
type Cerera struct {
	bc       *chain.Chain
	g        validator.Validator
	p        pool.TxPool
	v        storage.Vault
	txTable  *storage.TxTable
	registry *service.Registry
	ice      *icenet.Ice
	m        miner.Miner
	status   [8]byte

	shutdownOnce sync.Once
	shutdownErr  error
}

// NewCerera создаёт и инициализирует экземпляр Cerera.
func NewCerera(cfg *config.Config, ctx context.Context, mode, port string, httpPort int, mine bool) (*Cerera, error) {
	registry, err := service.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create service registry: %w", err)
	}
	ctx = service.WithRegistry(ctx, registry)

	// Инициализация внутренних компонентов
	if err := gigea.Init(ctx, cfg.NetCfg.ADDR); err != nil {
		return nil, fmt.Errorf("failed to init gigea: %w", err)
	}

	// Инициализация хранилища
	vault, err := storage.NewD5Vault(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}
	if err := registry.Register(&service.Service{
		Name:    vault.ServiceName(),
		Methods: vault.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register vault RPC service: %w", err)
	}

	//  цепочки
	bc, err := chain.Mold(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to mold blockchain parts: %w", err)
	}
	registry.Register(&service.Service{
		Name:    bc.ServiceName(),
		Methods: bc.Methods(),
	})

	d5Vault, ok := vault.(*storage.D5Vault)
	if !ok {
		return nil, fmt.Errorf("vault is not *storage.D5Vault")
	}
	d5Vault.SaveBaseline()

	txTable := storage.NewTxTable()

	validatorInst, err := validator.NewValidator(ctx, *cfg, d5Vault, txTable)
	if err != nil {
		return nil, fmt.Errorf("failed to init validator: %w", err)
	}
	registry.Register(&service.Service{
		Name:    validatorInst.ServiceName(),
		Methods: validatorInst.Methods(),
	})

	// инициализация пула
	mempool, err := pool.InitPool(cfg.POOL.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to init pool: %w", err)
	}
	// register pool in registry
	if err := registry.Register(&service.Service{
		Name:    mempool.ServiceName(),
		Methods: mempool.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register pool RPC service: %w", err)
	}

	// Инициализация http сервера
	if err := network.SetUpHttp(ctx, cfg, httpPort); err != nil {
		return nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Инициализация Ice компонента
	ice, err := icenet.Start(cfg, ctx, port)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ice: %w", err)
	}

	//
	validatorInst.SetPool(mempool)
	validatorInst.SetChain(bc)

	mempool.SetTxValidator(func(tx *types.GTransaction) bool {
		return validatorInst.ValidateRawTransaction(*tx)
	})
	ice.AttachBlockValidator(validatorInst.GetBlockValidator())
	ice.SetTxValidator(func(tx *types.GTransaction) bool {
		return validatorInst.ValidateRawTransaction(*tx)
	})
	ice.SetTxSubmitter(mempool.AddRawTransaction)
	ice.SetVault(d5Vault)

	chain.SetReorgHandler(validatorInst.ReplayChain)
	sp := ice.ServiceProviderPtr()
	if sp != nil {
		sp.SetChainRef(bc)
		sp.SetVault(d5Vault)
	}

	// Инициализация майнера
	minerInstance, err := miner.Init(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init miner: %w", err)
	}
	minerInstance.SetChain(bc)
	minerInstance.SetPool(mempool)
	minerInstance.SetValidator(validatorInst)
	minerInstance.SetBlockPublisher(ice)
	minerInstance.SetHeightLock(bc)

	forkDet := fork.NewDetector(bc, sp)
	forkDet.SetOnReorg(minerInstance.OnChainReorg)
	ice.SetForkDetector(forkDet)

	ice.SetOnBlockFinalized(minerInstance.OnBlockFinalized)
	if mine {
		if err := minerInstance.Start(); err != nil {
			appLog.Errorw("Failed to start miner", "err", err)
			return nil, fmt.Errorf("failed to start miner: %w", err)
		}
	}

	mempool.Register(minerInstance)

	// Connect components to Ice (sync + handler use same ApiProvider)
	// if ice != nil {
	// Register Ice in service registry for broadcasting
	// 	registry.Register(ice.ServiceName(), ice)
	// 	registry.Register("ice", ice) // Also register with short name
	// }

	// Shared service provider for network/consensus access to chain, vault, pool, etc.
	// sp := service.GetServiceProvider()
	// ice.SetServiceProvider(sp)

	// Wire gigea-level consensus manager to the underlying icenet consensus engine.
	// This keeps node logic decoupled from transport while still allowing future
	// use of gigea.ConsensusManager as a façade.
	// _ = gigea.NewConsensusManager(ctx, icenet.NewNetworkAdapter(ice), cfg.NetCfg.ADDR, sp)

	return &Cerera{
		bc:       bc,
		g:        validatorInst,
		p:        mempool,
		v:        vault,
		txTable:  txTable,
		registry: registry,
		ice:      ice,
		m:        minerInstance,
		status:   [8]byte{0xf, validatorInst.Status(), 0x4, vault.Status(), 0x0, 0x3, 0x1, 0x7},
	}, nil
}

// Shutdown stops node components in order: miner, pool, Ice, then vault.
func (app *Cerera) Shutdown(ctx context.Context) error {
	if app == nil {
		return nil
	}
	app.shutdownOnce.Do(func() {
		var errs []error

		if app.m != nil {
			appLog.Infow("Stopping miner...")
			app.m.Stop()
		}
		if app.p != nil {
			appLog.Infow("Stopping transaction pool...")
			app.p.Stop()
		}
		if app.ice != nil {
			appLog.Infow("Stopping Ice P2P network...")
			app.ice.Stop(ctx)
		}
		if app.v != nil {
			if err := app.v.Close(); err != nil {
				appLog.Errorw("Failed to close vault", "err", err)
				errs = append(errs, err)
			} else {
				appLog.Infow("Vault closed successfully")
			}
		}
		app.shutdownErr = errors.Join(errs...)
	})
	return app.shutdownErr
}

// setupLogging настраивает логирование в файл.
func setupLogging(logPath string) error {
	_, err := logger.Init(logger.Config{
		Path:    logPath,
		Level:   "debug",
		Console: true,
	})
	return err
}

// parseFlags разбирает аргументы командной строки.
func parseFlags() (config.Config, string, string, int, bool, string) {
	port := flag.String("port", "31000", "p2p port for connection")
	keyPath := flag.String("key", "", "path to pem key")
	mode := flag.String("mode", "server", "Режим работы: server, client, or p2p")
	httpPort := flag.Int("http", 8080, "Порт для http сервера")
	mine := flag.Bool("miner", true, "Флаг для добычи новых блоков")
	inMem := flag.Bool("mem", false, "Хранение данных в памяти (true) или на диске (false)")
	dataDir := flag.String("data-dir", "", "Каталог данных: chain.dat, vault/, config.json")
	tls := flag.Bool("s", false, "Включить HTTPS (TLS)")
	flag.Parse()

	if *dataDir == "" {
		if envDir := os.Getenv("CERERA_DATA_DIR"); envDir != "" {
			*dataDir = envDir
		}
	}

	var cfg *config.Config
	var err error
	logPath := "logfile"

	if *dataDir != "" {
		if err := os.MkdirAll(*dataDir, 0o700); err != nil {
			panic(fmt.Errorf("create data dir: %w", err))
		}
		cfgPath := filepath.Join(*dataDir, config.ConfigFileName)
		cfg, err = config.OpenConfig(cfgPath)
		if err != nil {
			panic(err)
		}
		if err := cfg.ApplyDataDir(*dataDir); err != nil {
			panic(err)
		}
		if *inMem {
			cfg.SetInMem(true)
		} else {
			cfg.IN_MEM = false
			cfg.WriteConfigToFile()
		}
		logPath = filepath.Join(*dataDir, "logfile")
	} else {
		cfg, err = config.GenerageConfig()
		if err != nil {
			panic(err)
		}
		cfg.SetInMem(*inMem)
	}

	cfg.SetNodeKey(*keyPath)
	cfg.SetAutoGen(true)
	cfg.SEC.HTTP.TLS = *tls

	return *cfg, *mode, *port, *httpPort, *mine, logPath
}

func main() {

	// Парсинг флагов и создание конфигурации
	cfg, mode, port, httpPort, mine, logPath := parseFlags()

	// Настройка логирования
	if err := setupLogging(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Создание контекста с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Инициализация приложения
	app, err := NewCerera(&cfg, ctx, mode, port, httpPort, mine)
	if err != nil {
		appLog.Errorw("Failed to initialize Cerera", "err", err)
		os.Exit(1)
	}

	appLog.Infow("Cerera node running; press Ctrl+C to stop")
	<-ctx.Done()
	appLog.Infow("Shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		appLog.Errorw("Shutdown completed with errors", "err", err)
	}
}
