package coinbase

import (
	"math/big"
	"strings"
	"testing"

	"github.com/cerera/internal/cerera/types"
)

func TestReward(t *testing.T) {
	InitOperationData()
	reward := RewardBlock()
	if blockReward.Cmp(reward) != 0 {
		t.Errorf("expected %v, got %v\n", blockReward, reward)
	}
}
func TestInitOperationData(t *testing.T) {
	err := InitOperationData()
	if err != nil {
		t.Fatalf("InitOperationData failed: %v", err)
	}
	expectedCoinbase := types.HexToAddress(AddressHex)
	if Coinbase.address != expectedCoinbase {
		t.Errorf("Coinbase address mismatch: got %s, want %s", Coinbase.address.Hex(), expectedCoinbase.Hex())
	}
	expectedFaucet := types.HexToAddress(FaucetAddressHex)
	if Faucet.address != expectedFaucet {
		t.Errorf("Faucet address mismatch: got %s, want %s", Faucet.address.Hex(), expectedFaucet.Hex())
	}
}

func TestGetCoinbaseAddress(t *testing.T) {
	InitOperationData()
	addr := GetCoinbaseAddress()
	expected := types.HexToAddress(AddressHex)
	if addr != expected {
		t.Errorf("GetCoinbaseAddress returned %s, want %s", addr.Hex(), expected.Hex())
	}
}

func TestGetCoinbaseBalance(t *testing.T) {
	InitOperationData()
	bal := GetCoinbaseBalance()
	if bal.Cmp(TotalValue) != 0 {
		t.Errorf("GetCoinbaseBalance returned %v, want %v", bal, TotalValue)
	}
}

func TestCoinBaseStateAccount(t *testing.T) {
	InitOperationData()
	acc := CoinBaseStateAccount()
	expected := types.HexToAddress(AddressHex)
	if acc.Address != expected {
		t.Errorf("CoinBaseStateAccount address mismatch: got %s, want %s", acc.Address.Hex(), expected.Hex())
	}
}

func TestRewardBlock(t *testing.T) {
	InitOperationData()
	initial := GetCoinbaseBalance()
	reward := RewardBlock()
	expected := blockReward
	if reward.Cmp(expected) != 0 {
		t.Errorf("RewardBlock returned %v, want %v", reward, expected)
	}
	after := GetCoinbaseBalance()
	expectedAfter := new(big.Int).Sub(initial, blockReward)
	if after.Cmp(expectedAfter) != 0 {
		t.Errorf("Coinbase balance after RewardBlock: got %v, want %v", after, expectedAfter)
	}
}

func TestDropFaucet(t *testing.T) {
	InitOperationData()
	initial := GetCoinbaseBalance()
	val := big.NewInt(1000)
	got := DropFaucet(val)
	if got.Cmp(val) != 0 {
		t.Errorf("DropFaucet returned %v, want %v", got, val)
	}
	after := GetCoinbaseBalance()
	expectedAfter := new(big.Int).Sub(initial, val)
	if after.Cmp(expectedAfter) != 0 {
		t.Errorf("Coinbase balance after DropFaucet: got %v, want %v", after, expectedAfter)
	}
}

func TestCreateCoinBaseTransation(t *testing.T) {
	InitOperationData()
	nonce := uint64(42)
	addr := GetCoinbaseAddress()
	tx := CreateCoinBaseTransation(nonce, addr)
	if tx.Nonce() != nonce {
		t.Errorf("CreateCoinBaseTransation nonce: got %d, want %d", tx.Nonce(), nonce)
	}
	if *tx.To() != addr {
		t.Errorf("CreateCoinBaseTransation to: got %s, want %s", tx.To().Hex(), addr.Hex())
	}
	if tx.Value().Cmp(blockReward) != 0 {
		t.Errorf("CreateCoinBaseTransation value: got %v, want %v", tx.Value(), blockReward)
	}
}

func TestFaucetAccount(t *testing.T) {
	InitOperationData()
	acc := FaucetAccount()
	expected := types.HexToAddress(FaucetAddressHex)
	if acc.Address != expected {
		t.Errorf("FaucetAccount address mismatch: got %s, want %s", acc.Address.Hex(), expected.Hex())
	}
}

func TestGetFaucetAddress(t *testing.T) {
	InitOperationData()
	addr := GetFaucetAddress()
	expected := types.HexToAddress(FaucetAddressHex)
	if addr != expected {
		t.Errorf("GetFaucetAddress returned %s, want %s", addr.Hex(), expected.Hex())
	}
}

func TestGetFaucetBalance(t *testing.T) {
	InitOperationData()
	bal := GetFaucetBalance()
	if bal.Cmp(FaucetInitialBalance) != 0 {
		t.Errorf("GetFaucetBalance returned %v, want %v", bal, FaucetInitialBalance)
	}
}

