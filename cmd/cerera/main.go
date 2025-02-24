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
	"github.com/cerera/internal/cerera/miner"
	"github.com/cerera/internal/cerera/net"
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
	"github.com/cerera/internal/coinbase"
	"github.com/cerera/internal/gigea/gigea"
)

type Process struct {
}

func (p *Process) Stop() {
	fmt.Printf("Stopping...\r\n")
	fmt.Printf("Stopped!\r\n")
}

type cerera struct {
	bc     chain.Chain
	g      validator.Validator
	h      *network.Node
	p      *pool.Pool
	proc   Process
	v      storage.Vault
	status [8]byte
}

// todo run as daemin service
func main() {
	// listenRpcPortParam := flag.Int("r", -1, "rpc port to listen")
	// listenP2pPortParam := flag.Int("l", -1, "p2p port for connections")
	addr := flag.String("addr", "", "p2p address for connection")
	// port := flag.Int("p", 10101, "p2p port for connection")
	// gossipAddress := flag.String("g", "", "gossip address")
	keyPathFlag := flag.String("key", "", "path to pem key")
	// logto := flag.String("logto", "stdout", "file path to log to, \"syslog\" or \"stdout\"")

	mode := flag.String("mode", "server", "Режим работы: server, client, p2p")
	address := flag.String("address", "127.0.0.1:10001", "Адрес для подключения или прослушивания")
	http := flag.Int("http", 8080, "Порт для http сервера")
	mine := flag.Bool("miner", false, "Флаг для добычи новых блоков")

	inMemFlag := flag.Bool("mem", true, "Хранение данных память/диск")
	flag.Parse()

	// init log
	// Open log file
	f, err := os.OpenFile("logfile", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	cfg := config.GenerageConfig()
	// cfg.SetPorts(*listenRpcPortParam, *listenP2pPortParam)
	cfg.SetNodeKey(*keyPathFlag)
	cfg.SetAutoGen(true)
	cfg.SetInMem(*inMemFlag)

	ctx, _ := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)

	//## No multithreading.
	// start steps

	// inner structs
	gigea.Init(cfg.NetCfg.ADDR)
	coinbase.InitOperationData()

	// public structs
	storage.NewD5Vault(cfg)

	// i/o structs
	if *mode == "p2p" {
		go net.StartNode(*addr, cfg.NetCfg.ADDR)
	} else {
		network.NewServer(cfg, *mode, *address)
	}
	go network.SetUpHttp(*http)
	fmt.Println(*mine)

	miner.Init()
	if *mine {
		miner.Run()
	}

	validator.NewValidator(ctx, *cfg)
	chain.InitBlockChain(cfg)
	pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize)

	c := cerera{
		// g:  validator.NewValidator(ctx, *cfg),
		// bc: chain.InitBlockChain(cfg), //? chain use validator, init it before, not a clean way
		// h: host,
		// p: pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize),
		// v:      storage.NewD5Vault(cfg),
		status: [8]byte{0xf, 0x4, 0x2, 0xb, 0x0, 0x3, 0x1, 0x7},
	}
	<-ctx.Done()
	c.proc.Stop()
}

// stages:
// start app
// check network connection and status of network
// ...
// PROFIT
