package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/storage"
	"github.com/cerera/internal/gigea"
	"github.com/cerera/internal/miner"
	"github.com/cerera/internal/network"
	"github.com/cerera/internal/service"
	"github.com/cerera/internal/validator"
	"github.com/chzyer/readline"
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

	// Инициализация хранилища
	vault, err := storage.NewD5Vault(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}
	registry.Register(vault.ServiceName(), vault)

	// Инициализация цепочки
	chain, err := chain.Mold(cfg)
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

	// Инициализация http сервера
	if err := network.SetUpHttp(ctx, cfg, httpPort); err != nil {
		log.Printf("HTTP server error: %v", err)
	}

	// Инициализация майнера
	miner, err := miner.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init miner: %w", err)
	}
	if mine {
		if err := miner.Start(); err != nil {
			log.Printf("Failed to start miner: %v", err)
			return nil, fmt.Errorf("failed to start miner: %w", err)
		}
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

// parseInteractive запрашивает параметры в интерактивном режиме.
func parseInteractive() (config.Config, string, string, int, bool, bool) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Cerera Configuration ===")
	fmt.Println()

	// P2P Port
	fmt.Print("P2P port for connection [31000]: ")
	portInput, _ := reader.ReadString('\n')
	port := strings.TrimSpace(portInput)
	if port == "" {
		port = "31000"
	}

	// Key path
	fmt.Print("Path to PEM key (optional, press Enter to set to [ddddd.nodekey.pem]): ")
	keyInput, _ := reader.ReadString('\n')
	keyPath := strings.TrimSpace(keyInput)
	if keyPath == "" {
		keyPath = "ddddd.nodekey.pem"
	}

	// Mode
	fmt.Print("Mode (server/client/p2p) [p2p]: ")
	modeInput, _ := reader.ReadString('\n')
	mode := strings.TrimSpace(modeInput)
	if mode == "" {
		mode = "p2p"
	}
	for mode != "server" && mode != "client" && mode != "p2p" {
		fmt.Print("Invalid mode. Please enter server, client, or p2p: ")
		modeInput, _ = reader.ReadString('\n')
		mode = strings.TrimSpace(modeInput)
	}

	// HTTP Port
	fmt.Print("HTTP server port [1337]: ")
	httpInput, _ := reader.ReadString('\n')
	httpStr := strings.TrimSpace(httpInput)
	http := 1337
	if httpStr != "" {
		if parsed, err := strconv.Atoi(httpStr); err == nil {
			http = parsed
		}
	}

	// Miner
	fmt.Print("Enable mining? (y/n) [y]: ")
	mineInput, _ := reader.ReadString('\n')
	mineStr := strings.ToLower(strings.TrimSpace(mineInput))
	mine := true
	if mineStr == "n" || mineStr == "no" {
		mine = false
	}

	// In Memory
	fmt.Print("Store data in memory? (y/n) [y]: ")
	memInput, _ := reader.ReadString('\n')
	memStr := strings.ToLower(strings.TrimSpace(memInput))
	inMem := true
	if memStr == "n" || memStr == "no" {
		inMem = false
	}

	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  P2P Port: %s\n", port)
	fmt.Printf("  Key Path: %s\n", keyPath)
	fmt.Printf("  Mode: %s\n", mode)
	fmt.Printf("  HTTP Port: %d\n", http)
	fmt.Printf("  Mining: %v\n", mine)
	fmt.Printf("  In Memory: %v\n", inMem)
	fmt.Println()

	cfg := config.GenerageConfig()
	cfg.SetNodeKey(keyPath)
	cfg.SetAutoGen(true)
	cfg.SetInMem(inMem)

	return *cfg, mode, port, http, mine, inMem
}

func main() {
	// Настройка логирования
	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}

	// Проверяем наличие конфига
	var cfg config.Config
	var mode string
	var port string
	var httpPort int
	var mine bool

	if _, err := os.Stat("config.json"); err == nil {
		// Конфиг существует, загружаем его
		cfgPtr := config.GenerageConfig()
		cfg = *cfgPtr

		// Используем значения из конфига
		if cfg.NetCfg.P2P != 0 {
			port = strconv.Itoa(cfg.NetCfg.P2P)
		} else {
			port = "31000"
		}
		if cfg.NetCfg.RPC != 0 {
			httpPort = cfg.NetCfg.RPC
		} else {
			httpPort = 1337
		}

		// Значения по умолчанию для параметров, которых нет в конфиге
		mode = "p2p"
		mine = true
	} else {
		// Конфига нет, используем интерактивный ввод
		cfg, mode, port, httpPort, mine, _ = parseInteractive()
	}

	// Создание контекста с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("cfg: ", cfg)
	fmt.Println("mode: ", mode)
	fmt.Println("port: ", port)
	fmt.Println("httpPort: ", httpPort)
	fmt.Println("mine: ", mine)

	// Инициализация приложения
	app, err := NewCerera(&cfg, ctx, mode, port, httpPort, mine)
	if err != nil {
		log.Printf("Failed to initialize Cerera: %v", err)
		os.Exit(1)
	}

	// dht, err := mesh.Start(&cfg, ctx, port)
	// if err != nil {
	// 	log.Printf("Failed to initialize DHT: %v", err)
	// 	os.Exit(1)
	// }
	// mesh.Connect(app.cmdChan)

	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()
	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}
		input := strings.Split(line, " ")
		switch input[0] {
		// case "store":
		// 	id, err := dht.Store([]byte(input[1]))
		// 	if err != nil {
		// 		fmt.Println(err.Error())
		// 	}
		// 	fmt.Println("Stored ID: " + id)
		// case "get":
		// 	data, exists, err := dht.Get(input[1])
		// 	if err != nil {
		// 		fmt.Println(err.Error())
		// 	}
		// 	fmt.Println("Searching for", input[1])
		// 	if exists {
		// 		fmt.Println("..Found data:", string(data))
		// 	} else {
		// 		fmt.Println("..Nothing found for this key!")
		// 	}
		// case "info":
		// 	nodes := dht.NumNodes()
		// 	self := dht.GetSelfID()
		// 	addr := dht.GetNetworkAddr()
		// 	fmt.Println("Addr: " + addr)
		// 	fmt.Println("ID: " + self)
		// 	fmt.Println("Known Nodes: " + strconv.Itoa(nodes))
		case "balance", "b":
			// app.ExecuteCli("balance")
			fmt.Println("Node balance")
		case "send":
			fmt.Println("Send transaction")
		case "status":
			fmt.Printf("Status: %x\n", app.status)
		case "help":
			fmt.Println(Usage())
		case "exit":
			os.Exit(0)
		default:
			fmt.Println("Unknown command, use help to see available commands")
		}
	}

	// Ожидание сигнала завершения
	<-ctx.Done()

	log.Println("Получен сигнал завершения, начинаем graceful shutdown...")

	// Закрываем vault (закрывает bitcask базу данных)
	if app.v != nil {
		if err := (*app.v).Close(); err != nil {
			log.Printf("Ошибка при закрытии vault: %v", err)
		} else {
			log.Println("Vault успешно закрыт")
		}
	}

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