// func TestFaucetTransaction(t *testing.T) {
// 	InitOperationData()
// 	nonce := uint64(1)
// 	dest := GetCoinbaseAddress()
// 	cnt := 100.0
// 	initial := GetFaucetBalance()
// 	tx := FaucetTransaction(nonce, dest)
// 	expectedSub := new(big.Int).Add(
// 		types.FloatToBigInt(cnt),
// 		types.FloatToBigInt(1000),
// 	)
// 	after := GetFaucetBalance()
// 	expectedAfter := new(big.Int).Sub(initial, expectedSub)
// 	if after.Cmp(expectedAfter) != 0 {
// 		t.Errorf("Faucet balance after FaucetTransaction: got %v, want %v", after, expectedAfter)
// 	}
// 	if *tx.To() != dest {
// 		t.Errorf("FaucetTransaction to: got %s, want %s", tx.To().Hex(), dest.Hex())
// 	}
// 	if tx.Value().Cmp(types.FloatToBigInt(cnt)) != 0 {
// 		t.Errorf("FaucetTransaction value: got %v, want %v", tx.Value(), types.FloatToBigInt(cnt))
// 	}
// }

// Test CheckFaucetLimits function
func TestCheckFaucetLimits(t *testing.T) {
	// Initialize coinbase data
	err := InitOperationData()
	if err != nil {
		t.Fatalf("Failed to initialize coinbase data: %v", err)
	}

	testAddr := types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	t.Run("valid_amount", func(t *testing.T) {
		amount := types.FloatToBigInt(100.0)
		err := CheckFaucetLimits(testAddr, amount)
		if err != nil {
			t.Errorf("CheckFaucetLimits() error = %v", err)
		}
	})

	t.Run("amount_below_minimum", func(t *testing.T) {
		amount := types.FloatToBigInt(0.5)
		err := CheckFaucetLimits(testAddr, amount)
		if err == nil {
			t.Error("CheckFaucetLimits() should fail for amount below minimum")
		}
		if !strings.Contains(err.Error(), "below minimum") {
			t.Errorf("CheckFaucetLimits() error message should contain 'below minimum', got: %v", err)
		}
	})

	t.Run("amount_above_maximum", func(t *testing.T) {
		amount := types.FloatToBigInt(2000.0)
		err := CheckFaucetLimits(testAddr, amount)
		if err == nil {
			t.Error("CheckFaucetLimits() should fail for amount above maximum")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("CheckFaucetLimits() error message should contain 'exceeds maximum', got: %v", err)
		}
	})

	t.Run("nil_amount", func(t *testing.T) {
		err := CheckFaucetLimits(testAddr, nil)
		if err == nil {
			t.Error("CheckFaucetLimits() should fail for nil amount")
		}
		if !strings.Contains(err.Error(), "invalid faucet amount") {
			t.Errorf("CheckFaucetLimits() error message should contain 'invalid faucet amount', got: %v", err)
		}
	})

	t.Run("zero_amount", func(t *testing.T) {
		amount := big.NewInt(0)
		err := CheckFaucetLimits(testAddr, amount)
		if err == nil {
			t.Error("CheckFaucetLimits() should fail for zero amount")
		}
		if !strings.Contains(err.Error(), "invalid faucet amount") {
			t.Errorf("CheckFaucetLimits() error message should contain 'invalid faucet amount', got: %v", err)
		}
	})
}

// Test RecordFaucetRequest function
func TestRecordFaucetRequest(t *testing.T) {
	testAddr := types.HexToAddress("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	amount := types.FloatToBigInt(100.0)

	// Record a request
	RecordFaucetRequest(testAddr, amount)

	// Check cooldown remaining
	cooldown := GetFaucetCooldownRemaining(testAddr)
	if cooldown <= 0 {
		t.Error("GetFaucetCooldownRemaining() should return positive value after recording request")
	}
}

// Test GetFaucetCooldownRemaining function
func TestGetFaucetCooldownRemaining(t *testing.T) {
	// Use a different address to avoid interference from other tests
	testAddr := types.HexToAddress("0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba")

	t.Run("no_previous_request", func(t *testing.T) {
		cooldown := GetFaucetCooldownRemaining(testAddr)
		if cooldown != 0 {
			t.Errorf("GetFaucetCooldownRemaining() = %v, want 0", cooldown)
		}
	})

	t.Run("recent_request", func(t *testing.T) {
		amount := types.FloatToBigInt(100.0)
		RecordFaucetRequest(testAddr, amount)

		cooldown := GetFaucetCooldownRemaining(testAddr)
		if cooldown <= 0 {
			t.Error("GetFaucetCooldownRemaining() should return positive value for recent request")
		}
	})
}
