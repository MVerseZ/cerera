package pallada

import (
	"math/big"

	"github.com/cerera/internal/cerera/types"
)

// BlockInfo содержит информацию о текущем блоке
type BlockInfo struct {
	Number    uint64 // Номер блока
	Timestamp uint64 // Временная метка блока
	Hash      []byte // Хеш блока (32 байта)
}

// StorageInterface определяет интерфейс для работы с storage контрактов
type StorageInterface interface {
	GetStorage(address types.Address, key *big.Int) (*big.Int, error)
	SetStorage(address types.Address, key *big.Int, value *big.Int) error
}

// CallInterface определяет интерфейс для вызова контрактов из контрактов
// Используется для реализации CALL опкода
type CallInterface interface {
	// Call вызывает контракт по адресу
	// Возвращает: результат выполнения, успех (true = успех, false = ошибка), использованный газ
	Call(caller types.Address, address types.Address, value *big.Int, input []byte, gasLimit uint64) ([]byte, bool, uint64)
}

// Context содержит контекст выполнения контракта в блокчейне
type Context struct {
	Caller    types.Address    // Адрес вызывающего (отправителя транзакции)
	Address   types.Address    // Адрес контракта (получателя)
	Value     *big.Int         // Значение транзакции (wei)
	Input     []byte           // Входные данные транзакции (tx.Data())
	GasLimit  uint64           // Лимит газа для выполнения
	GasPrice  *big.Int         // Цена газа
	BlockInfo *BlockInfo       // Информация о блоке
	Storage   StorageInterface // Интерфейс для работы с storage (может быть nil)
	CallerInt CallInterface    // Интерфейс для вызова контрактов (может быть nil)
}

// NewContext создает новый контекст выполнения
func NewContext(caller, address types.Address, value *big.Int, input []byte, gasLimit uint64, gasPrice *big.Int, blockInfo *BlockInfo) *Context {
	if value == nil {
		value = big.NewInt(0)
	}
	if gasPrice == nil {
		gasPrice = big.NewInt(0)
	}
	if blockInfo == nil {
		blockInfo = &BlockInfo{}
	}
	return &Context{
		Caller:    caller,
		Address:   address,
		Value:     new(big.Int).Set(value),
		Input:     input,
		GasLimit:  gasLimit,
		GasPrice:  new(big.Int).Set(gasPrice),
		BlockInfo: blockInfo,
		Storage:   nil, // Storage устанавливается отдельно
	}
}

// NewContextWithStorage создает новый контекст с storage
func NewContextWithStorage(caller, address types.Address, value *big.Int, input []byte, gasLimit uint64, gasPrice *big.Int, blockInfo *BlockInfo, storage StorageInterface) *Context {
	ctx := NewContext(caller, address, value, input, gasLimit, gasPrice, blockInfo)
	ctx.Storage = storage
	return ctx
}

// NewContextWithCall создает новый контекст с поддержкой вызова контрактов
func NewContextWithCall(caller, address types.Address, value *big.Int, input []byte, gasLimit uint64, gasPrice *big.Int, blockInfo *BlockInfo, storage StorageInterface, callerInt CallInterface) *Context {
	ctx := NewContextWithStorage(caller, address, value, input, gasLimit, gasPrice, blockInfo, storage)
	ctx.CallerInt = callerInt
	return ctx
}

// AddressToBigInt конвертирует Address в big.Int (для стека VM)
func AddressToBigInt(addr types.Address) *big.Int {
	// Address это [32]byte, конвертируем в big.Int
	addrBytes := addr.Bytes()
	return new(big.Int).SetBytes(addrBytes)
}

// BigIntToAddress конвертирует big.Int в Address
func BigIntToAddress(val *big.Int) types.Address {
	var addr types.Address
	addrBytes := val.Bytes()
	// Дополняем до 32 байт слева нулями
	if len(addrBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(addrBytes):], addrBytes)
		addrBytes = padded
	} else if len(addrBytes) > 32 {
		// Берем последние 32 байта
		addrBytes = addrBytes[len(addrBytes)-32:]
	}
	addr.SetBytes(addrBytes)
	return addr
}
