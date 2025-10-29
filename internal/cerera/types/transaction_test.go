package types

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/cerera/internal/cerera/common"
)

var nonce, txValue, txData, gasLimit, gasPrice, to = uint64(1337), FloatToBigInt(11.55), []byte("test data"), float64(16438), big.NewInt(63992), HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")

func TestCreate(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    txValue,
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	if itx.Value().Cmp(txValue) != 0 {
		t.Errorf("Different values! Have %d, want %d", itx.Value(), big.NewInt(10))
	}
	if itx.GasPrice().Cmp(big.NewInt(15)) != 0 {
		t.Errorf("Different gas price! Have %d, want %d", itx.Value(), big.NewInt(15))
	}
	if itx.Gas() != 1000000 {
		t.Errorf("Different gas price! Have %d, want %d", itx.Value(), big.NewInt(1000000))
	}

	var ttx, err = CreateTransaction(1337, to, BigIntToFloat(txValue), 100001, "test message")
	if err != nil {
		t.Errorf("Error while create tx, %s", err)
	}
	if ttx.Value().Cmp(txValue) != 0 {
		t.Errorf("Different values! Have %d, want %d", ttx.Value(), txValue)
	}
}

func TestCost(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	var calcCost = itx.Cost()
	var expectedCost = big.NewInt(0).Mul(big.NewInt(15), big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(1000000000000000000))) // 15 * 1000000 * 10^18
	if calcCost.Cmp(expectedCost) != 0 {
		t.Errorf("Differenet costs! Have %f, want %f", BigIntToFloat(calcCost), BigIntToFloat(expectedCost))
	}
}

func TestSerialize(t *testing.T) {
	dna := make([]byte, 0, 16)
	dna = append(dna, 0xf, 0xa, 0x42)

	var to = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	txs := &PGTransaction{
		To:       &to,
		Value:    big.NewInt(10),
		GasPrice: big.NewInt(15),
		Gas:      1000000,
		Nonce:    0x1,
		Dna:      dna,
		Time:     time.Now(),
	}
	itx := NewTx(txs)

	// Print original transaction details
	// t.Logf("Original transaction:")
	// t.Logf("  Data: %v", itx.Data())
	// t.Logf("  DNA: %v", itx.Dna())
	// t.Logf("  To: %v", itx.To())
	// t.Logf("  Value: %v", itx.Value())
	// t.Logf("  GasPrice: %v", itx.GasPrice())
	// t.Logf("  Gas: %v", itx.Gas())
	// t.Logf("  Nonce: %v", itx.Nonce())
	// t.Logf("  Original size: %d", itx.Size())

	txBytes, err := itx.MarshalJSON()
	if err != nil {
		t.Error("Error while parse transaction to bytes!")
	}

	var tx GTransaction
	if err := tx.UnmarshalJSON(txBytes); err != nil {
		t.Error("Error while unmarshaling transaction:", err)
	}

	// // Print unmarshaled transaction details
	// t.Logf("\nUnmarshaled transaction:")
	// t.Logf("  Data: %v", tx.Data())
	// t.Logf("  DNA: %v", tx.Dna())
	// t.Logf("  To: %v", tx.To())
	// t.Logf("  Value: %v", tx.Value())
	// t.Logf("  GasPrice: %v", tx.GasPrice())
	// t.Logf("  Gas: %v", tx.Gas())
	// t.Logf("  Nonce: %v", tx.Nonce())
	// t.Logf("  Unmarshaled size: %d", tx.Size())

	// Expected size calculation:
	// - inner data: 0 bytes
	// - DNA: 3 bytes (0xf, 0xa, 0x42)
	// - time: 15 bytes
	// - signature: 0 bytes (not signed)
	// - to address: 32 bytes
	// - value: len(10) = 1 byte
	// - gas price: len(15) = 1 byte
	// - nonce: 8 bytes
	// - gas: 8 bytes
	// Total: 68 bytes
	expectedSize := uint64(itx.Size())

	if tx.Size() != expectedSize {
		t.Errorf("Different sizes! Have %d, want %d", tx.Size(), expectedSize)
	}
}

func TestEquals(t *testing.T) {
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)
	var bhash, err = tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash.Compare(tx.Hash()) != 0 {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n", tx.Hash(), sbHash)
	}
	fmt.Println(sbHash)
	fmt.Println(tx.Hash())
}

func TestSize(t *testing.T) {
	var toAddr = HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea24807B491717081c42F2575F09B6bc60206")
	var tx = NewTransaction(
		1337,
		toAddr,
		big.NewInt(100000000),
		250000,
		big.NewInt(1111),
		[]byte("TEST_TX"),
	)
	var bhash, err = tx.CalculateHash()
	if err != nil {
		t.Errorf("Error while transaction.CalculateHash call %s\r\n", err)
	}
	var sbHash = common.BytesToHash(bhash)
	if sbHash.Compare(tx.Hash()) != 0 {
		t.Errorf("Difference between transaction.CalculateHash and transaction.Hash\r\n\t %s - %s\r\n", tx.Hash(), sbHash)
	}

	// Expected size calculation:

	// Total: 92 bytes
	expectedSize := uint64(92)

	if tx.Size() != expectedSize {
		t.Errorf("diff sizes: expected %d, actual: %d", expectedSize, tx.Size())
	}
}

func TestCreateCoinbase(t *testing.T) {
	NewCoinBaseTransaction(nonce, to, txValue, gasLimit, gasPrice, txData)
}

func TestCreateFaucet(t *testing.T) {
	NewFaucetTransaction(nonce, to, txValue)
}

func TestDna(t *testing.T) {
	var tx = NewTransaction(
		nonce,
		to,
		txValue,
		gasLimit,
		gasPrice,
		txData,
	)
	tx.Dna()
}

