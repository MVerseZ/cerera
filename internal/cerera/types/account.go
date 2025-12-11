package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"sync"

	"github.com/cerera/internal/cerera/common"
)

const BaseAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000f"
const FaucetAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000a"
const CoreStakingAddressHex = "0xf00000000000000000000000000000000000000000000000000000000000000b"

type Input struct {
	*sync.RWMutex
	M map[common.Hash]*big.Int
}

const DEBUG = false

type StateAccount struct {
	Address    Address
	balance    *big.Int `json:"-"` // не сериализуем balance в JSON
	Bloom      []byte
	CodeHash   []byte
	Nonce      uint64
	Root       common.Hash // merkle root of the storage trie
	Inputs     *Input      `json:"-"` // не сериализуем Inputs в JSON из-за mutex
	Status     byte        // 0: OP_ACC_NEW, 1: OP_ACC_STAKE, 2: OP_ACC_F, 3: OP_ACC_NODE, 4: VOID
	Type       byte        // 0: normal account, 1: staking account, 2: voting account, 3: faucet account, 4: coinbase account
	Passphrase common.Hash // hash of password
	// bip32 data
	MPub [78]byte
	// MPriv    *bip32.Key
}

// TODO
func NewStateAccount(address Address, balance float64, root common.Hash) *StateAccount {
	return &StateAccount{
		Address: address,
		Nonce:   1,
		balance: FloatToBigInt(balance),
		Root:    root,
		Bloom:   []byte{0xf, 0xf, 0xf, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Status:  0,
		Type:    0,
		Inputs: &Input{
			RWMutex: &sync.RWMutex{},
			M:       make(map[common.Hash]*big.Int),
		},
	}
}

func (sa *StateAccount) GetBalance() float64 {
	return BigIntToFloat(sa.balance)
}

func (sa *StateAccount) SetBalance(balance float64) {
	sa.balance = FloatToBigInt(balance)
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
	// Write MPub
	mpubBytes := sa.MPub[:]
	binary.Write(&buf, binary.LittleEndian, uint32(len(mpubBytes)))
	buf.Write(mpubBytes)
	// fmt.Printf("Buffer after mpub: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after mpub: %d\n", buf.Len())
	}
	// Write Bloom
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.Bloom)))
	buf.Write(sa.Bloom)
	// fmt.Printf("Buffer after bloom: %x\n", buf.Bytes())
	if DEBUG {
		fmt.Printf("Buffer length after bloom: %d\n", buf.Len())
	}
	// Write CodeHash
	if sa.Address == HexToAddress(BaseAddressHex) || sa.Address == HexToAddress(FaucetAddressHex) || sa.Address == HexToAddress(CoreStakingAddressHex) {
		zeroBuf := make([]byte, 4)
		buf.Write(zeroBuf)
	} else {
		binary.Write(&buf, binary.LittleEndian, uint32(len(sa.CodeHash)))
		buf.Write(sa.CodeHash)
	}
	// fmt.Printf("Buffer after code hash: %x\n", buf.Bytes())
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

	// Write Inputs map
	sa.Inputs.RLock()
	inputsCount := uint32(len(sa.Inputs.M))
	binary.Write(&buf, binary.LittleEndian, inputsCount)

	// Записываем каждую пару (hash, value)
	for txHash, val := range sa.Inputs.M {
		// Записываем hash (32 bytes)
		hashBytes := txHash.Bytes()
		buf.Write(hashBytes)

		// Записываем значение big.Int
		valBytes := val.Bytes()
		binary.Write(&buf, binary.LittleEndian, uint32(len(valBytes)))
		if len(valBytes) > 0 {
			buf.Write(valBytes)
		}
	}
	sa.Inputs.RUnlock()

	if DEBUG {
		fmt.Printf("Buffer length after inputs (%d entries): %d\n", inputsCount, buf.Len())
	}

	// Check if there's any '\n' byte in the buffer (typically end of serialized account line in file)
	// if bytes.Contains(buf.Bytes(), []byte{'\n'}) {
	// 	if DEBUG {
	// 		fmt.Printf("Warning: Buffer contains newline (\\n) byte!\n")
	// 	}
	// }

	return buf.Bytes()
}

// FromBytes creates StateAccount from custom binary format
func BytesToStateAccount(data []byte) *StateAccount {
	sa := &StateAccount{}
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
	sa.Address = BytesToAddress(addressBytes)

	// Read Passphrase (32 bytes, no length prefix)
	passphraseBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	if _, err := io.ReadFull(buf, passphraseBytes); err != nil {
		return nil
	}
	sa.Passphrase = common.Hash(passphraseBytes)

	// Read MPub
	var mpubLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &mpubLen); err != nil {
		return nil
	}
	mpubBytes := make([]byte, mpubLen)
	if mpubLen > 0 {
		if _, err := io.ReadFull(buf, mpubBytes); err != nil {
			return nil
		}
	}
	copy(sa.MPub[:], mpubBytes)

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
	var codeHashLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &codeHashLen); err != nil {
		return nil
	}
	if codeHashLen > 0 {
		sa.CodeHash = make([]byte, codeHashLen)
		if _, err := io.ReadFull(buf, sa.CodeHash); err != nil {
			return nil
		}
	} else {
		sa.CodeHash = make([]byte, 0)
	}

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
	sa.balance = new(big.Int).SetBytes(balanceBytes)

	// Initialize Inputs (not serialized, but must be initialized to avoid nil pointer)
	sa.Inputs = &Input{
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
