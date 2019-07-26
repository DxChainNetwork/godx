package ethdb

import (
	"github.com/DxChainNetwork/godx/rlp"
)

// MakeKey constructs prefix key for key-value storage
func MakeKey(prefix string, key interface{}) ([]byte, error) {
	keyBytes, err := rlp.EncodeToBytes(key)
	if err != nil {
		return nil, err
	}

	result := []byte(prefix)
	result = append(result, keyBytes...)
	return result, nil
}

// StorageContractDB wraps Database
type StorageContractDB struct {
	DB Database
}

// GetWithPrefix query data by prefix key
func (scdb *StorageContractDB) GetWithPrefix(key interface{}, prefix string) ([]byte, error) {
	keyByPrefix, err := MakeKey(prefix, key)
	if err != nil {
		return nil, err
	}

	value, err := scdb.DB.Get(keyByPrefix)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// StoreWithPrefix store prefix key with value
func (scdb *StorageContractDB) StoreWithPrefix(key interface{}, value []byte, prefix string) error {
	keyByPrefix, err := MakeKey(prefix, key)
	if err != nil {
		return err
	}

	err = scdb.DB.Put(keyByPrefix, value)
	if err != nil {
		return err
	}

	return nil
}

// DeleteWithPrefix remove the value by the prefix key
func (scdb *StorageContractDB) DeleteWithPrefix(key interface{}, prefix string) error {
	keyByPrefix, err := MakeKey(prefix, key)
	if err != nil {
		return err
	}

	err = scdb.DB.Delete(keyByPrefix)
	if err != nil {
		return err
	}

	return nil
}
