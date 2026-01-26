package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/common"
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

// HeightLockManager manages height locking to prevent forks
// When a block is received from another node, it locks the height
// and signals the miner to cancel mining for that height
type HeightLockManager struct {
	mu           sync.RWMutex
	lockedHeight int           // Height that is already locked (has a block)
	cancelChan   chan struct{} // Channel to signal mining cancellation
	cancelChanMu sync.Mutex    // Mutex for cancelChan operations
}

// Global height lock manager instance
var globalHeightLock = NewHeightLockManager()

// NewHeightLockManager creates a new height lock manager
func NewHeightLockManager() *HeightLockManager {
	return &HeightLockManager{
		lockedHeight: 0,
		cancelChan:   make(chan struct{}),
	}
}

// TryLockHeight attempts to lock a height for mining
// Returns true if the height can be mined, false if already locked
func (hlm *HeightLockManager) TryLockHeight(height int) bool {
	hlm.mu.RLock()
	defer hlm.mu.RUnlock()
	return height > hlm.lockedHeight
}

// IsHeightLocked checks if a height is already locked
func (hlm *HeightLockManager) IsHeightLocked(height int) bool {
	hlm.mu.RLock()
	defer hlm.mu.RUnlock()
	return height <= hlm.lockedHeight
}

// LockHeight locks a height when a block is received from another node
// This also signals cancellation to any ongoing mining
func (hlm *HeightLockManager) LockHeight(height int) {
	hlm.mu.Lock()
	defer hlm.mu.Unlock()

	if height > hlm.lockedHeight {
		hlm.lockedHeight = height

		// Signal cancellation to mining
		hlm.cancelChanMu.Lock()
		select {
		case <-hlm.cancelChan:
			// Already closed, create a new one
		default:
			close(hlm.cancelChan)
		}
		hlm.cancelChan = make(chan struct{})
		hlm.cancelChanMu.Unlock()

		appLog.Infow("Height locked by external block", "height", height)
	}
}

// GetCancelChannel returns the current cancellation channel
func (hlm *HeightLockManager) GetCancelChannel() <-chan struct{} {
	hlm.cancelChanMu.Lock()
	defer hlm.cancelChanMu.Unlock()
	return hlm.cancelChan
}

// GetLockedHeight returns the current locked height
func (hlm *HeightLockManager) GetLockedHeight() int {
	hlm.mu.RLock()
	defer hlm.mu.RUnlock()
	return hlm.lockedHeight
}

// GetGlobalHeightLock returns the global height lock manager
func GetGlobalHeightLock() *HeightLockManager {
	return globalHeightLock
}

// ChainAdapter adapts chain.Chain to icenet.ChainProvider interface
type ChainAdapter struct {
	chain      *chain.Chain
	validator  validator.Validator
	heightLock *HeightLockManager
}

func (ca *ChainAdapter) GetCurrentHeight() int {
	if ca.chain == nil {
		return 0
	}
	latestBlock := ca.chain.GetLatestBlock()
	if latestBlock == nil || latestBlock.Head == nil {
		return 0
	}
	return latestBlock.Header().Height
}

func (ca *ChainAdapter) GetBlockByHeight(height int) *block.Block {
	if ca.chain == nil {
		return nil
	}
	return ca.chain.GetBlockByNumber(height)
}

func (ca *ChainAdapter) GetBlockByHash(hash common.Hash) *block.Block {
	if ca.chain == nil {
		return nil
	}
	return ca.chain.GetBlock(hash)
}

func (ca *ChainAdapter) GetBestHash() common.Hash {
	if ca.chain == nil {
		return common.EmptyHash()
	}
	latestBlock := ca.chain.GetLatestBlock()
	if latestBlock == nil {
		return common.EmptyHash()
	}
	return latestBlock.GetHash()
}

