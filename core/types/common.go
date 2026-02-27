package types

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/big"
	"sync"

	"github.com/cerera/core/account"
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
func FloatToBigInt(f float64) *big.Int              { return common.FloatToBigInt(f) }
func BigIntToFloat(bi *big.Int) float64             { return common.BigIntToFloat(bi) }
func DecimalStringToWei(s string) (*big.Int, error) { return common.DecimalStringToWei(s) }

// Re-exports from crypto package
var (
	GenerateAccount            = crypto.GenerateAccount
	GenerateMasterKey          = crypto.GenerateMasterKey
	PubkeyToAddress            = crypto.PubkeyToAddress
	EncodePrivateKeyToByte     = crypto.EncodePrivateKeyToByte
	EncodePrivateKeyToToString = crypto.EncodePrivateKeyToToString
	DecodePrivKey              = crypto.DecodePrivKey
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

// FromBytes creates StateAccount from custom binary format
func BytesToStateAccount(data []byte) *account.StateAccount {
	sa := &account.StateAccount{}
	buf := bytes.NewReader(data)

	// Read Type (added to serialization in newer versions)
	firstByte, err := buf.ReadByte()
	if err != nil {
		return nil
	}
	if firstByte <= 4 {
		sa.Type = firstByte
	} else {
		// Older serialized accounts did not include Type,
		// rewind and treat the byte as part of the address length.
		sa.Type = 0
		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			return nil
		}
	}

	// Read Address
	var addressLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &addressLen); err != nil {
		return nil
	}
	addressBytes := make([]byte, addressLen)
	if addressLen > 0 {
		if _, err := io.ReadFull(buf, addressBytes); err != nil {
			return nil
		}
	}
	sa.Address = address.BytesToAddress(addressBytes)

	// Read Passphrase (32 bytes, no length prefix)
	passphraseBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	if _, err := io.ReadFull(buf, passphraseBytes); err != nil {
		return nil
	}
	sa.Passphrase = common.Hash(passphraseBytes)

	// Read MPub
	// var mpubLen uint32
	// if err := binary.Read(buf, binary.LittleEndian, &mpubLen); err != nil {
	// 	return nil
	// }
	// mpubBytes := make([]byte, mpubLen)
	// if mpubLen > 0 {
	// 	if _, err := io.ReadFull(buf, mpubBytes); err != nil {
	// 		return nil
	// 	}
	// }
	// copy(sa.MPub[:], mpubBytes)

	// Read Bloom
	var bloomLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &bloomLen); err != nil {
		return nil
	}
	sa.Bloom = make([]byte, bloomLen)
	if bloomLen > 0 {
		if _, err := io.ReadFull(buf, sa.Bloom); err != nil {
			return nil
		}
	}

	// Read CodeHash
	// var codeHashLen uint32
	// if err := binary.Read(buf, binary.LittleEndian, &codeHashLen); err != nil {
	// 	return nil
	// }
	// if codeHashLen > 0 {
	// 	sa.CodeHash = make([]byte, codeHashLen)
	// 	if _, err := io.ReadFull(buf, sa.CodeHash); err != nil {
	// 		return nil
	// 	}
	// } else {
	// 	sa.CodeHash = make([]byte, 0)
	// }

	// Read Nonce
	if err := binary.Read(buf, binary.LittleEndian, &sa.Nonce); err != nil {
		return nil
	}

	// Read Root (32 bytes, no length prefix)
	rootBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	if _, err := io.ReadFull(buf, rootBytes); err != nil {
		return nil
	}
	sa.Root = common.Hash(rootBytes)

	// Read Status (1 byte, no length prefix)
	statusByte, err := buf.ReadByte()
	if err != nil {
		return nil
	}
	sa.Status = statusByte

	// Read balance as big.Int bytes
	var balanceLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &balanceLen); err != nil {
		return nil
	}
	balanceBytes := make([]byte, balanceLen)
	if balanceLen > 0 {
		if _, err := io.ReadFull(buf, balanceBytes); err != nil {
			return nil
		}
	}
	sa.SetBalanceBI(new(big.Int).SetBytes(balanceBytes))

	// Initialize Inputs (not serialized, but must be initialized to avoid nil pointer)
	sa.Inputs = &account.Input{
		RWMutex: &sync.RWMutex{},
		M:       make(map[common.Hash]*big.Int),
	}

	// Read Inputs map
	var inputsCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &inputsCount); err != nil {
		// Если не удалось прочитать (старая версия без инпутов), просто возвращаем
		// Inputs уже инициализирован пустым
		return sa
	}

	// Читаем каждую пару (hash, value)
	sa.Inputs.Lock()
	for i := uint32(0); i < inputsCount; i++ {
		// Читаем hash (32 bytes)
		hashBytes := make([]byte, 32)
		if _, err := io.ReadFull(buf, hashBytes); err != nil {
			sa.Inputs.Unlock()
			return nil
		}
		txHash := common.Hash(hashBytes)

		// Читаем значение big.Int
		var valLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &valLen); err != nil {
			sa.Inputs.Unlock()
			return nil
		}
		valBytes := make([]byte, valLen)
		if valLen > 0 {
			if _, err := io.ReadFull(buf, valBytes); err != nil {
				sa.Inputs.Unlock()
				return nil
			}
		}
		val := new(big.Int).SetBytes(valBytes)
		sa.Inputs.M[txHash] = val
	}
	sa.Inputs.Unlock()

	return sa
}
