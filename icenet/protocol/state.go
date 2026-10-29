package protocol

import (
	"github.com/cerera/core/common"
	"github.com/cerera/icenet/service"
)

type Status struct {
	ChainID        int         `json:"chainId"`
	GenesisHash    common.Hash `json:"genesisHash"`
	StorageService string      `json:"storageService,omitempty"`
	StorageData    int         `json:"storageData,omitempty"` // Number of accounts
	ContractCodes  int         `json:"contractCodes,omitempty"`
	ContractSlots  int         `json:"contractSlots,omitempty"`
}

func NewStatus(chainID int, genesisHash common.Hash) *Status {
	return &Status{
		ChainID:     chainID,
		GenesisHash: genesisHash,
	}
}

// GetStatus builds a Status value from the provided ServiceProvider.
// If the provider is nil or genesis block is unavailable, fields fall back
// to their zero values.
func GetStatus(serviceProvider *service.ServiceProvider) (Status, error) {
	status := Status{}

	if serviceProvider != nil {
		status.ChainID = serviceProvider.GetChainID()

		// Storage fingerprint (service name / implementation type).
		status.StorageService = serviceProvider.GetStorageServiceName()

		if genesis := serviceProvider.GetBlockByHeight(0); genesis != nil {
			status.GenesisHash = genesis.Hash
		}

		// storage data (number of accounts)
		status.StorageData = serviceProvider.GetStorageSize()
		stats := serviceProvider.GetVaultSyncStats()
		status.ContractCodes = stats.ContractCodes
		status.ContractSlots = stats.ContractSlots
	}

	return status, nil
}
