package ethdb

import (
	"bytes"
	"testing"
)

func TestMakeKey(t *testing.T) {
	prefix := "hello-"
	key := "0x123"
	keyBytes, err := MakeKey(prefix, key)
	if err != nil {
		t.Errorf("failed to make key: %v", err)
	}

	if !bytes.Contains(keyBytes, []byte(prefix)) {
		t.Errorf("failed to make key")
	}
}

func TestStorageContractDBOperation(t *testing.T) {
	db := NewMemDatabase()
	scdb := StorageContractDB{db}

	key := "0x123"
	value := "123"
	prefix := "test-"

	// store
	err := scdb.StoreWithPrefix(key, []byte(value), prefix)
	if err != nil {
		t.Errorf("something wrong to store key-value: %v", err)
	}

	// query
	gettedValue, err := scdb.GetWithPrefix(key, prefix)
	if err != nil {
		t.Errorf("something wrong to query key-value: %v", err)
	}

	if value != string(gettedValue) {
		t.Errorf("wanted %s, getted %s", value, string(gettedValue))
	}

	// delete
	err = scdb.DeleteWithPrefix(key, prefix)
	if err != nil {
		t.Errorf("something wrong to delete key-value: %v", err)
	}

	queryAgain, _ := scdb.GetWithPrefix(key, prefix)
	if queryAgain != nil {
		t.Errorf("failed to delete key: %v", key)
	}
}
