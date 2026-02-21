package pallada

import (
	"fmt"
)

// GasMeterImpl реализует интерфейс GasMeter
// Принцип: Single Responsibility (SRP) - только управление газом
type GasMeterImpl struct {
	gasLimit uint64 // Лимит газа
	gasUsed  uint64 // Использованный газ
}

// NewGasMeter создает новый счетчик газа
func NewGasMeter(gasLimit uint64) *GasMeterImpl {
	return &GasMeterImpl{
		gasLimit: gasLimit,
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
