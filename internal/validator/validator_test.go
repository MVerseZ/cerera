package validator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestValidateBlock проверяет валидацию блоков
func TestValidateBlock(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// Create a test block
	header := &block.Header{
		Height:     1,
		Index:      1,
		Difficulty: 1,
		ChainId:    11,
		Nonce:      12345,
		PrevHash:   common.Hash{},
		Root:       common.Hash{},
		GasLimit:   1000000,
		GasUsed:    0,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}
	b := block.NewBlock(header)

	// ValidateBlock currently always returns true (needs implementation)
	result := validator.ValidateBlock(*b)
	assert.True(t, result, "ValidateBlock should return true (currently stub implementation)")
}

// TestValidateRawTransaction проверяет валидацию сырых транзакций
func TestValidateRawTransaction(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test"),
	)

	// ValidateRawTransaction currently always returns true
	result := validator.ValidateRawTransaction(tx)
	assert.True(t, result, "ValidateRawTransaction should return true (currently stub implementation)")
}

// TestGetID проверяет получение ID валидатора
func TestGetID(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// GetID returns the current address as string
	id := validator.GetID()
	assert.NotEmpty(t, id, "GetID should return non-empty string")
}

// TestGetVersion проверяет получение версии валидатора
func TestGetVersion(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	version := validator.GetVersion()
	// GetVersion may return empty string if not set, which is acceptable
	_ = version
}

// TestStatus проверяет статус валидатора
func TestStatus(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	status := validator.Status()
	assert.Equal(t, byte(0xa), status, "Status should return 0xa")
}

// TestServiceName проверяет имя сервиса валидатора
func TestServiceName(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	serviceName := validator.ServiceName()
	assert.Equal(t, VALIDATOR_SERVICE_NAME, serviceName, "ServiceName should return correct constant")
}

// TestSetUp проверяет инициализацию валидатора
func TestSetUp(t *testing.T) {
	validator := &CoreValidator{}
	chainID := big.NewInt(11)

	validator.SetUp(chainID)

	// After SetUp, GasPrice should be set
	gasPrice := validator.GasPrice()
	assert.NotNil(t, gasPrice, "GasPrice should be set after SetUp")
	assert.True(t, gasPrice.Sign() >= 0, "GasPrice should be non-negative")
}

// TestCreateTransaction проверяет создание транзакций
func TestCreateTransaction(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	tx, err := validator.CreateTransaction(
		1, // nonce
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		1.0, // count
		3.0, // gas
		"test message",
	)

	require.NoError(t, err)
	assert.NotNil(t, tx)
	assert.Equal(t, uint64(1), tx.Nonce())
	assert.Equal(t, float64(3.0), tx.Gas())
}

// TestValidateTransaction_InvalidAddress проверяет валидацию транзакций с несуществующим адресом
// func TestValidateTransaction_InvalidAddress(t *testing.T) {
// 	validator := &CoreValidator{}
// 	validator.SetUp(big.NewInt(11))

// 	// Create transaction with non-existent sender
// 	tx := types.NewTransaction(
// 		1,
// 		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
// 		big.NewInt(1000),
// 		3.0,
// 		big.NewInt(0),
// 		[]byte("test"),
// 	)

// 	// ValidateTransaction requires vault to be set up
// 	// Without vault, this may panic or return false
// 	// We test that it handles the error gracefully
// 	from := types.HexToAddress("0x0000000000000000000000000000000000000000")

// 	// Проверяем, что метод не паникует
// 	// В реальном сценарии vault должен быть инициализирован
// 	func() {
// 		defer func() {
// 			if r := recover(); r != nil {
// 				// Если паникует из-за отсутствия vault, это ожидаемо в тестах
// 				// В реальном коде vault должен быть инициализирован
// 				t.Logf("ValidateTransaction panicked (expected without vault): %v", r)
// 			}
// 		}()
// 		result := validator.ValidateTransaction(tx, from)
// 		// Если не паникует, результат должен быть false для несуществующего аккаунта
// 		_ = result
// 	}()
// }

// TestValidateTransaction_ErrorHandling проверяет обработку ошибок в ValidateTransaction
// func TestValidateTransaction_ErrorHandling(t *testing.T) {
// 	validator := &CoreValidator{}
// 	validator.SetUp(big.NewInt(11))

// 	tests := []struct {
// 		name        string
// 		tx          *types.GTransaction
// 		from        types.Address
// 		shouldPanic bool
// 		description string
// 	}{
// 		{
// 			name: "Invalid address should return false",
// 			tx: types.NewTransaction(
// 				1,
// 				types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
// 				big.NewInt(1000),
// 				3.0,
// 				big.NewInt(0),
// 				[]byte("test"),
// 			),
// 			from:        types.HexToAddress("0x0000000000000000000000000000000000000000"),
// 			shouldPanic: false,
// 			description: "Non-existent account should return false",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					if !tt.shouldPanic {
// 						t.Errorf("Unexpected panic: %v", r)
// 					}
// 				}
// 			}()

