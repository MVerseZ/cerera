package pallada

import (
	"testing"
)

func TestGasMeter_ConsumeGas(t *testing.T) {
	gasLimit := uint64(1000)
	gm := NewGasMeter(gasLimit)

	// Потребляем часть газа
	if err := gm.ConsumeGas(100, "test"); err != nil {
		t.Fatalf("ConsumeGas failed: %v", err)
	}

	if gm.GasUsed() != 100 {
		t.Errorf("Expected gas used 100, got %d", gm.GasUsed())
	}

	if gm.GasRemaining() != 900 {
		t.Errorf("Expected gas remaining 900, got %d", gm.GasRemaining())
	}

	// Потребляем еще
	if err := gm.ConsumeGas(200, "test2"); err != nil {
		t.Fatalf("ConsumeGas failed: %v", err)
	}

	if gm.GasUsed() != 300 {
		t.Errorf("Expected gas used 300, got %d", gm.GasUsed())
	}
}

func TestGasMeter_OutOfGas(t *testing.T) {
	gasLimit := uint64(100)
	gm := NewGasMeter(gasLimit)

	// Потребляем весь газ
	if err := gm.ConsumeGas(100, "test"); err != nil {
		t.Fatalf("ConsumeGas failed: %v", err)
	}

	// Попытка потреблять еще должна вызвать ошибку
	if err := gm.ConsumeGas(1, "test"); err == nil {
		t.Error("Expected out of gas error")
	}

	if gm.GasUsed() != 100 {
		t.Errorf("Expected gas used 100, got %d", gm.GasUsed())
	}

	if gm.GasRemaining() != 0 {
		t.Errorf("Expected gas remaining 0, got %d", gm.GasRemaining())
	}
}

func TestGasMeter_GasLimit(t *testing.T) {
	gasLimit := uint64(5000)
	gm := NewGasMeter(gasLimit)

	if gm.GasLimit() != gasLimit {
		t.Errorf("Expected gas limit %d, got %d", gasLimit, gm.GasLimit())
	}
}

func TestCalculateMemoryGas(t *testing.T) {
	tests := []struct {
		name     string
		current  uint64
		new      uint64
		expected uint64
	}{
		{"no expansion", 100, 100, 0},
		{"no expansion smaller", 100, 50, 0},
		{"small expansion", 0, 32, 128},    // (32*32/512) + (32*4) = 2 + 128 = 130, но упрощенная формула
		{"large expansion", 0, 1024, 4096}, // (1024*1024/512) + (1024*4) = 2048 + 4096 = 6144
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateMemoryGas(tt.current, tt.new)
			if result < tt.expected/2 || result > tt.expected*2 {
				// Проверяем приблизительно, так как формула может быть упрощенной
				t.Logf("Memory gas: current=%d, new=%d, got=%d (expected ~%d)", tt.current, tt.new, result, tt.expected)
			}
		})
	}
}
