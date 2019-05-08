// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"strconv"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	PrefixStorageContract       = "storagecontract-"
	PrefixExpireStorageContract = "expirestoragecontract-"
)

// make key for key-value storage
func makeKey(prefix string, key interface{}) ([]byte, error) {
	keyBytes, err := rlp.EncodeToBytes(key)
	if err != nil {
		return nil, err
	}

	result := []byte(prefix)
	result = append(result, keyBytes...)
	return result, nil
}

func SplitStorageContractID(key []byte) (uint64, types.StorageContractID) {
	prefixBytes := []byte(PrefixExpireStorageContract)
	if !bytes.HasPrefix(key, prefixBytes) {
		return 0, types.StorageContractID{}
	}

	item := key[len(prefixBytes):]
	sepIndex := bytes.Index(item, []byte("-"))
	heightBytes := item[:sepIndex]
	height, err := strconv.ParseUint(string(heightBytes), 10, 64)
	if err != nil {
		log.Error("failed to parse uint", "height_str", string(heightBytes), "error", err)
		return 0, types.StorageContractID{}
	}

	scIDBytes := item[(sepIndex + 1):]
	var scID types.StorageContractID
	err = rlp.DecodeBytes(scIDBytes, &scID)
	if err != nil {
		log.Error("failed to decode rlp bytes", "error", err)
		return 0, types.StorageContractID{}
	}

	return height, scID
}

func getWithPrefix(db ethdb.Database, key interface{}, prefix string) ([]byte, error) {
	keyByPrefix, err := makeKey(prefix, key)
	if err != nil {
		return nil, err
	}

	value, err := db.Get(keyByPrefix)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func storeWithPrefix(db ethdb.Database, key, value interface{}, prefix string) error {
	keyByPrefix, err := makeKey(prefix, key)
	if err != nil {
		return err
	}

	valueBytes, err := rlp.EncodeToBytes(value)
	if err != nil {
		return err
	}

	err = db.Put(keyByPrefix, valueBytes)
	if err != nil {
		return err
	}

	return nil
}

func deleteWithPrefix(db ethdb.Database, key interface{}, prefix string) error {
	keyByPrefix, err := makeKey(prefix, key)
	if err != nil {
		return err
	}

	err = db.Delete(keyByPrefix)
	if err != nil {
		return err
	}

	return nil
}

func GetStorageContract(db ethdb.Database, storageContractID types.StorageContractID) (types.StorageContract, error) {
	valueBytes, err := getWithPrefix(db, storageContractID, PrefixStorageContract)
	if err != nil {
		return types.StorageContract{}, err
	}

	var fc types.StorageContract
	err = rlp.DecodeBytes(valueBytes, &fc)
	if err != nil {
		return types.StorageContract{}, err
	}
	return fc, nil
}

// StorageContractID ==》StorageContract
func StoreStorageContract(db ethdb.Database, storageContractID types.StorageContractID, sc types.StorageContract) error {
	return storeWithPrefix(db, storageContractID, sc, PrefixStorageContract)
}

func DeleteStorageContract(db ethdb.Database, storageContractID types.StorageContractID) error {
	return deleteWithPrefix(db, storageContractID, PrefixStorageContract)
}

// StorageContractID ==》[]byte{}
func StoreExpireStorageContract(db ethdb.Database, storageContractID types.StorageContractID, windowEnd types.BlockHeight) error {
	windowStr := strconv.FormatUint(uint64(windowEnd), 10)
	return storeWithPrefix(db, storageContractID, []byte{}, PrefixExpireStorageContract+windowStr+"-")
}

func DeleteExpireStorageContract(db ethdb.Database, storageContractID types.StorageContractID, height types.BlockHeight) error {
	heightStr := strconv.FormatUint(uint64(height), 10)
	return deleteWithPrefix(db, storageContractID, PrefixExpireStorageContract+heightStr+"-")
}
