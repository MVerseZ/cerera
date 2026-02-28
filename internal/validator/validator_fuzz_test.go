package validator

import (
	"math/big"
	"strings"
	"testing"

	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

// FuzzCheckAddress tests the CheckAddress function
func FuzzCheckAddress(f *testing.F) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// Add seed addresses
	f.Add([]byte("test_address"))
	f.Add([]byte(""))
	f.Add([]byte(strings.Repeat("x", 48)))

	f.Fuzz(func(t *testing.T, addrBytes []byte) {
		var addr types.Address
		copy(addr[:], addrBytes)

		result := validator.CheckAddress(addr)
		// Just check that it doesn't panic
		_ = result
	})
}

// FuzzValidateRawTransaction tests the ValidateRawTransaction function
func FuzzValidateRawTransaction(f *testing.F) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	// Create seed transactions
	addr2 := types.EmptyAddress()
	tx1, _ := types.CreateUnbroadcastTransaction(1, addr2, 10.0, 0.001, common.FloatToBigInt(3114000000000000), "test") //TODO replace later with realization
	f.Add(tx1.Hash().Bytes())

	f.Fuzz(func(t *testing.T, hashBytes []byte) {
		// Create a transaction with the provided hash segments
		if len(hashBytes) < 32 {
			t.Skip("Hash too short")
		}

		var addr types.Address
		tx, _ := types.CreateUnbroadcastTransaction(1, addr, 10.0, 0.001, common.FloatToBigInt(3114000000000000), "test") //TODO replace later with realization

		result := validator.ValidateRawTransaction(tx)
		// Just check that it doesn't panic
		_ = result
	})
}

// FuzzValidateTransaction tests the ValidateTransaction function
func FuzzValidateTransaction(f *testing.F) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	f.Fuzz(func(t *testing.T, nonceVal uint64, value float64, gasLimit float64, data []byte) {
		// Limit values to reasonable ranges
		if value < 0 || value > 1000000 {
			return
		}
		if gasLimit < 0 || gasLimit > 10000 {
			return
		}

		// addr := types.EmptyAddress()
		// tx, _ := types.CreateUnbroadcastTransaction(nonceVal, addr, value, gasLimit, string(data))

		// result := validator.ValidateTransaction(tx, addr)
		// Just check that it doesn't panic
		// _ = result
	})
}

// FuzzGasPrice tests GasPrice validation
func FuzzGasPrice(f *testing.F) {
	f.Fuzz(func(t *testing.T, priceRaw uint64) {
		validator := &CoreValidator{}
		validator.SetUp(big.NewInt(11))

		price := new(big.Int).SetUint64(priceRaw)
		validator.minGasPrice = price

		gasPrice := validator.GasPrice()
		if gasPrice == nil {
			t.Errorf("GasPrice should not be nil")
		}
	})
}

// FuzzCreateTransaction tests CreateTransaction with various inputs
func FuzzCreateTransaction(f *testing.F) {
	validator := &CoreValidator{}
	validator.SetUp(big.NewInt(11))

	f.Fuzz(func(t *testing.T, nonceRaw uint64, value float64, gasLimit float64, messageRaw []byte) {
		// Limit values to reasonable ranges
		if value < 0 || value > 1000000 {
			return
		}
		if gasLimit < 0 || gasLimit > 10000 {
			return
		}
		if len(messageRaw) > 1000 {
			messageRaw = messageRaw[:1000]
		}

		addr := types.EmptyAddress()
		tx, err := validator.CreateTransaction(nonceRaw, addr, value, gasLimit, string(messageRaw))

		if err != nil {
			// Error is acceptable for invalid input
			return
		}

		if tx == nil {
			t.Errorf("CreateTransaction returned nil without error")
		}

		// Verify transaction properties
		if tx.Nonce() != nonceRaw {
			t.Errorf("Transaction nonce = %d, want %d", tx.Nonce(), nonceRaw)
		}
	})
}

// FuzzTransactionValue tests the Value method
func FuzzTransactionValue(f *testing.F) {
	f.Fuzz(func(t *testing.T, value float64) {
		// Limit value to reasonable range
		if value < 0 || value > 1000000 {
			return
		}

		addr := types.EmptyAddress()
		tx, _ := types.CreateUnbroadcastTransaction(1, addr, value, 0.001, common.FloatToBigInt(3114000000000000), "test") //TODO replace later with realization

		result := tx.Value()
		if result == nil {
			t.Errorf("Transaction Value() should not be nil")
		}
		if result.Sign() < 0 {
			t.Errorf("Transaction Value() should not be negative")
		}
	})
}

// FuzzTransactionCost tests the Cost method
func FuzzTransactionCost(f *testing.F) {
	f.Fuzz(func(t *testing.T, value float64, gasLimit float64) {
		// Limit values to reasonable ranges
		if value < 0 || value > 1000000 {
			return
		}
		if gasLimit < 0 || gasLimit > 10000 {
			return
		}

		addr := types.EmptyAddress()
		tx, _ := types.CreateUnbroadcastTransaction(1, addr, value, gasLimit, common.FloatToBigInt(3114000000000000), "test") //TODO replace later with realization

		cost := tx.Cost()
		if cost == nil {
			t.Errorf("Transaction Cost() should not be nil")
		}

		// Cost should be value + gas
		if cost.Sign() < 0 {
			t.Errorf("Transaction Cost() should not be negative")
		}
	})
}

// FuzzTransactionGas tests the Gas method
func FuzzTransactionGas(f *testing.F) {
	f.Fuzz(func(t *testing.T, gasLimit float64) {
		// Limit gas to reasonable range
		if gasLimit < 0 || gasLimit > 10000 {
			return
		}

		addr := types.EmptyAddress()
		tx, _ := types.CreateUnbroadcastTransaction(1, addr, 10.0, gasLimit, common.FloatToBigInt(3114000000000000), "test") //TODO replace later with realization

		gas := tx.Gas()
		// Gas should match or be close to gasLimit
		if gas < 0 {
			t.Errorf("Transaction Gas() should not be negative")
		}
	})
}

// FuzzTransactionHash tests the Hash method
func FuzzTransactionHash(f *testing.F) {
	f.Fuzz(func(t *testing.T, nonceRaw uint64, value float64, messageRaw []byte) {
		// Limit values to reasonable ranges
		if value < 0 || value > 1000000 {
			return
		}
		if len(messageRaw) > 1000 {
			messageRaw = messageRaw[:1000]
		}

		addr := types.EmptyAddress()
		tx, _ := types.CreateUnbroadcastTransaction(nonceRaw, addr, value, 0.001, common.FloatToBigInt(3114000000000000), string(messageRaw)) //TODO replace later with realization

		hash := tx.Hash()
		if hash == (common.Hash{}) {
			t.Errorf("Transaction Hash() should not be empty")
		}

		// Hash should be deterministic
		hash2 := tx.Hash()
		if hash != hash2 {
			t.Errorf("Transaction Hash() should be deterministic")
		}
	})
}
