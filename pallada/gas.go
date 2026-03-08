package pallada

import (
	"fmt"
)

const MAX_GAS_LIMIT = 100000000000

// MinTransferGas is the minimum gas required for a canonical value transfer.
// Derived from precompiling the minimal transfer bytecode:
//
//	PUSH1+SLOAD+PUSH1+SUB+PUSH1+SSTORE (sender side)
//	PUSH1+PUSH1+SLOAD+ADD+PUSH1+SSTORE (receiver side) + STOP
//
// = 632 gas units (1 gas unit = 1 DUST, 1 CER = 1,000,000 DUST).
const MinTransferGas uint64 = 632

// GasMeterImpl реализует интерфейс GasMeter
// Принцип: Single Responsibility (SRP) - только управление газом
type GasMeterImpl struct {
	gasLimit uint64 // Лимит газа
	gasUsed  uint64 // Использованный газ
}

// NewGasMeter создает новый счетчик газа
func NewGasMeterWithLimit(gasLimit uint64) *GasMeterImpl {
	return &GasMeterImpl{
		gasLimit: gasLimit,
		gasUsed:  0,
	}
}

func NewGasMeter() *GasMeterImpl {
	return &GasMeterImpl{
		gasLimit: MAX_GAS_LIMIT,
		gasUsed:  0,
	}
}

// ConsumeGas потребляет указанное количество газа
func (gm *GasMeterImpl) ConsumeGas(amount uint64, reason string) error {
	if gm.gasUsed+amount > gm.gasLimit {
		return fmt.Errorf("out of gas: %s (used: %d, limit: %d, requested: %d)",
			reason, gm.gasUsed, gm.gasLimit, amount)
	}
	gm.gasUsed += amount
	return nil
}

// GasRemaining возвращает оставшееся количество газа
func (gm *GasMeterImpl) GasRemaining() uint64 {
	if gm.gasLimit > gm.gasUsed {
		return gm.gasLimit - gm.gasUsed
	}
	return 0
}

// GasUsed возвращает использованное количество газа
func (gm *GasMeterImpl) GasUsed() uint64 {
	return gm.gasUsed
}

// GasLimit возвращает лимит газа
func (gm *GasMeterImpl) GasLimit() uint64 {
	return gm.gasLimit
}

// Gas costs constants (базовые стоимости операций)
const (
	GasZero        uint64 = 0     // Бесплатные операции
	GasBase        uint64 = 2     // Базовая стоимость операции
	GasVeryLow     uint64 = 3     // Очень дешевые операции
	GasLow         uint64 = 5     // Дешевые операции
	GasMid         uint64 = 8     // Средние операции
	GasHigh        uint64 = 10    // Дорогие операции
	GasExtStep     uint64 = 20    // Шаг расширения памяти
	GasExtByte     uint64 = 4     // Байт расширения памяти
	GasSLoad       uint64 = 100   // Загрузка из storage
	GasSStore      uint64 = 200   // Сохранение в storage (базовая)
	GasSStoreSet   uint64 = 20000 // Сохранение нового значения в storage
	GasSStoreReset uint64 = 5000  // Изменение существующего значения в storage
	GasCall        uint64 = 100   // Базовая стоимость CALL
	GasCallValue   uint64 = 9000  // Дополнительная стоимость при передаче value
	GasCallStipend uint64 = 2300  // Газ, передаваемый вызываемому контракту
	GasReturn      uint64 = 0     // RETURN бесплатный
	GasRevert      uint64 = 0     // REVERT бесплатный

	// Хэш-операции
	GasKeccak256     uint64 = 30  // Базовая стоимость KECCAK256
	GasKeccak256Word uint64 = 6   // Стоимость за каждые 32 байта входа KECCAK256
	GasSHA256        uint64 = 60  // Базовая стоимость SHA256
	GasSHA256Word    uint64 = 12  // Стоимость за каждые 32 байта входа SHA256
	GasRIPEMD160     uint64 = 600 // Базовая стоимость RIPEMD160
	GasRIPEMD160Word uint64 = 120 // Стоимость за каждые 32 байта входа RIPEMD160

	// ECRECOVER
	GasEcrecover uint64 = 3000 // Стоимость восстановления адреса из подписи

	// Управление потоком
	GasJump     uint64 = 8  // JUMP
	GasJumpi    uint64 = 10 // JUMPI
	GasJumpdest uint64 = 1  // JUMPDEST (маркер, дешевый)

	// Calldata
	GasCalldataLoad uint64 = 3  // CALLDATALOAD
	GasCalldataSize uint64 = 2  // CALLDATASIZE
	GasCalldataCopy uint64 = 3  // CALLDATACOPY базовая + 3 за каждые 32 байта

	// LOG события
	GasLogBase    uint64 = 375  // Базовая стоимость LOG
	GasLogData    uint64 = 8    // Стоимость за байт данных события
	GasLogTopic   uint64 = 375  // Стоимость за каждый топик
)

// CalculateMemoryGas вычисляет стоимость газа для операций с памятью
func CalculateMemoryGas(currentSize, newSize uint64) uint64 {
	if newSize <= currentSize {
		return 0
	}

	// Стоимость расширения памяти
	expansion := newSize - currentSize
	// Стоимость = (expansion^2 / 512) + (expansion * GasExtByte)
	// Упрощенная формула для производительности
	cost := (expansion * expansion / 512) + (expansion * GasExtByte)
	return cost
}

// MinGasPrice returns the minimum gas price expressed in CER.
// 1 GAS UNIT = 1 DUST, 1 CER = 1,000,000 DUST → 1 DUST = 0.000001 CER.
// FloatToBigInt(MinGasPrice()) == 1 DUST == 1 gas unit cost.
func MinGasPrice() float64 {
	return 0.000001 // 1 DUST per gas unit
}
