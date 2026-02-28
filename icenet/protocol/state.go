package protocol

import (
	"github.com/cerera/core/common"
	"github.com/cerera/internal/service"
)

type Status struct {
	ChainID     int         `json:"chainId"`
	GenesisHash common.Hash `json:"genesisHash"`
}

func NewStatus(chainID int, genesisHash common.Hash) *Status {
	return &Status{
		ChainID:     chainID,
		GenesisHash: genesisHash,
	}
}

func GetStatus(serviceProvider service.ServiceProvider) (Status, error) {
	// TODO: Implement this
	// return Status{
	// 	ChainID:     serviceProvider.GetChainID(),
	// 	GenesisHash: serviceProvider.GetBlockByHeight(0).Hash,
	// }, nil

	// mock data
	return Status{
		ChainID:     1,
		GenesisHash: common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"),
	}, nil
}
