package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	"github.com/cerera/internal/cerera/common"
)

type Input struct {
	*sync.RWMutex
	M map[common.Hash]*big.Int
}

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
	sa.Inputs.Lock()
	defer sa.Inputs.Unlock()
	sa.Inputs.M[txHash] = cnt
}

// ToBytes converts StateAccount to custom binary format
func (sa *StateAccount) Bytes() []byte {
	// add by order of length fields constant
	var buf bytes.Buffer
	fmt.Printf("Buffer before anything: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())

	// Write Address (assuming Address is []byte or has Bytes() method)
	addressBytes := sa.Address.Bytes()
	fmt.Printf("Add address to buffer: %x\n", addressBytes)
	binary.Write(&buf, binary.LittleEndian, uint32(len(addressBytes)))
	buf.Write(addressBytes)
	fmt.Printf("Buffer after address: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())

	// Write Passphrase
	passphraseBytes := sa.Passphrase.Bytes()
	buf.Write(passphraseBytes)
	fmt.Printf("Buffer after passphrase: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write MPub
	mpubBytes := sa.MPub[:]
	binary.Write(&buf, binary.LittleEndian, uint32(len(mpubBytes)))
	buf.Write(mpubBytes)
	fmt.Printf("Buffer after mpub: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write Bloom
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.Bloom)))
	buf.Write(sa.Bloom)
	fmt.Printf("Buffer after bloom: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write CodeHash
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.CodeHash)))
	buf.Write(sa.CodeHash)
	fmt.Printf("Buffer after code hash: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write Nonce
	binary.Write(&buf, binary.LittleEndian, sa.Nonce)
	fmt.Printf("Buffer after nonce: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write Root (assuming common.Hash has Bytes() method)
	rootBytes := sa.Root.Bytes()
	buf.Write(rootBytes)
	fmt.Printf("Buffer after root: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write Status
	statusBytes := sa.Status
	buf.WriteByte(statusBytes)
	fmt.Printf("Buffer after status: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())

	// Write balance as big.Int bytes
	balanceBytes := sa.balance.Bytes()
	binary.Write(&buf, binary.LittleEndian, uint32(len(balanceBytes)))
	buf.Write(balanceBytes)
	fmt.Printf("Buffer after balance: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	// Write Inputs map
	sa.Inputs.RLock()
	inputCount := uint32(len(sa.Inputs.M))
	sa.Inputs.RUnlock()
	binary.Write(&buf, binary.LittleEndian, inputCount)
	fmt.Printf("Buffer after input count: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())
	sa.Inputs.RLock()
	for hash, amount := range sa.Inputs.M {
		// Write hash
		hashBytes := hash.Bytes()
		buf.Write(hashBytes)
		// Write amount
		amountBytes := amount.Bytes()
		binary.Write(&buf, binary.LittleEndian, uint32(len(amountBytes)))
		buf.Write(amountBytes)
	}
	sa.Inputs.RUnlock()
	fmt.Printf("Buffer after inputs: %x\n", buf.Bytes())
	fmt.Printf("Buffer length: %d\n", buf.Len())

	return buf.Bytes()
}

// FromBytes creates StateAccount from custom binary format
func BytesToStateAccount(data []byte) *StateAccount {
	sa := &StateAccount{}
	buf := bytes.NewReader(data)

	// Read Address
	var addressLen uint32
	binary.Read(buf, binary.LittleEndian, &addressLen)
	addressBytes := make([]byte, addressLen)
	buf.Read(addressBytes)
	sa.Address = BytesToAddress(addressBytes)

	// Read Passphrase (32 bytes, no length prefix)
	passphraseBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	buf.Read(passphraseBytes)
	sa.Passphrase = common.Hash(passphraseBytes)

	// Read MPub
	var mpubLen uint32
	binary.Read(buf, binary.LittleEndian, &mpubLen)
	mpubBytes := make([]byte, mpubLen)
	buf.Read(mpubBytes)
	copy(sa.MPub[:], mpubBytes)

	// Read Bloom
	var bloomLen uint32
	binary.Read(buf, binary.LittleEndian, &bloomLen)
	sa.Bloom = make([]byte, bloomLen)
	buf.Read(sa.Bloom)

	// Read CodeHash
	var codeHashLen uint32
	binary.Read(buf, binary.LittleEndian, &codeHashLen)
	sa.CodeHash = make([]byte, codeHashLen)
	buf.Read(sa.CodeHash)

	// Read Nonce
	binary.Read(buf, binary.LittleEndian, &sa.Nonce)

	// Read Root (32 bytes, no length prefix)
	rootBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	buf.Read(rootBytes)
	sa.Root = common.Hash(rootBytes)

	// Read Status (1 byte, no length prefix)
	statusByte, _ := buf.ReadByte()
	sa.Status = statusByte

	// Read balance as big.Int bytes
	var balanceLen uint32
	binary.Read(buf, binary.LittleEndian, &balanceLen)
	balanceBytes := make([]byte, balanceLen)
	buf.Read(balanceBytes)
	sa.balance = new(big.Int).SetBytes(balanceBytes)

	// Read Inputs map
	var inputCount uint32
	binary.Read(buf, binary.LittleEndian, &inputCount)
	sa.Inputs = &Input{
		RWMutex: &sync.RWMutex{},
		M:       make(map[common.Hash]*big.Int),
	}

	for i := uint32(0); i < inputCount; i++ {
		// Read hash
		hashBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
		buf.Read(hashBytes)
		hash := common.Hash(hashBytes)

		// Read amount
		var amountLen uint32
		binary.Read(buf, binary.LittleEndian, &amountLen)
		amountBytes := make([]byte, amountLen)
		buf.Read(amountBytes)
		amount := new(big.Int).SetBytes(amountBytes)

		sa.Inputs.M[hash] = amount
	}

	return sa
}
