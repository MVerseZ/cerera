package account

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"sync"

	"github.com/cerera/core/address"
	"github.com/cerera/core/common"
)

const BaseAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000f"
const FaucetAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000a"
const CoreStakingAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000b"

type Input struct {
	*sync.RWMutex
	M map[common.Hash]*big.Int
}

const DEBUG = false

// walletKeysMagic marks KeyHash/Data trailer (WLK1 LE). Required in every record.
const walletKeysMagic uint32 = 0x314B4C57

const maxWalletDataLen = 4096

type StateAccountData struct {
	Address address.Address
	Nonce   uint64
	Balance *big.Int
}

type StateAccount struct {
	StateAccountData
	Status byte // 0: OP_ACC_NEW, 1: OP_ACC_STAKE, 2: OP_ACC_F, 3: OP_ACC_NODE, 4: VOID
	Type   byte // 0: normal account, 1: staking account, 2: voting account, 3: faucet account, 4: coinbase account
}

// TODO
func NewStateAccount(address address.Address, balance float64, root common.Hash) *StateAccount {
	return &StateAccount{
		StateAccountData: StateAccountData{
			Address: address,
			Nonce:   1,
			Balance: big.NewInt(0),
		},
		Status: 0,
		Type:   0,
	}
}

func (sa *StateAccount) GetBalance() float64 {
	return common.BigIntToFloat(sa.Balance)
}

func (sa *StateAccount) SetBalance(balance float64) {
	sa.Balance = common.FloatToBigInt(balance)
}

// GetBalanceBI returns a copy of the current balance as big.Int.
func (sa *StateAccount) GetBalanceBI() *big.Int {
	if sa.Balance == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(sa.Balance)
}

// SetBalanceBI sets the balance using big.Int value (copying the input).
func (sa *StateAccount) SetBalanceBI(v *big.Int) {
	if v == nil {
		sa.Balance = big.NewInt(0)
		return
	}
	sa.Balance = new(big.Int).Set(v)
}

// ToBytes converts StateAccount to custom binary format
func (sa *StateAccount) Bytes() []byte {
	// add by order of length fields constant
	var buf bytes.Buffer
	if DEBUG {
		fmt.Printf("Buffer length before: %d\n", buf.Len())
	}

	buf.WriteByte(sa.Type)
	if DEBUG {
		fmt.Printf("Buffer length after type: %d\n", buf.Len())
	}

	// Write Address (assuming Address is []byte or has Bytes() method)
	addressBytes := sa.Address.Bytes()
	// fmt.Printf("Add address to buffer: %x\n", addressBytes)
	binary.Write(&buf, binary.LittleEndian, uint32(len(addressBytes)))
	buf.Write(addressBytes)
	if DEBUG {
		fmt.Printf("Buffer length after address: %s %d\n", sa.Address.Hex(), buf.Len())
	}

	if DEBUG {
		fmt.Printf("Buffer length after code hash: %d\n", buf.Len())
	}
	// Write Nonce
	binary.Write(&buf, binary.LittleEndian, sa.Nonce)
	// fmt.Printf("Buffer after nonce: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after nonce: %d\n", buf.Len())
	}

	// Write Status
	statusBytes := sa.Status
	buf.WriteByte(statusBytes)
	if DEBUG {
		fmt.Printf("Buffer length after status: %d\n", buf.Len())
	}

	// Write balance as big.Int bytes
	balanceBytes := sa.Balance.Bytes()
	binary.Write(&buf, binary.LittleEndian, uint32(len(balanceBytes)))
	buf.Write(balanceBytes)
	// fmt.Printf("Buffer after balance: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after balance: %d\n", buf.Len())
	}

	// Inputs are not persisted; rebuilt from chain state. Always zero entries.
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	if DEBUG {
		fmt.Printf("Buffer length after inputs (0 entries, chain-derived): %d\n", buf.Len())
	}

	return buf.Bytes()
}

// ValidSerialized reports whether data is a complete current-format account blob.
func ValidSerialized(data []byte) bool {
	return FromBytes(data) != nil
}

// FromBytes creates StateAccount from custom binary format (same as types.BytesToStateAccount).
func FromBytes(data []byte) *StateAccount {
	sa := &StateAccount{}
	buf := bytes.NewReader(data)

	firstByte, err := buf.ReadByte()
	if err != nil {
		return nil
	}
	if firstByte > 4 {
		return nil
	}
	sa.Type = firstByte

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

	if err := binary.Read(buf, binary.LittleEndian, &sa.Nonce); err != nil {
		return nil
	}

	statusByte, err := buf.ReadByte()
	if err != nil {
		return nil
	}
	sa.Status = statusByte

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

	return sa
}
