package coinbase

import (
	"math/big"
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
	if Coinbase.address.Hex() != AddressHex {
		t.Errorf("Coinbase address mismatch: got %s, want %s", Coinbase.address.Hex(), AddressHex)
	}
	if Faucet.address.Hex() != FaucetAddressHex {
		t.Errorf("Faucet address mismatch: got %s, want %s", Faucet.address.Hex(), FaucetAddressHex)
	}
}

func TestGetCoinbaseAddress(t *testing.T) {
	InitOperationData()
	addr := GetCoinbaseAddress()
	if addr.Hex() != AddressHex {
		t.Errorf("GetCoinbaseAddress returned %s, want %s", addr.Hex(), AddressHex)
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
	if acc.Address.Hex() != AddressHex {
		t.Errorf("CoinBaseStateAccount address mismatch: got %s, want %s", acc.Address.Hex(), AddressHex)
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
	if acc.Address.Hex() != FaucetAddressHex {
		t.Errorf("FaucetAccount address mismatch: got %s, want %s", acc.Address.Hex(), FaucetAddressHex)
	}
}

func TestGetFaucetAddress(t *testing.T) {
	InitOperationData()
	addr := GetFaucetAddress()
	if addr.Hex() != FaucetAddressHex {
		t.Errorf("GetFaucetAddress returned %s, want %s", addr.Hex(), FaucetAddressHex)
	}
}

func TestGetFaucetBalance(t *testing.T) {
	InitOperationData()
	bal := GetFaucetBalance()
	if bal.Cmp(FaucetInitialBalance) != 0 {
		t.Errorf("GetFaucetBalance returned %v, want %v", bal, FaucetInitialBalance)
	}
}

func TestFaucetTransaction(t *testing.T) {
	InitOperationData()
	nonce := uint64(1)
	dest := GetCoinbaseAddress()
	cnt := 100.0
	initial := GetFaucetBalance()
	tx := FaucetTransaction(nonce, dest, cnt)
	expectedSub := new(big.Int).Add(
		types.FloatToBigInt(cnt),
		types.FloatToBigInt(1000),
	)
	after := GetFaucetBalance()
	expectedAfter := new(big.Int).Sub(initial, expectedSub)
	if after.Cmp(expectedAfter) != 0 {
		t.Errorf("Faucet balance after FaucetTransaction: got %v, want %v", after, expectedAfter)
	}
	if *tx.To() != dest {
		t.Errorf("FaucetTransaction to: got %s, want %s", tx.To().Hex(), dest.Hex())
	}
	if tx.Value().Cmp(types.FloatToBigInt(cnt)) != 0 {
		t.Errorf("FaucetTransaction value: got %v, want %v", tx.Value(), types.FloatToBigInt(cnt))
	}
}
