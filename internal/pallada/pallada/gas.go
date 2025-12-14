package pallada

// GasCost представляет стоимость газа для различных операций
type GasCost struct {
	Zero     uint64 // Базовые операции (0 газа)
	Base     uint64 // Базовые операции (3 газа)
	VeryLow  uint64 // Очень дешевые операции (3 газа)
	Low      uint64 // Дешевые операции (5 газа)
	Mid      uint64 // Средние операции (8 газа)
	High     uint64 // Дорогие операции (10 газа)
	ExtCode  uint64 // Операции с внешним кодом (700 газа)
	Balance  uint64 // Чтение баланса (400 газа)
	SLoad    uint64 // Чтение storage (200 газа)
	SStore   uint64 // Запись в storage (20000 газа)
	Call     uint64 // Вызов контракта (700 газа)
	Create   uint64 // Создание контракта (32000 газа)
	Memory   uint64 // Память (3 газа за слово)
	Log      uint64 // Лог (375 газа)
	LogData  uint64 // Данные лога (8 газа за байт)
	LogTopic uint64 // Топик лога (375 газа)
	Sha3     uint64 // SHA3 (30 газа)
	Sha3Word uint64 // SHA3 слово (6 газа)
}

// DefaultGasCost возвращает стандартные стоимости газа
func DefaultGasCost() *GasCost {
	return &GasCost{
		Zero:     0,
		Base:     2,
		VeryLow:  3,
		Low:      5,
		Mid:      8,
		High:     10,
		ExtCode:  700,
		Balance:  400,
		SLoad:    200,
		SStore:   20000,
		Call:     700,
		Create:   32000,
		Memory:   3,
		Log:      375,
		LogData:  8,
		LogTopic: 375,
		Sha3:     30,
		Sha3Word: 6,
	}
}

// GasMeter отслеживает использование газа
type GasMeter struct {
	gasLimit uint64
	gasUsed  uint64
	Cost     *GasCost // Экспортируем для доступа из operations.go
}

// NewGasMeter создает новый счетчик газа
func NewGasMeter(gasLimit uint64) *GasMeter {
	return &GasMeter{
		gasLimit: gasLimit,
		gasUsed:  0,
		Cost:     DefaultGasCost(),
	}
}

// UseGas расходует газ и возвращает ошибку, если лимит превышен
func (gm *GasMeter) UseGas(amount uint64) error {
	if gm.gasUsed+amount > gm.gasLimit {
		return ErrOutOfGas
	}
	gm.gasUsed += amount
	return nil
}

// RefundGas возвращает газ (для операций типа SSTORE)
func (gm *GasMeter) RefundGas(amount uint64) {
	if amount > gm.gasUsed {
		gm.gasUsed = 0
	} else {
		gm.gasUsed -= amount
	}
}

// GasUsed возвращает использованный газ
func (gm *GasMeter) GasUsed() uint64 {
	return gm.gasUsed
}

// GasRemaining возвращает оставшийся газ
func (gm *GasMeter) GasRemaining() uint64 {
	if gm.gasUsed > gm.gasLimit {
		return 0
	}
	return gm.gasLimit - gm.gasUsed
}

// GasLimit возвращает лимит газа
func (gm *GasMeter) GasLimit() uint64 {
	return gm.gasLimit
}

// CalculateMemoryGas рассчитывает газ для расширения памяти
func (gm *GasMeter) CalculateMemoryGas(oldSize, newSize uint64) uint64 {
	if newSize <= oldSize {
		return 0
	}
	// Газ = (newSize^2 - oldSize^2) / 512 + 3 * (newSize - oldSize)
	// Упрощенная версия: 3 газа за каждое новое слово (32 байта)
	words := (newSize + 31) / 32
	oldWords := (oldSize + 31) / 32
	if words <= oldWords {
		return 0
	}
	return (words - oldWords) * gm.Cost.Memory
}

// CalculateCallDataGas рассчитывает газ для данных вызова
func (gm *GasMeter) CalculateCallDataGas(dataSize uint64) uint64 {
	// 68 газа за ненулевой байт, 4 газа за нулевой байт
	// Упрощенная версия: 16 газа за байт
	return dataSize * 16
}

// Ошибка нехватки газа
var ErrOutOfGas = &GasError{message: "out of gas"}

// GasError представляет ошибку, связанную с газом
type GasError struct {
	message string
}

func (e *GasError) Error() string {
	return e.message
}

// IsOutOfGas проверяет, является ли ошибка ошибкой нехватки газа
func IsOutOfGas(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*GasError)
	return ok
}
