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
	"github.com/cerera/internal/cerera/network"
	"github.com/cerera/internal/cerera/pool"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/validator"
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

func main() {
	listenRpcPortParam := flag.Int("r", -1, "rpc port to listen")
	listenP2pPortParam := flag.Int("l", -1, "p2p port for connections")
	keyPathFlag := flag.String("key", "", "path to pem key")
	// logto := flag.String("logto", "stdout", "file path to log to, \"syslog\" or \"stdout\"")
	flag.Parse()

	cfg := config.GenerageConfig()
	cfg.SetPorts(*listenRpcPortParam, *listenP2pPortParam)
	cfg.SetNodeKey(*keyPathFlag)

	ctx, _ := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)

	// ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// minwinsvc.SetOnExit(cancel)

	host := network.InitP2PHost(ctx, *cfg)
	host.SetUpHttp(ctx, *cfg)

	c := cerera{
		bc:     chain.InitBlockChain(cfg),
		g:      validator.NewValidator(ctx, *cfg),
		h:      host,
		p:      pool.InitPool(cfg.POOL.MinGas, cfg.POOL.MaxSize),
		v:      storage.NewD5Vault(&cfg.NetCfg),
		status: [8]byte{0xf, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0},
	}

	for {
		if c.h.NetType == 0x2 {
			fmt.Printf("Sync accounts...\r\n")
			c.h.HandShake()
			// c.status[0] = 0xb
		}
		if c.status[0] == 0xb || c.h.NetType == 0x1 {
			c.status[0] = 0xb
			break
		}
		time.Sleep(3 * time.Second)
	}

	c.g.SetUp(cfg.Chain.ChainID)

	<-ctx.Done()
	_ = c.h.Stop()
	c.proc.Stop()
}