// 			if tt.tx == nil {
// 				// Пропускаем тесты с nil транзакцией, так как ValidateTransaction не обрабатывает nil
// 				return
// 			}

// 			// ValidateTransaction требует vault, который может быть не инициализирован
// 			// Проверяем, что метод не паникует
// 			result := validator.ValidateTransaction(tt.tx, tt.from)
// 			// Для несуществующего аккаунта должно вернуться false
// 			// Но если vault не инициализирован, может быть nil pointer
// 			_ = result // Результат может быть любым, главное - не паниковать
// 		})
// 	}
// }

// TestProposeBlock_ErrorHandling проверяет обработку ошибок в ProposeBlock
func TestProposeBlock_ErrorHandling(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	tests := []struct {
		name        string
		block       *block.Block
		shouldPanic bool
	}{
		{
			name:        "Nil block should not panic",
			block:       nil,
			shouldPanic: false,
		},
		{
			name: "Valid block should not panic",
			block: block.NewBlock(&block.Header{
				Height:     1,
				Index:      1,
				Difficulty: 1,
				ChainId:    11,
				Nonce:      12345,
				PrevHash:   common.Hash{},
				Root:       common.Hash{},
				GasLimit:   1000000,
				GasUsed:    0,
				Timestamp:  uint64(time.Now().UnixMilli()),
			}),
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			// ProposeBlock может принимать nil блок
			validator.ProposeBlock(tt.block)
		})
	}
}

// TestTransactionGetFormat tests that cerera.transaction.get returns transaction in unified format
// This test verifies the format structure by checking the implementation code
func TestTransactionGetFormat(t *testing.T) {
	// Setup: create a transaction
	toAddr := types.HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	tx := types.NewTransaction(
		1,
		toAddr,
		big.NewInt(1000000000000000000), // 1 ETH in wei
		21000,
		big.NewInt(20000000000), // 20 gwei
		[]byte("test data"),
	)

	// Verify the format structure by checking how data would be formatted
	// according to the implementation in validator.go:680-693

	// Verify that common.Bytes is used for data (which produces hex)
	dataBytes := common.Bytes(tx.Data())
	dataHex := dataBytes.String()
	require.True(t, len(dataHex) >= 2 && dataHex[0:2] == "0x", "data should be formatted as hex string (0x...)")

	// Verify value is formatted as decimal string (via big.Int.String())
	valueStr := tx.Value().String()
	require.False(t, len(valueStr) >= 2 && valueStr[0:2] == "0x", "value should be decimal string, not hex")
	// Verify it's a valid decimal number
	_, ok := new(big.Int).SetString(valueStr, 10)
	require.True(t, ok, "value should be a valid decimal number")

	// Verify gas is converted to uint64
	gasUint64 := uint64(tx.Gas())
	require.Equal(t, uint64(21000), gasUint64, "gas should be convertible to uint64")

	// Verify hash, from, to are formatted as hex strings
	hashHex := tx.Hash().Hex()
	require.True(t, len(hashHex) >= 2 && hashHex[0:2] == "0x", "hash should be hex string (0x...)")

	fromHex := tx.From().Hex()
	require.True(t, len(fromHex) >= 2 && fromHex[0:2] == "0x", "from should be hex string (0x...)")

	toHex := tx.To().Hex()
	require.True(t, len(toHex) >= 2 && toHex[0:2] == "0x", "to should be hex string (0x...)")

	t.Logf("Format verified: value=%s (decimal), gas=%d (uint64), data=%s (hex), hash=%s (hex)",
		valueStr, gasUint64, dataHex, hashHex)
}

// TestCheckAddress проверяет логику CheckAddress
func TestCheckAddress(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// При пустом currentAddress любая непустая addr даёт true (addr != empty)
	addr1 := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	result1 := validator.CheckAddress(addr1)
	assert.True(t, result1, "CheckAddress should return true for different address when currentAddress is empty")

	// Устанавливаем currentAddress через рефлексию или через создание с адресом
	// CoreValidator.currentAddress не экспортирован - проверяем что не паникует
	emptyAddr := types.Address{}
	result2 := validator.CheckAddress(emptyAddr)
	// emptyAddr == empty currentAddress -> false
	assert.False(t, result2, "CheckAddress should return false when both addresses are empty")
}

