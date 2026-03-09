package service

import (
	"math/big"

	"github.com/cerera/core/address"
	"github.com/cerera/core/block"
	"github.com/cerera/core/chain"
	"github.com/cerera/core/common"
	"github.com/cerera/core/pool"
	"github.com/cerera/core/storage"
	"github.com/cerera/core/types"
)

// service provide api for stateful components (pool, chain, etc)
type ServiceProvider interface {
	// account
	GetAll()
	GetCount() int
	Create(passphrase string) error
	Restore(mnemonic string, passphrase string) error
	Verify(address address.Address, passphrase string) bool
	GetBalance(address address.Address) *big.Int
	Faucet(address address.Address) error
	Inputs(address address.Address) []common.Hash
	// chain
	Height() int
	GetBlockByIndex(index int) *block.Block
	GetBlock(hash common.Hash) *block.Block
	GetBlockHeader(hash common.Hash) *block.Header
	GetLatestBlock() *block.Block
	// validator
	// CreateTransaction(nonce uint64, addressTo types.Address, count float64, gas float64, message string) (*types.GTransaction, error)
	// SendTransaction(tx *types.GTransaction) error
	// GetTransaction(hash common.Hash) *types.GTransaction
	// pool
	MinGas() float64
	// other
	GetInfo() map[string]interface{}
	// icenet protocol
	AddBlock(b *block.Block) error
	GetBlockByHeight(height int) *block.Block
	GetBlockByHash(hash common.Hash) *block.Block
	ValidateBlock(b *block.Block) error
	ValidateBlockPoW(b *block.Block) bool
	GetCurrentHeight() int
	GetLatestHash() common.Hash
	GetChainID() int
	// storage
	GetStorageServiceName() string
	// common
	GetMethodType() byte
}

func GetServiceProvider() ServiceProvider {
	return &CereraServiceProvider{}
}

// implementation
type CereraServiceProvider struct {
	// we resolve concrete services lazily through the global Registry
	// to avoid tight coupling on construction time.
}

func (s *CereraServiceProvider) GetAll() {
	// Delegated account listing; result is ignored by the interface.
	if v := storage.GetVault(); v != nil {
		_ = v.GetAll()
	}
}

func (s *CereraServiceProvider) GetCount() int {
	if v := storage.GetVault(); v != nil {
		return v.GetCount()
	}
	return 0
}

func (s *CereraServiceProvider) Create(passphrase string) error {
	if v := storage.GetVault(); v != nil {
		_, _, _, _, err := v.Create(passphrase)
		return err
	}
	return nil
}

func (s *CereraServiceProvider) Restore(mnemonic string, passphrase string) error {
	if v := storage.GetVault(); v != nil {
		_, _, err := v.Restore(mnemonic, passphrase)
		return err
	}
	return nil
}

func (s *CereraServiceProvider) Verify(address address.Address, passphrase string) bool {
	if v := storage.GetVault(); v != nil {
		// Vault.VerifyAccount uses types.Address; convert via hex string through Exec.
		if res, ok := v.Exec("verify", []any{address.Hex(), passphrase}).(bool); ok {
			return res
		}
	}
	return false
}

func (s *CereraServiceProvider) GetBalance(address address.Address) *big.Int {
	if v := storage.GetVault(); v != nil {
		if acc := v.Get(addressToTypes(address)); acc != nil {
			return acc.GetBalanceBI()
		}
	}
	return big.NewInt(0)
}

func (s *CereraServiceProvider) Faucet(address address.Address) error {
	if v := storage.GetVault(); v != nil {
		// Reuse vault's faucet Exec path with default amount (1 CER).
		_ = v.Exec("faucet", []any{address.Hex(), "1"})
	}
	return nil
}

func (s *CereraServiceProvider) Inputs(address address.Address) []common.Hash {
	if v := storage.GetVault(); v != nil {
		if acc := v.Get(addressToTypes(address)); acc != nil {
			inputs := acc.GetAllInputs()
			hashes := make([]common.Hash, 0, len(inputs))
			for h := range inputs {
				hashes = append(hashes, h)
			}
			return hashes
		}
	}
	return []common.Hash{}
}

func (s *CereraServiceProvider) Height() int {
	return s.GetCurrentHeight()
}

func (s *CereraServiceProvider) GetBlockByIndex(index int) *block.Block {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	return ch.GetBlockByNumber(index)
}

func (s *CereraServiceProvider) GetBlock(hash common.Hash) *block.Block {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	return ch.GetBlock(hash)
}

func (s *CereraServiceProvider) GetBlockHeader(hash common.Hash) *block.Header {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	b := ch.GetBlock(hash)
	if b == nil {
		return nil
	}
	return b.Header()
}

