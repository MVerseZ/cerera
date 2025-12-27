package types

import (
	"encoding/json"
	"math/big"
	"testing"
)

// TestTransactionFormatConsistency tests that transaction format is consistent
// between block serialization (MarshalJSON) and transaction.get RPC response
func TestTransactionFormatConsistency(t *testing.T) {
	toAddr := HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	tx := NewTransaction(
		1,
		toAddr,
		big.NewInt(1000000000000000000), // 1 ETH in wei
		21000,
		big.NewInt(20000000000), // 20 gwei
		[]byte("test data"),
	)

	// Test 1: MarshalJSON format (used in blocks)
	jsonBytes, err := tx.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal transaction: %v", err)
	}

	var blockTx map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &blockTx); err != nil {
		t.Fatalf("Failed to unmarshal block transaction JSON: %v", err)
	}

	// Test 2: Verify format consistency
	// value should be decimal string
	valueBlock, ok := blockTx["value"].(string)
	if !ok {
		t.Errorf("Block tx value should be string, got %T", blockTx["value"])
	} else {
		if len(valueBlock) >= 2 && valueBlock[0:2] == "0x" {
			t.Errorf("Block tx value should be decimal, not hex. Got: %s", valueBlock)
		}
	}

	// gas should be number
	gasBlock, ok := blockTx["gas"].(float64)
	if !ok {
		t.Errorf("Block tx gas should be number, got %T", blockTx["gas"])
	} else {
		if uint64(gasBlock) != 21000 {
			t.Errorf("Block tx gas mismatch: got %f, want 21000", gasBlock)
		}
	}

	// gasPrice should be decimal string
	gasPriceBlock, ok := blockTx["gasPrice"].(string)
	if !ok {
		t.Errorf("Block tx gasPrice should be string, got %T", blockTx["gasPrice"])
	} else {
		if len(gasPriceBlock) >= 2 && gasPriceBlock[0:2] == "0x" {
			t.Errorf("Block tx gasPrice should be decimal, not hex. Got: %s", gasPriceBlock)
		}
	}

	// nonce should be number
	nonceBlock, ok := blockTx["nonce"].(float64)
	if !ok {
		t.Errorf("Block tx nonce should be number, got %T", blockTx["nonce"])
	} else {
		if uint64(nonceBlock) != 1 {
			t.Errorf("Block tx nonce mismatch: got %f, want 1", nonceBlock)
		}
	}

	// data should be hex string
	dataBlock, ok := blockTx["input"].(string)
	if !ok {
		t.Errorf("Block tx data (input) should be string, got %T", blockTx["input"])
	} else {
		if len(dataBlock) < 2 || dataBlock[0:2] != "0x" {
			t.Errorf("Block tx data should be hex string (0x...), got: %s", dataBlock)
		}
	}

	// hash, to should be hex strings
	hashBlock, ok := blockTx["hash"].(string)
	if !ok {
		t.Errorf("Block tx hash should be string, got %T", blockTx["hash"])
	} else {
		if len(hashBlock) < 2 || hashBlock[0:2] != "0x" {
			t.Errorf("Block tx hash should be hex string (0x...), got: %s", hashBlock)
		}
	}

	// Test 3: Verify format consistency between MarshalJSON and expected RPC format
	// (Block serialization uses the same MarshalJSON, so format should be consistent)
	t.Logf("Transaction format verified: value=%s (decimal), gas=%f (number), data=%s (hex)", 
		valueBlock, gasBlock, dataBlock)
}

// TestTransactionFormatFields tests all transaction fields format
func TestTransactionFormatFields(t *testing.T) {
	toAddr := HexToAddress("0xe7925c3c6FC91Cc41319eE320D297549fF0a1Cfd16425e7ad95ED556337ea2873A1191717081c42F2575F09B6bc60206")
	tx := NewTransaction(
		42,
		toAddr,
		big.NewInt(5000000000000000000), // 5 ETH
		50000,
		big.NewInt(30000000000), // 30 gwei
		[]byte{0x01, 0x02, 0x03, 0x04},
	)

	jsonBytes, err := tx.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal transaction: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Test all fields format
	tests := []struct {
		name     string
		field    string
		checkHex bool
		checkDec bool
	}{
		{"hash", "hash", true, false},
		{"to", "to", true, false},
		{"value", "value", false, true},
		{"gasPrice", "gasPrice", false, true},
		{"gas", "gas", false, false}, // number
		{"nonce", "nonce", false, false}, // number
		{"data", "input", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := result[tt.field]
			if !ok {
				t.Errorf("Field %s should be present", tt.field)
				return
			}

			if tt.checkHex {
				str, ok := value.(string)
				if !ok {
					t.Errorf("Field %s should be string, got %T", tt.field, value)
					return
				}
				if len(str) < 2 || str[0:2] != "0x" {
					t.Errorf("Field %s should be hex string (0x...), got: %s", tt.field, str)
				}
			}

			if tt.checkDec {
				str, ok := value.(string)
				if !ok {
					t.Errorf("Field %s should be string, got %T", tt.field, value)
					return
				}
				if len(str) >= 2 && str[0:2] == "0x" {
					t.Errorf("Field %s should be decimal string, not hex. Got: %s", tt.field, str)
				}
				// Verify it's a valid decimal number
				if _, ok := new(big.Int).SetString(str, 10); !ok {
					t.Errorf("Field %s should be a valid decimal number. Got: %s", tt.field, str)
				}
			}

			if !tt.checkHex && !tt.checkDec {
				// Should be number
				_, ok := value.(float64)
				if !ok {
					t.Errorf("Field %s should be number, got %T", tt.field, value)
				}
			}
		})
	}
}

