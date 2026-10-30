package node

import (
	"context"
	"errors"
	"fmt"
	"sync"

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

var log = logger.Named("node")

// Node holds wired runtime components for a Cerera node process.
type Node struct {
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

// New creates and wires a Cerera node.
func New(cfg *config.Config, ctx context.Context, mode, port string, httpPort int, mine bool) (*Node, error) {
	registry, err := service.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to create service registry: %w", err)
	}
	ctx = service.WithRegistry(ctx, registry)

	if err := gigea.Init(ctx, cfg.NetCfg.ADDR); err != nil {
		return nil, fmt.Errorf("failed to init gigea: %w", err)
	}

	vault, err := storage.OpenNodeVault(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init vault: %w", err)
	}
	if err := registry.Register(&service.Service{
		Name:    vault.ServiceName(),
		Methods: vault.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register vault RPC service: %w", err)
	}

	bc, err := chain.Mold(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to mold blockchain parts: %w", err)
	}
	if err := registry.Register(&service.Service{
		Name:    bc.ServiceName(),
		Methods: bc.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register chain RPC service: %w", err)
	}

	txTable := storage.NewTxTable()

	validatorInst, err := validator.NewValidator(ctx, *cfg, vault, txTable)
	if err != nil {
		return nil, fmt.Errorf("failed to init validator: %w", err)
	}
	if err := registry.Register(&service.Service{
		Name:    validatorInst.ServiceName(),
		Methods: validatorInst.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register validator RPC service: %w", err)
	}

	mempool, err := pool.InitPool(cfg.POOL.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to init pool: %w", err)
	}
	if err := registry.Register(&service.Service{
		Name:    mempool.ServiceName(),
		Methods: mempool.Methods(),
	}); err != nil {
		return nil, fmt.Errorf("register pool RPC service: %w", err)
	}

	if err := network.SetUpHttp(ctx, cfg, httpPort); err != nil {
		return nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	ice, err := icenet.Start(cfg, ctx, port)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Ice: %w", err)
	}

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
	ice.SetVault(vault)

	bc.SetReorgHandler(validatorInst.ReplayChain)
	sp := ice.ServiceProviderPtr()
	if sp != nil {
		sp.SetChainRef(bc)
		sp.SetVault(vault)
	}

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
			log.Errorw("Failed to start miner", "err", err)
			return nil, fmt.Errorf("failed to start miner: %w", err)
		}
	}

	mempool.Register(minerInstance)

	return &Node{
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
func (n *Node) Shutdown(ctx context.Context) error {
	if n == nil {
		return nil
	}
	n.shutdownOnce.Do(func() {
		var errs []error

		if n.m != nil {
			log.Infow("Stopping miner...")
			n.m.Stop()
		}
		if n.p != nil {
			log.Infow("Stopping transaction pool...")
			n.p.Stop()
		}
		if n.ice != nil {
			log.Infow("Stopping Ice P2P network...")
			n.ice.Stop(ctx)
		}
		if n.v != nil {
			if err := n.v.Close(); err != nil {
				log.Errorw("Failed to close vault", "err", err)
				errs = append(errs, err)
			} else {
				log.Infow("Vault closed successfully")
			}
		}
		n.shutdownErr = errors.Join(errs...)
	})
	return n.shutdownErr
}