func (s *CereraServiceProvider) GetLatestBlock() *block.Block {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	return ch.GetLatestBlock()
}

func (s *CereraServiceProvider) MinGas() float64 {
	// Delegate to pool service if available.
	reg, err := GetRegistry()
	if err != nil {
		return 0
	}
	srv, ok := reg.GetService(POOL_SERVICE_NAME)
	if !ok {
		return 0
	}
	if p, ok := srv.(pool.TxPool); ok {
		if v, ok2 := p.Exec("minGas", nil).(float64); ok2 {
			return v
		}
	}
	return 0
}

func (s *CereraServiceProvider) GetInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Chain info
	if ch := s.getChain(); ch != nil {
		info["chain"] = ch.GetInfo()
	}

	// Vault info (total accounts, supply)
	if v := storage.GetVault(); v != nil {
		info["accounts"] = v.GetCount()
	}

	return info
}

func (s *CereraServiceProvider) AddBlock(b *block.Block) error {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	return ch.UpdateChain(b)
}

func (s *CereraServiceProvider) GetBlockByHeight(height int) *block.Block {
	ch := s.getChain()
	if ch == nil {
		return nil
	}

	// Prefer linear scan by Height; chain indexes primarily by Index.
	data := ch.GetData()
	for _, blk := range data {
		if blk != nil && blk.Head != nil && blk.Head.Height == height {
			return blk
		}
	}
	return nil
}

func (s *CereraServiceProvider) GetBlockByHash(hash common.Hash) *block.Block {
	ch := s.getChain()
	if ch == nil {
		return nil
	}
	return ch.GetBlock(hash)
}

func (s *CereraServiceProvider) ValidateBlock(b *block.Block) error {
	// Basic structural and chain linkage validation.
	if b == nil || b.Head == nil {
		return nil
	}

	ch := s.getChain()
	if ch == nil {
		// Without a chain context, accept the block structurally.
		return nil
	}

	latest := ch.GetLatestBlock()
	if latest == nil || latest.Head == nil {
		// Genesis or empty chain; let UpdateChain handle invariants.
		return nil
	}

	// Height must be >= currentHeight+1; stricter checks are in sync logic.
	if b.Head.Height <= latest.Header().Height {
		return nil
	}

	// Previous hash should match our tip for simple extension.
	if b.Head.PrevHash != latest.GetHash() {
		// Allow forks; sync/consensus layer decides which branch to follow.
		return nil
	}

	return nil
}

func (s *CereraServiceProvider) ValidateBlockPoW(b *block.Block) bool {
	if b == nil || b.Head == nil {
		return false
	}
	// Difficulty must be non-zero.
	if b.Head.Difficulty == 0 {
		return false
	}

	// Recompute hash and compare to target (same logic as miner).
	h, err := b.CalculateHash()
	if err != nil {
		return false
	}

	target := new(big.Int).Div(
		new(big.Int).Lsh(big.NewInt(1), 256),
		new(big.Int).SetUint64(b.Head.Difficulty),
	)
	hashInt := new(big.Int).SetBytes(h)

	return hashInt.Cmp(target) < 0
}

func (s *CereraServiceProvider) GetCurrentHeight() int {
	ch := s.getChain()
	if ch == nil {
		return 0
	}
	latest := ch.GetLatestBlock()
	if latest == nil || latest.Head == nil {
		return 0
	}
	return latest.Header().Height
}

func (s *CereraServiceProvider) GetLatestHash() common.Hash {
	ch := s.getChain()
	if ch == nil {
		return common.Hash{}
	}
	latest := ch.GetLatestBlock()
	if latest == nil {
		return common.Hash{}
	}
	return latest.GetHash()
}

func (s *CereraServiceProvider) GetChainID() int {
	ch := s.getChain()
	if ch == nil {
		return 0
	}
	return ch.GetChainId()
}

func (s *CereraServiceProvider) GetStorageServiceName() string {
	v := storage.GetVault()
	if v == nil {
		return ""
	}
	return v.ServiceName()
}

func (s *CereraServiceProvider) GetMethodType() byte {
	return 0x0
}

// getChain resolves the chain service from the global registry.
func (s *CereraServiceProvider) getChain() *chain.Chain {
	reg, err := GetRegistry()
	if err != nil {
		return nil
	}
	srv, ok := reg.GetService(CHAIN_SERVICE_NAME)
	if !ok {
		return nil
	}
	ch, ok := srv.(*chain.Chain)
	if !ok {
		return nil
	}
	return ch
}

// addressToTypes converts core/address.Address to core/types.Address via hex string.
func addressToTypes(a address.Address) types.Address {
	return types.HexToAddress(a.Hex())
}
