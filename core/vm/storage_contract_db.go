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

// 构造合约存储键值对的 key
func makeKey(prefix string, key []byte) []byte {
	result := []byte(prefix)
	result = append(result, key...)
	return result
}

func SplitFileContractID(key []byte) (uint64, types.StorageContractID) {
	prefixBytes := []byte(PrefixExpireStorageContract)
	if !bytes.HasPrefix(key, prefixBytes) {
		return 0, types.StorageContractID{}
	}

	item := key[len(prefixBytes):]
	heightBytes := item[:8]
	height, err := strconv.ParseUint(string(heightBytes), 10, 64)
	if err != nil {
		log.Error("failed to parse uint", "height_str", string(heightBytes), "error", err)
		return 0, types.StorageContractID{}
	}

	scIDBytes := item[9:]
	var scID types.StorageContractID
	err = rlp.DecodeBytes(scIDBytes, &scID)
	if err != nil {
		log.Error("failed to decode rlp bytes", "error", err)
		return 0, types.StorageContractID{}
	}

	return height, scID
}

func getWithPrefix(db ethdb.Database, key interface{}, prefix string) ([]byte, error) {
	keyBytes, err := rlp.EncodeToBytes(key)
	if err != nil {
		return nil, err
	}

	keyByPrefix := makeKey(prefix, keyBytes)

	value, err := db.Get(keyByPrefix)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func storeWithPrefix(db ethdb.Database, key, value interface{}, prefix string) error {
	keyBytes, err := rlp.EncodeToBytes(key)
	if err != nil {
		return err
	}

	keyByPrefix := makeKey(prefix, keyBytes)

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
	keyBytes, err := rlp.EncodeToBytes(key)
	if err != nil {
		return err
	}

	keyByPrefix := makeKey(prefix, keyBytes)

	err = db.Delete(keyByPrefix)
	if err != nil {
		return err
	}

	return nil
}

func GetFileContract(db ethdb.Database, storageContractID types.StorageContractID) (types.StorageContract, error) {
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

// 合约DB存储：FileContractID ==》FileContract
func StoreFileContract(db ethdb.Database, storageContractID types.StorageContractID, sc types.StorageContract) error {
	return storeWithPrefix(db, storageContractID, sc, PrefixStorageContract)
}

func DeleteFileContract(db ethdb.Database, storageContractID types.StorageContractID) error {
	return deleteWithPrefix(db, storageContractID, PrefixStorageContract)
}

// 过期合约DB只是存储：FileContractID ==》[]byte{}
func StoreExpireFileContract(db ethdb.Database, storageContractID types.StorageContractID, windowEnd types.BlockHeight) error {
	windowStr := strconv.FormatUint(uint64(windowEnd), 10)
	return storeWithPrefix(db, storageContractID, []byte{}, PrefixExpireStorageContract+windowStr+"-")
}

func DeleteExpireFileContract(db ethdb.Database, storageContractID types.StorageContractID, height types.BlockHeight) error {
	heightStr := strconv.FormatUint(uint64(height), 10)
	return deleteWithPrefix(db, storageContractID, PrefixExpireStorageContract+heightStr+"-")
}
