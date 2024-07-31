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
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
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
	h      *network.Host
	p      *pool.Pool
	proc   Process
	v      storage.Vault
	status [8]byte
}

// todo run as daemin service
func main() {
	listenRpcPortParam := flag.Int("r", -1, "rpc port to listen")
	listenP2pPortParam := flag.Int("l", -1, "p2p port for connections")
	keyPathFlag := flag.String("key", "", "path to pem key")
	// logto := flag.String("logto", "stdout", "file path to log to, \"syslog\" or \"stdout\"")
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
	cfg.SetPorts(*listenRpcPortParam, *listenP2pPortParam)
	cfg.SetNodeKey(*keyPathFlag)
	cfg.SetAutoGen(true)

	ctx, _ := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)

	// ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// minwinsvc.SetOnExit(cancel)

	host := network.InitP2PHost(ctx, *cfg)
	// init rpc requests handling in
	host.SetUpHttp(ctx, *cfg)

	c := cerera{
		g:      validator.NewValidator(ctx, *cfg),
		bc:     chain.InitBlockChain(cfg), // chain use validator, init it before, not a clean way
		h:      host,
		p:      pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize),
		v:      storage.NewD5Vault(cfg),
		status: [8]byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0},
	}

	c.v.Prepare()

	// coinbase.SetCoinbase()

	s := gigea.Ring{
		Pool:       c.p,
		Chain:      &c.bc,
		Counter:    0,
		RoundTimer: time.NewTicker(time.Duration(3 * time.Second)),
	}

	for {
		if c.h.NetType == 0x2 {
			fmt.Printf("Try to sync accounts by network...\r\n")
			c.h.HandShake()
			c.status[0] = 0xb
		}
		if c.status[0] == 0xb || c.h.NetType == 0x1 {
			c.status[0] = 0xb
			break
		}
		time.Sleep(3 * time.Second)
	}

	c.g.SetUp(cfg.Chain.ChainID)

	go s.Execute()

	<-ctx.Done()
	_ = c.h.Stop()
	c.proc.Stop()
}