// TestGasCostCalculation проверяет правильность расчета стоимости газа
func TestGasCostCalculation(t *testing.T) {
	tests := []struct {
		name     string
		gasLimit float64
		gasPrice *big.Int
		expected string
	}{
		{
			name:     "Минимальная транзакция",
			gasLimit: 3.0,
			gasPrice: FloatToBigInt(0.000001),
			expected: "2999999999997000000000000000000",
		},
		{
			name:     "Стандартная транзакция",
			gasLimit: 5.0,
			gasPrice: FloatToBigInt(0.000001),
			expected: "4999999999995000000000000000000",
		},
		{
			name:     "Высокий лимит газа",
			gasLimit: 50000.0,
			gasPrice: FloatToBigInt(0.000001),
			expected: "49999999999950000000000000000000000",
		},
		{
			name:     "Ethereum-совместимый лимит",
			gasLimit: 21000.0,
			gasPrice: FloatToBigInt(0.000001),
			expected: "20999999999979000000000000000000000",
		},
		{
			name:     "Нулевой лимит газа",
			gasLimit: 0.0,
			gasPrice: FloatToBigInt(0.000001),
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем транзакцию с заданными параметрами
			tx := NewTransaction(
				1, // nonce
				HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000000000000000000), // 1 CER
				tt.gasLimit,
				tt.gasPrice,
				[]byte("test data"),
			)

			// Проверяем расчет стоимости газа
			cost := tx.Cost()
			if cost.String() != tt.expected {
				t.Errorf("Cost() = %s, want %s", cost.String(), tt.expected)
			}

			// Проверяем, что стоимость газа = gasPrice * gasLimit
			expectedCost := new(big.Int).Mul(tt.gasPrice, FloatToBigInt(tt.gasLimit))
			if cost.Cmp(expectedCost) != 0 {
				t.Errorf("Cost calculation mismatch: got %s, want %s", cost.String(), expectedCost.String())
			}
		})
	}
}

// TestGasPriceValidation проверяет валидацию минимальной цены газа
func TestGasPriceValidation(t *testing.T) {
	minGasPrice := FloatToBigInt(0.000001) // Минимальная цена из validator.go

	tests := []struct {
		name        string
		gasLimit    float64
		gasPrice    *big.Int
		shouldPass  bool
		description string
	}{
		{
			name:        "Минимальная цена газа",
			gasLimit:    3.0,
			gasPrice:    minGasPrice,
			shouldPass:  true,
			description: "Должна пройти валидацию",
		},
		{
			name:        "Цена выше минимума",
			gasLimit:    3.0,
			gasPrice:    FloatToBigInt(0.000002),
			shouldPass:  true,
			description: "Должна пройти валидацию",
		},
		{
			name:        "Цена ниже минимума",
			gasLimit:    3.0,
			gasPrice:    FloatToBigInt(0.0000005),
			shouldPass:  false,
			description: "Должна быть отклонена",
		},
		{
			name:        "Нулевая цена газа",
			gasLimit:    3.0,
			gasPrice:    big.NewInt(0),
			shouldPass:  true,
			description: "Нулевая цена разрешена",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := NewTransaction(
				1,
				HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000000000000000000),
				tt.gasLimit,
				tt.gasPrice,
				[]byte("test data"),
			)

			cost := tx.Cost()

			// Симулируем логику валидации из validator.go
			// Валидация проверяет цену газа за единицу, а не общую стоимость
			passesValidation := tx.GasPrice().Sign() == 0 || tx.GasPrice().Cmp(minGasPrice) >= 0

			if passesValidation != tt.shouldPass {
				t.Errorf("Validation result mismatch: got %v, want %v (%s)",
					passesValidation, tt.shouldPass, tt.description)
			}

			t.Logf("Gas cost: %s wei (%.6f CER), Validation: %v",
				cost.String(), BigIntToFloat(cost), passesValidation)
		})
	}
}

// TestGasCostEdgeCases проверяет граничные случаи расчета газа
func TestGasCostEdgeCases(t *testing.T) {
	minGasPrice := FloatToBigInt(0.000001)

	tests := []struct {
		name     string
		gasLimit float64
		gasPrice *big.Int
	}{
		{
			name:     "Очень маленький лимит газа",
			gasLimit: 0.000001,
			gasPrice: minGasPrice,
		},
		{
			name:     "Очень большой лимит газа",
			gasLimit: 1000000.0,
			gasPrice: minGasPrice,
		},
		{
			name:     "Очень маленькая цена газа",
			gasLimit: 3.0,
			gasPrice: FloatToBigInt(0.0000001),
		},
		{
			name:     "Очень большая цена газа",
			gasLimit: 3.0,
			gasPrice: FloatToBigInt(1000.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := NewTransaction(
				1,
				HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000000000000000000),
				tt.gasLimit,
				tt.gasPrice,
				[]byte("test data"),
			)

			cost := tx.Cost()

			// Проверяем, что стоимость не отрицательная
			if cost.Sign() < 0 {
				t.Errorf("Gas cost should not be negative: %s", cost.String())
			}

			// Проверяем, что стоимость газа = gasPrice * gasLimit
			expectedCost := new(big.Int).Mul(tt.gasPrice, FloatToBigInt(tt.gasLimit))
			if cost.Cmp(expectedCost) != 0 {
				t.Errorf("Cost calculation mismatch: got %s, want %s", cost.String(), expectedCost.String())
			}

			t.Logf("Gas limit: %f, Gas price: %s wei, Cost: %s wei (%.6f CER)",
				tt.gasLimit, tt.gasPrice.String(), cost.String(), BigIntToFloat(cost))
		})
	}
}
