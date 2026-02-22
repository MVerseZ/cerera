package service

import (
	"math/big"

	"github.com/cerera/core/address"
	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
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
	// common
	GetMethodType() byte
}

func GetServiceProvider() ServiceProvider {
	return &CereraServiceProvider{}
}

// implementation
type CereraServiceProvider struct {
}

func (s *CereraServiceProvider) GetAll() {
}

func (s *CereraServiceProvider) GetCount() int {
	return 0
}

func (s *CereraServiceProvider) Create(passphrase string) error {
	return nil
}

func (s *CereraServiceProvider) Restore(mnemonic string, passphrase string) error {
	return nil
}

func (s *CereraServiceProvider) Verify(address address.Address, passphrase string) bool {
	return false
}

func (s *CereraServiceProvider) GetBalance(address address.Address) *big.Int {
	return big.NewInt(0)
}

func (s *CereraServiceProvider) Faucet(address address.Address) error {
	return nil
}

func (s *CereraServiceProvider) Inputs(address address.Address) []common.Hash {
	return []common.Hash{}
}

func (s *CereraServiceProvider) Height() int {
	return 0
}

func (s *CereraServiceProvider) GetBlockByIndex(index int) *block.Block {
	return nil
}

func (s *CereraServiceProvider) GetBlock(hash common.Hash) *block.Block {
	return nil
}

func (s *CereraServiceProvider) GetBlockHeader(hash common.Hash) *block.Header {
	return nil
}

func (s *CereraServiceProvider) GetLatestBlock() *block.Block {
	return nil
}

func (s *CereraServiceProvider) MinGas() float64 {
	return 0
}

func (s *CereraServiceProvider) GetInfo() map[string]interface{} {
	return map[string]interface{}{}
}

func (s *CereraServiceProvider) AddBlock(b *block.Block) error {
	return nil
}

func (s *CereraServiceProvider) GetBlockByHeight(height int) *block.Block {
	return nil
}

func (s *CereraServiceProvider) GetBlockByHash(hash common.Hash) *block.Block {
	return nil
}

func (s *CereraServiceProvider) ValidateBlock(b *block.Block) error {
	return nil
}

func (s *CereraServiceProvider) ValidateBlockPoW(b *block.Block) bool {
	return false
}

func (s *CereraServiceProvider) GetCurrentHeight() int {
	return 0
}

func (s *CereraServiceProvider) GetLatestHash() common.Hash {
	return common.Hash{}
}

func (s *CereraServiceProvider) GetChainID() int {
	return 0
}

func (s *CereraServiceProvider) GetMethodType() byte {
	return 0x0
}
