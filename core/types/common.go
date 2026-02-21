package types

import (
	"math/big"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
	"github.com/cerera/core/crypto"
)

// Address is an alias for address.Address (moved from types to address package)
type Address = address.Address

// Re-exports from address package
var (
	HexToAddress   = address.HexToAddress
	BytesToAddress = address.BytesToAddress
	EmptyAddress   = address.EmptyAddress
)

// Re-exports from common package
var (
	EmptyCodeRootHash = common.EmptyCodeRootHash
)

// Re-exports from common package (functions)
func BigIntToFloat(bi *big.Int) float64 { return common.BigIntToFloat(bi) }
func DecimalStringToWei(s string) (*big.Int, error) { return common.DecimalStringToWei(s) }

// Re-exports from crypto package
var (
	GenerateAccount          = crypto.GenerateAccount
	PubkeyToAddress          = crypto.PubkeyToAddress
	EncodePrivateKeyToByte   = crypto.EncodePrivateKeyToByte
	EncodePrivateKeyToToString = crypto.EncodePrivateKeyToToString
	DecodePrivKey            = crypto.DecodePrivKey
)

// Event and system events (used by tests and observers)
type Event byte

const (
	EVENT_ADD    Event = 0x1
	EVENT_REMOVE Event = 0x2
	EVENT_STATUS Event = 0x3
)

func InitSystemEvents() *[]Event {
	return &[]Event{EVENT_ADD, EVENT_REMOVE, EVENT_STATUS}
}
