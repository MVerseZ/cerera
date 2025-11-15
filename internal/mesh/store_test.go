package mesh

import (
	"bytes"
	"testing"
	"time"
)

func TestMemoryStore_Init(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	if ms.data == nil {
		t.Error("Expected data map to be initialized")
	}
	if ms.mutex == nil {
		t.Error("Expected mutex to be initialized")
	}
	if ms.replicateMap == nil {
		t.Error("Expected replicateMap to be initialized")
	}
	if ms.expireMap == nil {
		t.Error("Expected expireMap to be initialized")
	}
}

func TestMemoryStore_Store(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	key := []byte("test-key")
	data := []byte("test-data")
	replication := time.Now().Add(time.Hour)
	expiration := time.Now().Add(2 * time.Hour)

	err := ms.Store(key, data, replication, expiration, true)
	if err != nil {
		t.Errorf("Store returned error: %v", err)
	}

	// Verify data was stored
	retrieved, found := ms.Retrieve(key)
	if !found {
		t.Error("Expected data to be found")
	}
	if !bytes.Equal(retrieved, data) {
		t.Errorf("Expected data %v, got %v", data, retrieved)
	}
}

func TestMemoryStore_Retrieve(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	key := []byte("test-key")
	data := []byte("test-data")
	replication := time.Now().Add(time.Hour)
	expiration := time.Now().Add(2 * time.Hour)

	// Retrieve non-existent key
	_, found := ms.Retrieve(key)
	if found {
		t.Error("Expected key to not be found")
	}

	// Store and retrieve
	ms.Store(key, data, replication, expiration, true)
	retrieved, found := ms.Retrieve(key)
	if !found {
		t.Error("Expected key to be found")
	}
	if !bytes.Equal(retrieved, data) {
		t.Errorf("Expected data %v, got %v", data, retrieved)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	key := []byte("test-key")
	data := []byte("test-data")
	replication := time.Now().Add(time.Hour)
	expiration := time.Now().Add(2 * time.Hour)

	// Store data
	ms.Store(key, data, replication, expiration, true)

	// Verify it exists
	_, found := ms.Retrieve(key)
	if !found {
		t.Error("Expected key to exist before deletion")
	}

	// Delete
	ms.Delete(key)

	// Verify it's gone
	_, found = ms.Retrieve(key)
	if found {
		t.Error("Expected key to be deleted")
	}
}

func TestMemoryStore_GetKey(t *testing.T) {
	ms := &MemoryStore{}

	data1 := []byte("test-data")
	data2 := []byte("test-data")
	data3 := []byte("different-data")

	key1 := ms.GetKey(data1)
	key2 := ms.GetKey(data2)
	key3 := ms.GetKey(data3)

	// Same data should produce same key
	if !bytes.Equal(key1, key2) {
		t.Error("Expected same data to produce same key")
	}

	// Different data should produce different key
	if bytes.Equal(key1, key3) {
		t.Error("Expected different data to produce different key")
	}

	// Key should be 20 bytes (SHA1)
	if len(key1) != 20 {
		t.Errorf("Expected key length 20, got %d", len(key1))
	}
}

func TestMemoryStore_GetAllKeysForReplication(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	key1 := []byte("key1")
	key2 := []byte("key2")
	key3 := []byte("key3")
	data := []byte("data")

	// Store keys with different replication times
	ms.Store(key1, data, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true)   // Past replication time
	ms.Store(key2, data, time.Now().Add(-time.Minute), time.Now().Add(time.Hour), true) // Past replication time
	ms.Store(key3, data, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour), true)  // Future replication time

	keys := ms.GetAllKeysForReplication()

	// Should return keys with past replication time
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys for replication, got %d", len(keys))
	}

	// Verify correct keys are returned
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[string(k)] = true
	}

	if !keyMap["key1"] {
		t.Error("Expected key1 to be in replication list")
	}
	if !keyMap["key2"] {
		t.Error("Expected key2 to be in replication list")
	}
	if keyMap["key3"] {
		t.Error("Expected key3 NOT to be in replication list")
	}
}

func TestMemoryStore_ExpireKeys(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	key1 := []byte("key1")
	key2 := []byte("key2")
	data := []byte("data")

	// Store keys with different expiration times
	ms.Store(key1, data, time.Now().Add(time.Hour), time.Now().Add(-time.Hour), true) // Expired
	ms.Store(key2, data, time.Now().Add(time.Hour), time.Now().Add(time.Hour), true)  // Not expired

	// Verify both exist
	_, found1 := ms.Retrieve(key1)
	_, found2 := ms.Retrieve(key2)
	if !found1 || !found2 {
		t.Error("Expected both keys to exist before expiration")
	}

	// Expire keys
	ms.ExpireKeys()

	// Verify expired key is gone
	_, found1 = ms.Retrieve(key1)
	if found1 {
		t.Error("Expected key1 to be expired")
	}

	// Verify non-expired key still exists
	_, found2 = ms.Retrieve(key2)
	if !found2 {
		t.Error("Expected key2 to still exist")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ms := &MemoryStore{}
	ms.Init()

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := []byte{byte(idx)}
			data := []byte{byte(idx), byte(idx)}
			replication := time.Now().Add(time.Hour)
			expiration := time.Now().Add(2 * time.Hour)
			ms.Store(key, data, replication, expiration, true)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all keys were stored
	for i := 0; i < 10; i++ {
		key := []byte{byte(i)}
		_, found := ms.Retrieve(key)
		if !found {
			t.Errorf("Expected key %d to be found", i)
		}
	}
}
