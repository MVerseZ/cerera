package types

import (
	"bytes"
	"encoding/binary"
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
	Status     string
	Passphrase common.Hash // hash of password
	// bip32 data
	MPub string
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
		Status:  "OP_ACC_NEW",
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

// func (sa *StateAccount) Bytes() []byte {

// 	buf, err := json.Marshal(sa)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return buf
// }

func (sa *StateAccount) AddInput(txHash common.Hash, cnt *big.Int) {
	sa.Inputs.Lock()
	defer sa.Inputs.Unlock()
	sa.Inputs.M[txHash] = cnt
}

// func BytesToStateAccount(data []byte) *StateAccount {
// 	sa := &StateAccount{}
// 	err := json.Unmarshal(data, sa)
// 	if err != nil {
// 		panic(err)
// 	}
// 	// Inputs не инициализируем, так как оно не сериализуется в JSON
// 	// и должно оставаться nil после десериализации
// 	return sa
// }

// ToBytes converts StateAccount to custom binary format
func (sa *StateAccount) Bytes() []byte {
	var buf bytes.Buffer

	// Write Address (assuming Address is []byte or has Bytes() method)
	addressBytes := sa.Address.Bytes()
	binary.Write(&buf, binary.LittleEndian, uint32(len(addressBytes)))
	buf.Write(addressBytes)

	// Write balance as big.Int bytes
	balanceBytes := sa.balance.Bytes()
	binary.Write(&buf, binary.LittleEndian, uint32(len(balanceBytes)))
	buf.Write(balanceBytes)

	// Write Bloom
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.Bloom)))
	buf.Write(sa.Bloom)

	// Write CodeHash
	binary.Write(&buf, binary.LittleEndian, uint32(len(sa.CodeHash)))
	buf.Write(sa.CodeHash)

	// Write Nonce
	binary.Write(&buf, binary.LittleEndian, sa.Nonce)

	// Write Root (assuming common.Hash has Bytes() method)
	rootBytes := sa.Root.Bytes()
	buf.Write(rootBytes)

	// Write Inputs map
	sa.Inputs.RLock()
	inputCount := uint32(len(sa.Inputs.M))
	sa.Inputs.RUnlock()
	binary.Write(&buf, binary.LittleEndian, inputCount)

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

	// Write Status
	statusBytes := []byte(sa.Status)
	binary.Write(&buf, binary.LittleEndian, uint32(len(statusBytes)))
	buf.Write(statusBytes)

	// Write Passphrase
	passphraseBytes := sa.Passphrase.Bytes()
	buf.Write(passphraseBytes)

	// Write MPub
	mpubBytes := []byte(sa.MPub)
	binary.Write(&buf, binary.LittleEndian, uint32(len(mpubBytes)))
	buf.Write(mpubBytes)

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
	sa.Address = Address(addressBytes)

	// Read balance
	var balanceLen uint32
	binary.Read(buf, binary.LittleEndian, &balanceLen)
	balanceBytes := make([]byte, balanceLen)
	buf.Read(balanceBytes)
	sa.balance = new(big.Int).SetBytes(balanceBytes)

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

	// Read Root
	rootBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	buf.Read(rootBytes)
	sa.Root = common.Hash(rootBytes)

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

	// Read Status
	var statusLen uint32
	binary.Read(buf, binary.LittleEndian, &statusLen)
	statusBytes := make([]byte, statusLen)
	buf.Read(statusBytes)
	sa.Status = string(statusBytes)

	// Read Passphrase
	passphraseBytes := make([]byte, 32) // Assuming common.Hash is 32 bytes
	buf.Read(passphraseBytes)
	sa.Passphrase = common.Hash(passphraseBytes)

	// Read MPub
	var mpubLen uint32
	binary.Read(buf, binary.LittleEndian, &mpubLen)
	mpubBytes := make([]byte, mpubLen)
	buf.Read(mpubBytes)
	sa.MPub = string(mpubBytes)

	return sa
}
