package mesh

import (
	"bytes"
	"math/big"
	"testing"
	"time"
)

func TestGetBucketIndexFromDifferingBit(t *testing.T) {
	tests := []struct {
		name     string
		id1      []byte
		id2      []byte
		expected int
	}{
		{
			name:     "same IDs",
			id1:      make([]byte, 20),
			id2:      make([]byte, 20),
			expected: 0, // Should return 0 for identical IDs
		},
		{
			name: "differing in first bit",
			id1:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			id2:  []byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // First bit (bit 0) differs
			// byteIndex = 0*8 = 0, bitIndex = 0, result = 160 - (0 + 0) - 1 = 159
			expected: 159,
		},
		{
			name: "differing in last bit",
			id1:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			id2:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // Last bit (bit 7 of last byte) differs
			// byteIndex = 19*8 = 152, bitIndex = 7, result = 160 - (152 + 7) - 1 = 0
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pad IDs to 20 bytes if needed
			id1 := make([]byte, 20)
			id2 := make([]byte, 20)
			copy(id1, tt.id1)
			copy(id2, tt.id2)

			result := getBucketIndexFromDifferingBit(id1, id2)
			if result != tt.expected {
				t.Errorf("Expected bucket index %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestHasBit(t *testing.T) {
	tests := []struct {
		name     string
		n        byte
		pos      uint
		expected bool
	}{
		{
			name:     "bit 0 set",
			n:        0b10000000,
			pos:      0,
			expected: true,
		},
		{
			name:     "bit 0 not set",
			n:        0b01111111,
			pos:      0,
			expected: false,
		},
		{
			name:     "bit 7 set",
			n:        0b00000001,
			pos:      7,
			expected: true,
		},
		{
			name:     "bit 7 not set",
			n:        0b11111110,
			pos:      7,
			expected: false,
		},
		{
			name:     "bit 3 set",
			n:        0b00010000,
			pos:      3,
			expected: true,
		},
		{
			name:     "all bits set",
			n:        0b11111111,
			pos:      4,
			expected: true,
		},
		{
			name:     "no bits set",
			n:        0b00000000,
			pos:      4,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasBit(tt.n, tt.pos)
			if result != tt.expected {
				t.Errorf("Expected hasBit(%b, %d) = %v, got %v", tt.n, tt.pos, tt.expected, result)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	// Test that newID generates valid IDs
	id1, err := newID()
	if err != nil {
		t.Fatalf("newID returned error: %v", err)
	}

	if len(id1) != 20 {
		t.Errorf("Expected ID length 20, got %d", len(id1))
	}

	// Generate another ID and verify they're different (very unlikely to be same)
	id2, err := newID()
	if err != nil {
		t.Fatalf("newID returned error: %v", err)
	}

	if bytes.Equal(id1, id2) {
		t.Error("Expected two generated IDs to be different")
	}

	// Verify ID is not all zeros
	allZeros := true
	for _, b := range id1 {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("Generated ID should not be all zeros")
	}
}

func TestHashTable_GetDistance(t *testing.T) {
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	ht, err := newHashTable(options)
	if err != nil {
		t.Fatalf("newHashTable returned error: %v", err)
	}

	tests := []struct {
		name     string
		id1      []byte
		id2      []byte
		expected *big.Int
	}{
		{
			name:     "same IDs",
			id1:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			id2:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			expected: big.NewInt(0),
		},
		{
			name: "different first byte",
			id1:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			id2:  []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			// hashTable.getDistance XORs all 20 bytes and interprets as big-endian number
			// 0x01 0x00 ... 0x00 = 2^152 (very large number)
			expected: new(big.Int).SetBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pad to 20 bytes
			id1 := make([]byte, 20)
			id2 := make([]byte, 20)
			copy(id1, tt.id1)
			copy(id2, tt.id2)

			result := ht.getDistance(id1, id2)
			if result.Cmp(tt.expected) != 0 {
				t.Errorf("Expected distance %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHashTable_TotalNodes(t *testing.T) {
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	ht, err := newHashTable(options)
	if err != nil {
		t.Fatalf("newHashTable returned error: %v", err)
	}

	// Initially should have 0 nodes
	if ht.totalNodes() != 0 {
		t.Errorf("Expected 0 nodes initially, got %d", ht.totalNodes())
	}
}

func TestHashTable_GetTotalNodesInBucket(t *testing.T) {
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	ht, err := newHashTable(options)
	if err != nil {
		t.Fatalf("newHashTable returned error: %v", err)
	}

	// All buckets should be empty initially
	for i := 0; i < b; i++ {
		count := ht.getTotalNodesInBucket(i)
		if count != 0 {
			t.Errorf("Expected bucket %d to have 0 nodes, got %d", i, count)
		}
	}
}

func TestHashTable_DoesNodeExistInBucket(t *testing.T) {
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	ht, err := newHashTable(options)
	if err != nil {
		t.Fatalf("newHashTable returned error: %v", err)
	}

	nodeID := []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	bucketIndex := getBucketIndexFromDifferingBit(ht.Self.ID, nodeID)

	// Node should not exist initially
	if ht.doesNodeExistInBucket(bucketIndex, nodeID) {
		t.Error("Expected node to not exist in bucket")
	}
}

func TestHashTable_ResetRefreshTimeForBucket(t *testing.T) {
	options := &Options{
		ID:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: "3000",
	}

	ht, err := newHashTable(options)
	if err != nil {
		t.Fatalf("newHashTable returned error: %v", err)
	}

	// Reset refresh time for bucket 0
	ht.resetRefreshTimeForBucket(0)
	refreshTime := ht.getRefreshTimeForBucket(0)

	// Refresh time should be recent (within last second)
	now := time.Now()
	diff := now.Sub(refreshTime)
	if diff < 0 || diff > time.Second {
		t.Errorf("Expected refresh time to be recent, got diff: %v", diff)
	}
}