func (ca *ChainAdapter) GetGenesisHash() common.Hash {
	if ca.chain == nil {
		return common.EmptyHash()
	}
	genesisBlock := ca.chain.GetBlockByNumber(0)
	if genesisBlock == nil {
		return common.EmptyHash()
	}
	return genesisBlock.GetHash()
}

func (ca *ChainAdapter) AddBlock(b *block.Block) error {
	if ca.chain == nil || b == nil {
		return fmt.Errorf("chain or block is nil")
	}

	// Check if we already have a block at this height
	currentHeight := ca.GetCurrentHeight()
	if b.Head.Height <= currentHeight {
		appLog.Debugw("Skipping block - already have block at this height",
			"receivedHeight", b.Head.Height,
			"currentHeight", currentHeight,
			"hash", b.Hash.Hex())
		return nil // Not an error, just skip
	}

	// Check if this is the next expected block
	if b.Head.Height != currentHeight+1 {
		return fmt.Errorf("block height %d is not sequential (expected %d)", b.Head.Height, currentHeight+1)
	}

	// Execute transactions in the block (critical for account sync!)
	if ca.validator != nil && b.Transactions != nil {
		for _, tx := range b.Transactions {
			if err := ca.validator.ExecuteTransaction(tx); err != nil {
				appLog.Warnw("Failed to execute tx during sync", "hash", tx.Hash(), "err", err)
			}
		}
	}

	// Update chain with the block
	if err := ca.chain.UpdateChain(b); err != nil {
		return err
	}

	// Lock this height to prevent local mining from adding a competing block
	// This signals the miner to cancel mining for this height
	if ca.heightLock != nil {
		ca.heightLock.LockHeight(b.Head.Height)
	}

	appLog.Infow("Block added to chain from network", "height", b.Head.Height, "hash", b.Hash.Hex())

	return nil
}

func (ca *ChainAdapter) GetChainID() int {
	if ca.chain == nil {
		return 0
	}
	return ca.chain.GetChainId()
}

func (ca *ChainAdapter) ValidateBlock(b *block.Block) error {
	if ca.chain == nil || b == nil {
		return fmt.Errorf("chain or block is nil")
	}

	// Basic validation
	if b.Head == nil {
		return fmt.Errorf("block header is nil")
	}

	// Check parent hash for non-genesis blocks
	if b.Head.Height > 0 {
		parentBlock := ca.chain.GetBlockByNumber(b.Head.Height - 1)
		if parentBlock == nil {
			return fmt.Errorf("parent block not found at height %d", b.Head.Height-1)
		}
		if b.Head.PrevHash != parentBlock.GetHash() {
			return fmt.Errorf("invalid parent hash")
		}
	}

	return nil
}

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
func NewCerera(cfg *config.Config, ctx context.Context, mode, port string, httpPort int, mine bool) (*Cerera, error) {
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
	minerInstance, err := miner.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init miner: %w", err)
	}
	if mine {
		if err := minerInstance.Start(); err != nil {
			appLog.Errorw("Failed to start miner", "err", err)
			return nil, fmt.Errorf("failed to start miner: %w", err)
		}
	}

	// gigea.E.Register(chain)
	// gigea.E.Register(miner.GetMiner())
	mempool.Register(minerInstance)

	// Инициализация Ice компонента
	ice, err := icenet.Start(cfg, ctx, port)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ice: %w", err)
	}

	// Connect chain to Ice for sync
	if ice != nil && chain != nil {
		ice.SetChainProvider(&ChainAdapter{
			chain:      chain,
			validator:  validator,
			heightLock: globalHeightLock,
		})

		// Set height lock provider for fork prevention
		ice.SetHeightLockProvider(globalHeightLock)

		// Register Ice in service registry for broadcasting
		registry.Register(ice.ServiceName(), ice)
		registry.Register("ice", ice) // Also register with short name
	}

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
			app.ice.Stop(ctx)
		}
		time.Sleep(2 * time.Second)
		done <- true
	}()

	<-done

}
