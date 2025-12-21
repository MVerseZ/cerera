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

// StorageInterface определяет интерфейс для работы с хранилищем контракта
// Принцип: Interface Segregation (ISP) - отдельный интерфейс для storage
type StorageInterface interface {
	// GetStorage получает значение из storage контракта по ключу
	GetStorage(address types.Address, key *big.Int) (*big.Int, error)
	// SetStorage сохраняет значение в storage контракта по ключу
	SetStorage(address types.Address, key *big.Int, value *big.Int) error
}

// CallInterface определяет интерфейс для межконтрактных вызовов
// Принцип: Interface Segregation (ISP) - отдельный интерфейс для вызовов
type CallInterface interface {
	// Call вызывает контракт по адресу
	// Возвращает: результат выполнения, успех выполнения, использованный газ
	Call(caller types.Address, address types.Address, value *big.Int, input []byte, gasLimit uint64) ([]byte, bool, uint64)
}

// GasMeter интерфейс для управления газом
// Принцип: Dependency Inversion (DIP) - зависимость от абстракции
type GasMeter interface {
	// ConsumeGas потребляет указанное количество газа
	ConsumeGas(amount uint64, reason string) error
	// GasRemaining возвращает оставшееся количество газа
	GasRemaining() uint64
	// GasUsed возвращает использованное количество газа
	GasUsed() uint64
	// GasLimit возвращает лимит газа
	GasLimit() uint64
}

// Context содержит контекст выполнения контракта
// Принцип: Single Responsibility (SRP) - контекст только хранит данные
type Context struct {
	Caller    types.Address    // Адрес вызывающего контракта/аккаунта
	Address   types.Address    // Адрес текущего контракта
	Value     *big.Int         // Значение, переданное с вызовом
	Input     []byte           // Входные данные
	GasLimit  uint64           // Лимит газа
	GasPrice  *big.Int         // Цена газа
	BlockInfo *BlockInfo       // Информация о блоке
	Storage   StorageInterface // Интерфейс для работы с storage
	Call      CallInterface    // Интерфейс для межконтрактных вызовов
}

// NewContext создает новый контекст выполнения
func NewContext(
	caller types.Address,
	address types.Address,
	value *big.Int,
	input []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	blockInfo *BlockInfo,
	storage StorageInterface,
) *Context {
	return &Context{
		Caller:    caller,
		Address:   address,
		Value:     value,
		Input:     input,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		BlockInfo: blockInfo,
		Storage:   storage,
		Call:      nil, // По умолчанию без поддержки вызовов
	}
}

// NewContextWithCall создает новый контекст с поддержкой межконтрактных вызовов
func NewContextWithCall(
	caller types.Address,
	address types.Address,
	value *big.Int,
	input []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	blockInfo *BlockInfo,
	storage StorageInterface,
	call CallInterface,
) *Context {
	return &Context{
		Caller:    caller,
		Address:   address,
		Value:     value,
		Input:     input,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		BlockInfo: blockInfo,
		Storage:   storage,
		Call:      call,
	}
}

// NewContextWithStorage создает новый контекст только с storage (без вызовов)
func NewContextWithStorage(
	caller types.Address,
	address types.Address,
	value *big.Int,
	input []byte,
	gasLimit uint64,
	gasPrice *big.Int,
	blockInfo *BlockInfo,
	storage StorageInterface,
) *Context {
	return NewContext(caller, address, value, input, gasLimit, gasPrice, blockInfo, storage)
}
