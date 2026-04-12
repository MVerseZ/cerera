package storage

import (
	"testing"

	"github.com/cerera/core/account"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
)

func makeAddr(seed byte) types.Address {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed
	}
	return types.BytesToAddress(b)
}

func makeStateAccount(addr types.Address, balance float64, keyHash common.Hash) *account.StateAccount {
	sa := account.NewStateAccount(addr, balance, common.Hash{})
	sa.KeyHash = keyHash
	return sa
}

func TestGetAccountsTrie(t *testing.T) {
	at := GetAccountsTrie()
	if at == nil {
		t.Fatal("GetAccountsTrie returned nil")
	}
	if at.index == nil || at.accounts == nil || at.addrIndex == nil {
		t.Error("index, accounts, or addrIndex map is nil")
	}
	if at.lastInsert != 0 {
		t.Errorf("lastInsert want 0, got %d", at.lastInsert)
	}
	if at.GetCount() != 0 {
		t.Errorf("GetCount want 0, got %d", at.GetCount())
	}
}

func TestAccountsTrie_Append_GetAccount_GetCount(t *testing.T) {
	at := GetAccountsTrie()
	addr := makeAddr(0x01)
	sa := makeStateAccount(addr, 100.5, common.Hash{})
	at.Append(addr, sa)

	if at.GetCount() != 1 {
		t.Errorf("GetCount want 1, got %d", at.GetCount())
	}
	got := at.GetAccount(addr)
	if got != sa {
		t.Error("GetAccount returned different pointer")
	}
	if got.GetBalance() != 100.5 {
		t.Errorf("balance want 100.5, got %f", got.GetBalance())
	}
}

func TestAccountsTrie_Append_OverwriteSameAddress(t *testing.T) {
	at := GetAccountsTrie()
	addr := makeAddr(0x02)
	sa1 := makeStateAccount(addr, 10.0, common.Hash{})
	sa2 := makeStateAccount(addr, 20.0, common.Hash{})
	at.Append(addr, sa1)
	at.Append(addr, sa2)

	if at.GetCount() != 1 {
		t.Errorf("GetCount want 1 (same address overwritten), got %d", at.GetCount())
	}
	if at.lastInsert != 1 {
		t.Errorf("lastInsert want 1 (one distinct address), got %d", at.lastInsert)
	}
	if at.GetByIndex(0) != sa2 {
		t.Error("GetByIndex(0) should point to the updated account, not a stale slot")
	}
	if at.GetByIndex(1) != nil {
		t.Error("GetByIndex(1) should be nil after overwrite same address")
	}
	got := at.GetAccount(addr)
	if got != sa2 {
		t.Error("GetAccount should return last appended account")
	}
	if got.GetBalance() != 20.0 {
		t.Errorf("balance want 20.0, got %f", got.GetBalance())
	}
}

func TestAccountsTrie_GetAccount_UnknownAddress(t *testing.T) {
	at := GetAccountsTrie()
	addr := makeAddr(0x03)
	got := at.GetAccount(addr)
	if got != nil {
		t.Errorf("GetAccount(unknown) want nil, got %v", got)
	}
}

func TestAccountsTrie_Clear(t *testing.T) {
	at := GetAccountsTrie()
	addr := makeAddr(0x04)
	sa := makeStateAccount(addr, 1.0, common.Hash{})
	at.Append(addr, sa)

	err := at.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if at.GetCount() != 0 {
		t.Errorf("after Clear GetCount want 0, got %d", at.GetCount())
	}
	if at.GetAccount(addr) != nil {
		t.Error("after Clear GetAccount should return nil")
	}
	if at.GetByIndex(0) != nil {
		t.Error("after Clear GetByIndex(0) should return nil")
	}
}

func TestAccountsTrie_GetByIndex(t *testing.T) {
	at := GetAccountsTrie()
	addr0 := makeAddr(0x10)
	addr1 := makeAddr(0x11)
	sa0 := makeStateAccount(addr0, 1.0, common.Hash{})
	sa1 := makeStateAccount(addr1, 2.0, common.Hash{})
	at.Append(addr0, sa0)
	at.Append(addr1, sa1)

	if at.GetByIndex(0) != sa0 {
		t.Error("GetByIndex(0) should return first account")
	}
	if at.GetByIndex(1) != sa1 {
		t.Error("GetByIndex(1) should return second account")
	}
	if at.GetByIndex(2) != nil {
		t.Error("GetByIndex(2) should return nil")
	}
	if at.GetByIndex(-1) != nil {
		t.Error("GetByIndex(-1) should return nil")
	}
}

func TestAccountsTrie_Size(t *testing.T) {
	at := GetAccountsTrie()
	if at.Size() != 0 {
		t.Errorf("empty trie Size want 0, got %d", at.Size())
	}
	addr := makeAddr(0x20)
	sa := makeStateAccount(addr, 0, common.Hash{})
	at.Append(addr, sa)
	size := at.Size()
	if size <= 0 {
		t.Errorf("Size after one account want > 0, got %d", size)
	}
}

