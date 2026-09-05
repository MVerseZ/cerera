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
	Root    common.Hash // merkle root of the storage trie
	KeyHash common.Hash // hash of the public key
	Data    []byte      // data of the account
}

type StateAccount struct {
	StateAccountData
	Bloom      []byte
	Status     byte        // 0: OP_ACC_NEW, 1: OP_ACC_STAKE, 2: OP_ACC_F, 3: OP_ACC_NODE, 4: VOID
	Type       byte        // 0: normal account, 1: staking account, 2: voting account, 3: faucet account, 4: coinbase account
	Passphrase common.Hash // hash of password
	// non serialized fields
	balance     *big.Int `json:"-"` // не сериализуем balance в JSON
	Inputs      *Input   `json:"-"` // не сериализуем Inputs в JSON из-за mutex
	InputsCount uint32   `json:"-"` // count of inputs
}

// TODO
func NewStateAccount(address address.Address, balance float64, root common.Hash) *StateAccount {
	return &StateAccount{
		StateAccountData: StateAccountData{
			Address: address,
			Nonce:   1,
			Root:    root,
		},
		balance: common.FloatToBigInt(balance),
		Bloom:   []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Status:  0,
		Type:    0,
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
		InputsCount: 0,
	}
}

func (sa *StateAccount) GetBalance() float64 {
	return common.BigIntToFloat(sa.balance)
}

func (sa *StateAccount) SetBalance(balance float64) {
	sa.balance = common.FloatToBigInt(balance)
}

// GetBalanceBI returns a copy of the current balance as big.Int.
func (sa *StateAccount) GetBalanceBI() *big.Int {
	if sa.balance == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(sa.balance)
}

// SetBalanceBI sets the balance using big.Int value (copying the input).
func (sa *StateAccount) SetBalanceBI(v *big.Int) {
	if v == nil {
		sa.balance = big.NewInt(0)
		return
	}
	sa.balance = new(big.Int).Set(v)
}

func (sa *StateAccount) BloomUp() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] < 0xf {
		sa.Bloom[1] = tmpBloom + 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) BloomDown() {
	var tmpBloom = sa.Bloom[1]
	if sa.Bloom[1] > 0x1 {
		sa.Bloom[1] = tmpBloom - 0x1
	} else {
		sa.Bloom[2] = 0xf
	}
}

func (sa *StateAccount) AddInput(txHash common.Hash, cnt *big.Int) {
	if sa.Inputs == nil {
		sa.Inputs = &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		}
	}
	sa.Inputs.Lock()
	defer sa.Inputs.Unlock()
	// Store a copy of cnt to avoid external modifications
	sa.InputsCount += 1
	if cnt != nil {
		sa.Inputs.M[txHash] = new(big.Int).Set(cnt)
	} else {
		sa.Inputs.M[txHash] = big.NewInt(0)
	}
}

// GetAllInputs возвращает копию всех инпутов (без mutex) для безопасного использования
func (sa *StateAccount) GetAllInputs() map[common.Hash]*big.Int {
	if sa.Inputs == nil {
		return make(map[common.Hash]*big.Int)
	}
	sa.Inputs.RLock()
	defer sa.Inputs.RUnlock()

	// Создаем копию map и значений
	result := make(map[common.Hash]*big.Int, len(sa.Inputs.M))
	for hash, val := range sa.Inputs.M {
		result[hash] = new(big.Int).Set(val)
	}
	return result
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

	// Write Passphrase
	passphraseBytes := sa.Passphrase.Bytes()
	buf.Write(passphraseBytes)
	// fmt.Printf("Buffer after passphrase: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after passphrase: %d\n", buf.Len())
	}

	// Write Bloom
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.Bloom)))
	buf.Write(sa.Bloom)
	// fmt.Printf("Buffer after bloom: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after bloom: %d\n", buf.Len())
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
	// Write Root (assuming common.Hash has Bytes() method)
	rootBytes := sa.Root.Bytes()
	buf.Write(rootBytes)
	// fmt.Printf("Buffer after root: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after root: %d\n", buf.Len())
	}
	// Write Status
	statusBytes := sa.Status
	buf.WriteByte(statusBytes)
	if DEBUG {
		fmt.Printf("Buffer length after status: %d\n", buf.Len())
	}

	// Write balance as big.Int bytes
	balanceBytes := sa.balance.Bytes()
	binary.Write(&buf, binary.LittleEndian, uint32(len(balanceBytes)))
	buf.Write(balanceBytes)
	// fmt.Printf("Buffer after balance: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after balance: %d\n", buf.Len())
	}

	// Inputs are not persisted; rebuilt from chain state. Always zero entries.
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	writeWalletExtension(&buf, sa)

	if DEBUG {
		fmt.Printf("Buffer length after inputs (0 entries, chain-derived): %d\n", buf.Len())
	}

	return buf.Bytes()
}

func writeWalletExtension(buf *bytes.Buffer, sa *StateAccount) {
	_ = binary.Write(buf, binary.LittleEndian, walletKeysMagic)
	keyHash := sa.KeyHash
	buf.Write(keyHash.Bytes())
	data := sa.Data
	if data == nil {
		data = []byte{}
	}
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(data)))
	if len(data) > 0 {
		buf.Write(data)
	}
}

func readWalletExtension(sa *StateAccount, buf *bytes.Reader) error {
	if buf.Len() < 4+32+4 {
		return fmt.Errorf("missing wallet keys trailer")
	}
	var magic uint32
	if err := binary.Read(buf, binary.LittleEndian, &magic); err != nil {
		return err
	}
	if magic != walletKeysMagic {
		return fmt.Errorf("invalid wallet keys magic: %x", magic)
	}
	keyHashBytes := make([]byte, 32)
	if _, err := io.ReadFull(buf, keyHashBytes); err != nil {
		return err
	}
	sa.KeyHash = common.Hash(keyHashBytes)

	var dataLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
		return err
	}
	if dataLen > maxWalletDataLen {
		return fmt.Errorf("wallet data too large: %d", dataLen)
	}
	if dataLen == 0 {
		sa.Data = nil
		return nil
	}
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(buf, data); err != nil {
		return err
	}
	sa.Data = data
	if buf.Len() != 0 {
		return fmt.Errorf("trailing account bytes: %d", buf.Len())
	}
	return nil
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

	passphraseBytes := make([]byte, 32)
	if _, err := io.ReadFull(buf, passphraseBytes); err != nil {
		return nil
	}
	sa.Passphrase = common.Hash(passphraseBytes)

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

	if err := binary.Read(buf, binary.LittleEndian, &sa.Nonce); err != nil {
		return nil
	}
	rootBytes := make([]byte, 32)
	if _, err := io.ReadFull(buf, rootBytes); err != nil {
		return nil
	}
	sa.Root = common.Hash(rootBytes)

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

	sa.Inputs = &Input{
		RWMutex: &sync.RWMutex{},
		M:       make(map[common.Hash]*big.Int),
	}

	var inputsCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &inputsCount); err != nil {
		return nil
	}
	if inputsCount != 0 {
		return nil
	}

	if err := readWalletExtension(sa, buf); err != nil {
		return nil
	}

	return sa
}
