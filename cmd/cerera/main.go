package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/net"
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea/gigea"
)

// Process управляет жизненным циклом приложения.
type Process struct{}

// Stop завершает работу приложения.
func (p *Process) Stop() {
	fmt.Println("Stopping...")
	fmt.Println("Stopped!")
}

// Cerera объединяет основные компоненты приложения.
type Cerera struct {
	bc     *chain.Chain
	g      *validator.Validator
	h      *network.Node
	p      *pool.Pool
	proc   Process
	v      *storage.Vault
	status [8]byte
}

// NewCerera создаёт и инициализирует экземпляр Cerera.
func NewCerera(cfg *config.Config, ctx context.Context, mode, address string, httpPort int, mine bool) (*Cerera, error) {
	// Инициализация внутренних компонентов
	if err := gigea.Init(cfg.NetCfg.ADDR); err != nil {
		return nil, fmt.Errorf("failed to init gigea: %w", err)
	}
	if err := coinbase.InitOperationData(); err != nil {
		return nil, fmt.Errorf("failed to init coinbase: %w", err)
	}

	// Инициализация хранилища
	vault, err := storage.NewD5Vault(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}

	// Инициализация сети
	if mode == "p2p" {
		go net.StartNode(address, cfg.NetCfg.ADDR)
	} else {
		if err := network.NewServer(cfg, mode, address); err != nil {
			return nil, fmt.Errorf("failed to start network server: %w", err)
		}
	}
	go network.SetUpHttp(httpPort)
	go network.NewWsManager().Start()

	// Инициализация валидатора, цепочки и пула
	val, err := validator.NewValidator(ctx, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init validator: %w", err)
	}
	if err := chain.InitBlockChain(cfg); err != nil {
		return nil, fmt.Errorf("failed to init blockchain: %w", err)
	}
	poolInstance, err := pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to init pool: %w", err)
	}

	// Инициализация майнера
	// if err := miner.Init(); err != nil {
	// 	return nil, fmt.Errorf("failed to init miner: %w", err)
	// }
	// if mine {
	// 	go miner.Run()
	// }

	var chain = chain.GetBlockChain()

	gigea.E.Register(chain)
	// gigea.E.Register(miner.GetMiner())

	return &Cerera{
		bc:     chain,
		g:      &val,
		p:      poolInstance,
		v:      &vault,
		proc:   Process{},
		status: [8]byte{0xf, val.Status(), poolInstance.Status, vault.Status(), 0x0, 0x3, 0x1, 0x7},
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
	mine := flag.Bool("miner", false, "Флаг для добычи новых блоков")
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

	// Ожидание завершения
	<-ctx.Done()
	app.proc.Stop()
}