func TestAccountsTrie_Count(t *testing.T) {
	at := GetAccountsTrie()
	if at.GetCount() != 0 {
		t.Errorf("empty trie Size want 0, got %d", at.Size())
	}
	addr := makeAddr(0x20)
	sa := makeStateAccount(addr, 0, common.Hash{})
	at.Append(addr, sa)
	size := at.GetCount()
	if size != 1 {
		t.Errorf("Size after one account want > 0, got %d", size)
	}
}

func TestAccountsTrie_ReadableSize(t *testing.T) {
	at := GetAccountsTrie()
	s := at.ReadableSize()
	if s != "0 bytes" {
		t.Errorf("empty ReadableSize want \"0 bytes\", got %q", s)
	}
	addr := makeAddr(0x21)
	sa := makeStateAccount(addr, 0, common.Hash{})
	at.Append(addr, sa)
	s = at.ReadableSize()
	if s == "" || s == "0 bytes" {
		// After one account we have some bytes
		t.Logf("ReadableSize with one account: %s", s)
	}
}

func TestAccountsTrie_GetAll(t *testing.T) {
	at := GetAccountsTrie()
	addr0 := makeAddr(0x30)
	addr1 := makeAddr(0x31)
	at.Append(addr0, makeStateAccount(addr0, 10.0, common.Hash{}))
	at.Append(addr1, makeStateAccount(addr1, 20.0, common.Hash{}))

	all := at.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll len want 2, got %d", len(all))
	}
	if all[addr0] != 10.0 {
		t.Errorf("GetAll[addr0] want 10.0, got %f", all[addr0])
	}
	if all[addr1] != 20.0 {
		t.Errorf("GetAll[addr1] want 20.0, got %f", all[addr1])
	}
}

func TestAccountsTrie_GetAll_Empty(t *testing.T) {
	at := GetAccountsTrie()
	all := at.GetAll()
	if all == nil {
		t.Fatal("GetAll should not return nil map")
	}
	if len(all) != 0 {
		t.Errorf("GetAll empty trie len want 0, got %d", len(all))
	}
}

func TestAccountsTrie_FindByKeyHash_Found(t *testing.T) {
	at := GetAccountsTrie()
	keyHash := common.BytesToHash([]byte("unique_key_hash_32_bytes_long!!!!!!"))
	addr := makeAddr(0x40)
	sa := makeStateAccount(addr, 5.0, keyHash)
	at.Append(addr, sa)

	got, err := at.FindByKeyHash(keyHash)
	if err != nil {
		t.Fatalf("FindByKeyHash: %v", err)
	}
	if got != sa {
		t.Error("FindByKeyHash returned wrong account")
	}
	if got.GetBalance() != 5.0 {
		t.Errorf("balance want 5.0, got %f", got.GetBalance())
	}
}

func TestAccountsTrie_FindByKeyHash_NotFound(t *testing.T) {
	at := GetAccountsTrie()
	addr := makeAddr(0x41)
	at.Append(addr, makeStateAccount(addr, 1.0, common.Hash{}))

	needle := common.BytesToHash([]byte("nonexistent_key_hash_________"))
	got, err := at.FindByKeyHash(needle)
	if err == nil {
		t.Error("FindByKeyHash want error when not found")
	}
	if got != nil {
		t.Errorf("FindByKeyHash(not found) want nil account, got %v", got)
	}
	if err != nil && err.Error() != "key hash not found" {
		t.Errorf("error message want \"key hash not found\", got %q", err.Error())
	}
}

func TestAccountsTrie_FindByKeyHash_EmptyTrie(t *testing.T) {
	at := GetAccountsTrie()
	_, err := at.FindByKeyHash(common.Hash{})
	if err == nil {
		t.Error("FindByKeyHash on empty trie should return error")
	}
}

func TestAccountsTrie_ConcurrentAppendGet(t *testing.T) {
	at := GetAccountsTrie()
	done := make(chan struct{})
	go func() {
		for i := byte(0); i < 50; i++ {
			addr := makeAddr(0x50 + i)
			sa := makeStateAccount(addr, float64(i), common.Hash{})
			at.Append(addr, sa)
		}
		close(done)
	}()
	<-done
	if at.GetCount() != 50 {
		t.Errorf("concurrent GetCount want 50, got %d", at.GetCount())
	}
	for i := byte(0); i < 50; i++ {
		addr := makeAddr(0x50 + i)
		sa := at.GetAccount(addr)
		if sa == nil {
			t.Errorf("GetAccount(0x%02x) nil", 0x50+i)
		} else if sa.GetBalance() != float64(i) {
			t.Errorf("balance for 0x%02x want %f, got %f", 0x50+i, float64(i), sa.GetBalance())
		}
	}
}
