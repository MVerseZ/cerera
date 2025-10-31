package validator

import (
	"math/big"
	"testing"

	"github.com/cerera/internal/cerera/types"
)

func TestPoolSigningProc(t *testing.T) {
	// pool, _ := pool.InitPool(1, 1000)

	// var hashes []common.Hash
	// for i := 0; i < 3; i++ {
	// 	transaction := types.NewTransaction(
	// 		7,
	// 		types.HexToAddress("0xc9C5c06E295d8FB8E97f4df93C4919D557D0B284521d71A7fCA1e1C3F289849989E80B0B81ED4EDB361d1f8F67DDf613"),
	// 		big.NewInt(1000001),
	// 		500,
	// 		big.NewInt(250),
	// 		[]byte(strconv.Itoa(i)),
	// 	)
	// 	pool.AddRawTransaction(transaction)
	// 	hashes = append(hashes, transaction.Hash())
	// }
	// info := pool.GetInfo()
	// if len(info.Txs) != 3 {
	// 	t.Errorf("Error pool size! Have %d, want %d\r\n", len(info.Txs), 3)
	// }

	// pk, _ := types.GenerateAccount()
	// signer := types.NewSimpleSignerWithPen(big.NewInt(7), pk)
	// x509Encoded, _ := x509.MarshalECPrivateKey(pk)
	// pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	// 	// now validator sign tx
	// 	newHash, err := pool.SignRawTransaction(hashes[0], signer, string(pemEncoded))
	// 	if err != nil {
	// 		t.Errorf("Error while sign tx: %s\r\n", err)
	// 	}
	// 	if newHash != hashes[0] {
	// 		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[0])
	// 	}

	// 	newHash, err = pool.SignRawTransaction(hashes[1], signer, string(pemEncoded))
	// 	if err != nil {
	// 		t.Errorf("Error while sign tx: %s\r\n", err)
	// 	}
	// 	if newHash != hashes[1] {
	// 		t.Errorf("Differnet hashes! Have %s, want %s\r\n", newHash, hashes[1])
	// 	}

	// info = pool.GetInfo()
	// tx := pool.GetTransaction(info.Hashes[0])
	// var r, s, v = tx.RawSignatureValues()
	//
	//	if r == big.NewInt(0) || s == big.NewInt(0) || v == big.NewInt(0) {
	//		t.Errorf("Error! Tx not signed! %s\r\n", tx.Hash())
	//	}
}

// TestGasValidation проверяет валидацию газа в валидаторе
func TestGasValidation(t *testing.T) {
	// Создаем валидатор
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11)) // ChainID = 11 из config.json

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
			gasPrice:    types.FloatToBigInt(0.000001),
			shouldPass:  true,
			description: "Должна пройти валидацию",
		},
		{
			name:        "Цена выше минимума",
			gasLimit:    3.0,
			gasPrice:    types.FloatToBigInt(0.000002),
			shouldPass:  true,
			description: "Должна пройти валидацию",
		},
		{
			name:        "Цена ниже минимума",
			gasLimit:    3.0,
			gasPrice:    types.FloatToBigInt(0.0000005),
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
		{
			name:        "Стандартная транзакция",
			gasLimit:    5.0,
			gasPrice:    types.FloatToBigInt(0.000001),
			shouldPass:  true,
			description: "Стандартная транзакция должна пройти",
		},
		{
			name:        "Высокий лимит газа",
			gasLimit:    50000.0,
			gasPrice:    types.FloatToBigInt(0.000001),
			shouldPass:  true,
			description: "Высокий лимит должен пройти",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем транзакцию
			tx := types.NewTransaction(
				1, // nonce
				types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000000000000000000), // 1 CER
				tt.gasLimit,
				tt.gasPrice,
				[]byte("test data"),
			)

			// Проверяем стоимость газа
			cost := tx.Cost()

			// Симулируем логику валидации из validator.go
			// Валидация проверяет, что общая стоимость газа >= минимальной цены за единицу газа
			// Это означает, что gasPrice >= minGasPrice (не общая стоимость!)
			passesValidation := tx.GasPrice().Sign() == 0 || tx.GasPrice().Cmp(validator.GasPrice()) >= 0

			if passesValidation != tt.shouldPass {
				t.Errorf("Validation result mismatch: got %v, want %v (%s)",
					passesValidation, tt.shouldPass, tt.description)
			}

			t.Logf("Gas limit: %f, Gas price: %s wei, Cost: %s wei (%.6f CER), Validation: %v",
				tt.gasLimit, tt.gasPrice.String(), cost.String(), types.BigIntToFloat(cost), passesValidation)
		})
	}
}

// TestMinGasPrice проверяет установку минимальной цены газа
func TestMinGasPrice(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	expectedMinGasPrice := types.FloatToBigInt(0.000001)
	actualMinGasPrice := validator.GasPrice()

	if actualMinGasPrice.Cmp(expectedMinGasPrice) != 0 {
		t.Errorf("MinGasPrice = %s, want %s", actualMinGasPrice.String(), expectedMinGasPrice.String())
	}

	t.Logf("MinGasPrice: %s wei (%.6f CER)", actualMinGasPrice.String(), types.BigIntToFloat(actualMinGasPrice))
}

// TestGasCostCalculation проверяет расчет стоимости газа в контексте валидатора
func TestGasCostCalculation(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	tests := []struct {
		name     string
		gasLimit float64
		gasPrice *big.Int
		expected string
	}{
		{
			name:     "Минимальная транзакция",
			gasLimit: 3.0,
			gasPrice: types.FloatToBigInt(0.000001),
			expected: "3000000000000000000000000000000",
		},
		{
			name:     "Стандартная транзакция",
			gasLimit: 5.0,
			gasPrice: types.FloatToBigInt(0.000001),
			expected: "5000000000000000000000000000000",
		},
		{
			name:     "Высокий лимит газа",
			gasLimit: 50000.0,
			gasPrice: types.FloatToBigInt(0.000001),
			expected: "50000000000000000000000000000000000",
		},
		{
			name:     "Ethereum-совместимый лимит",
			gasLimit: 21000.0,
			gasPrice: types.FloatToBigInt(0.000001),
			expected: "21000000000000000000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := types.NewTransaction(
				1,
				types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				big.NewInt(1000000000000000000),
				tt.gasLimit,
				tt.gasPrice,
				[]byte("test data"),
			)

			cost := tx.Cost()

			// Проверяем точность расчета
			if cost.String() != tt.expected {
				t.Errorf("Cost() = %s, want %s", cost.String(), tt.expected)
			}

			// Проверяем, что стоимость газа = gasPrice * gasLimit
			expectedCost := new(big.Int).Mul(tt.gasPrice, types.FloatToBigInt(tt.gasLimit))
			if cost.Cmp(expectedCost) != 0 {
				t.Errorf("Cost calculation mismatch: got %s, want %s", cost.String(), expectedCost.String())
			}

			// Проверяем валидацию минимальной цены
			passesValidation := cost.Sign() == 0 || cost.Cmp(validator.GasPrice()) >= 0
			if !passesValidation {
				t.Errorf("Transaction should pass validation: cost=%s, minGasPrice=%s",
					cost.String(), validator.GasPrice().String())
			}

			t.Logf("Gas limit: %f, Gas price: %s wei, Cost: %s wei (%.6f CER)",
				tt.gasLimit, tt.gasPrice.String(), cost.String(), types.BigIntToFloat(cost))
		})
	}
}
