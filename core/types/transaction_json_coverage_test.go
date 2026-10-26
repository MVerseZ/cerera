package types

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestUnifiedTransactionJSONValidationAndTypes(t *testing.T) {
	base := map[string]any{
		"type": LegacyTxType, "to": "0x1", "nonce": 1, "gas": 21000,
		"gasPrice": "1", "value": "2", "input": "0x", "payload": "0x", "dna": "0x",
	}
	encoded, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	var legacy GTransaction
	if err := json.Unmarshal(encoded, &legacy); err != nil {
		t.Fatalf("valid legacy transaction JSON failed: %v", err)
	}
	if legacy.Type() != LegacyTxType || legacy.Gas() != 21000 {
		t.Fatalf("decoded legacy transaction has type=%d gas=%d", legacy.Type(), legacy.Gas())
	}

	for _, field := range []string{"type", "to", "nonce", "gasPrice", "gas", "value", "input", "payload", "dna"} {
		t.Run("missing_"+field, func(t *testing.T) {
			invalid := make(map[string]any, len(base))
			for key, value := range base {
				invalid[key] = value
			}
			delete(invalid, field)
			data, err := json.Marshal(invalid)
			if err != nil {
				t.Fatal(err)
			}
			var tx GTransaction
			if err := json.Unmarshal(data, &tx); err == nil {
				t.Errorf("transaction without %q was accepted", field)
			}
		})
	}

	unsupported := map[string]any{"type": 99}
	data, _ := json.Marshal(unsupported)
	var tx GTransaction
	if err := json.Unmarshal(data, &tx); err != ErrTxTypeNotSupported {
		t.Fatalf("unsupported type error = %v, want %v", err, ErrTxTypeNotSupported)
	}

	faucet := NewFaucetTransaction(2, HexToAddress("0x2"), big.NewInt(3))
	faucetJSON, err := json.Marshal(faucet)
	if err != nil {
		t.Fatal(err)
	}
	var decodedFaucet GTransaction
	if err := json.Unmarshal(faucetJSON, &decodedFaucet); err != nil || decodedFaucet.Type() != FaucetTxType {
		t.Fatalf("faucet JSON round trip failed: type=%d err=%v", decodedFaucet.Type(), err)
	}
	var legacyFaucet GTransaction
	if err := json.Unmarshal([]byte(`{"type":"0x6","to":"0x02","nonce":"0x2","gas":"0x3","gasPrice":"0x1","value":"0x4"}`), &legacyFaucet); err != nil || legacyFaucet.Type() != FaucetTxType {
		t.Fatalf("legacy faucet JSON round trip failed: type=%d err=%v", legacyFaucet.Type(), err)
	}

	coinbase := NewCoinBaseTransaction(3, HexToAddress("0x3"), big.NewInt(4), 500, big.NewInt(1), []byte("coinbase"))
	coinbaseJSON, err := json.Marshal(coinbase)
	if err != nil {
		t.Fatal(err)
	}
	var decodedCoinbase GTransaction
	if err := json.Unmarshal(coinbaseJSON, &decodedCoinbase); err != nil || decodedCoinbase.Type() != CoinbaseTxType {
		t.Fatalf("coinbase JSON round trip failed: type=%d err=%v", decodedCoinbase.Type(), err)
	}

}