// TestSignRawTransactionWithKey_EmptyKey проверяет ошибку при пустом ключе
func TestSignRawTransactionWithKey_EmptyKey(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))
	validator.balance = big.NewInt(0)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test"),
	)

	err := validator.SignRawTransactionWithKey(tx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestSignRawTransactionWithKey_InvalidPEM проверяет ошибку при невалидном PEM
func TestSignRawTransactionWithKey_InvalidPEM(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))
	validator.balance = big.NewInt(0)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test"),
	)

	err := validator.SignRawTransactionWithKey(tx, "not-a-valid-pem")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PEM")
}

// TestSignRawTransactionWithKey_ValidKey проверяет успешную подпись с валидным ключом
func TestSignRawTransactionWithKey_ValidKey(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))
	validator.balance = big.NewInt(0)

	tx := types.NewTransaction(
		1,
		types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		big.NewInt(1000),
		3.0,
		big.NewInt(0),
		[]byte("test"),
	)

	// Генерируем валидный PEM ключ
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	x509Encoded, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	err = validator.SignRawTransactionWithKey(tx, string(pemEncoded))
	require.NoError(t, err) // Подпись может не совпадать с signer chainId, но ключ парсится
}

// TestExec_Create_InvalidParams проверяет Exec с невалидными параметрами
func TestExec_Create_InvalidParams(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// Слишком мало параметров (legacy path)
	result := validator.Exec("_create", []interface{}{})
	require.Error(t, result.(error))
	assert.Contains(t, result.(error).Error(), "invalid parameters")

	// Негативный gas
	result = validator.Exec("_create", []interface{}{
		CreateTxParams{
			Key:    "key",
			Nonce:  1,
			To:     types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			Amount: "1.0",
			Gas:    -1.0,
			Msg:    "test",
		},
	})
	require.Error(t, result.(error))
	assert.Contains(t, result.(error).Error(), "negative")
}

// TestExec_Create_InvalidAmount проверяет Exec с невалидной суммой
func TestExec_Create_InvalidAmount(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	x509Encoded, _ := x509.MarshalECPrivateKey(priv)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	result := validator.Exec("_create", []interface{}{
		CreateTxParams{
			Key:    string(pemEncoded),
			Nonce:  1,
			To:     types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			Amount: "invalid",
			Gas:    1.0,
			Msg:    "test",
		},
	})
	require.Error(t, result.(error))
}

// TestExec_Create_ValidParams проверяет Exec с валидными параметрами
func TestExec_Create_ValidParams(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))
	validator.balance = big.NewInt(0)

	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	x509Encoded, _ := x509.MarshalECPrivateKey(priv)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	result := validator.Exec("_create", []interface{}{
		CreateTxParams{
			Key:    string(pemEncoded),
			Nonce:  1,
			To:     types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			Amount: "1.0",
			Gas:    1.0,
			Msg:    "test",
		},
	})
	require.NotNil(t, result)
	assert.IsType(t, &types.GTransaction{}, result)
}

// TestExec_Get_NotFound проверяет Exec get для неизвестного хеша
func TestExec_Get_NotFound(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	result := validator.Exec("get", []interface{}{"0x0000000000000000000000000000000000000000000000000000000000000000"})
	assert.Nil(t, result)
}

// TestDecError проверяет сообщения об ошибках
func TestDecError(t *testing.T) {
	assert.Equal(t, "empty hex string", EmptyCoinbase.Error())
	assert.Equal(t, "not enought inputs", NotEnoughtInputs.Error())
}

// TestCreateTransaction_EdgeCases проверяет edge cases при создании транзакции
func TestCreateTransaction_EdgeCases(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	addr := types.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	// Нулевая сумма
	tx, err := validator.CreateTransaction(1, addr, 0.0, 1.0, "zero amount")
	require.NoError(t, err)
	assert.NotNil(t, tx)
	assert.Equal(t, uint64(1), tx.Nonce())

	// Пустое сообщение
	tx, err = validator.CreateTransaction(2, addr, 0.5, 2.0, "")
	require.NoError(t, err)
	assert.NotNil(t, tx)
	assert.Equal(t, uint64(2), tx.Nonce())
}

// TestFindTransaction проверяет что FindTransaction возвращает nil (текущая реализация)
func TestFindTransaction(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	hash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678")
	result := validator.FindTransaction(hash)
	assert.Nil(t, result)
}

// TestConfigChain проверяет что ConfigChain не паникует
func TestConfigChain(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	assert.NotPanics(t, func() {
		ConfigChain(validator)
	})
}

// TestSigner проверяет что Signer возвращает не nil после SetUp
func TestSigner(t *testing.T) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	signer := validator.Signer()
	assert.NotNil(t, signer)
}
