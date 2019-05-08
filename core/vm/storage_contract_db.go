// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"strconv"

	"github.com/DxChainNetwork/godx/common"

	"github.com/DxChainNetwork/godx/storage/storagehost"

	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	PrefixStorageContract       = "storagecontract-"
	PrefixExpireStorageContract = "expirestoragecontract-"
	PrefixStorageObligation     = "storageobligation-"
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

func SplitStorageContractID(key []byte) (uint64, common.Hash) {
	prefixBytes := []byte(PrefixExpireStorageContract)
	if !bytes.HasPrefix(key, prefixBytes) {
		return 0, common.Hash{}
	}

	item := key[len(prefixBytes):]
	sepIndex := bytes.Index(item, []byte("-"))
	heightBytes := item[:sepIndex]
	height, err := strconv.ParseUint(string(heightBytes), 10, 64)
	if err != nil {
		log.Error("failed to parse uint", "height_str", string(heightBytes), "error", err)
		return 0, common.Hash{}
	}

	scIDBytes := item[(sepIndex + 1):]
	var scID common.Hash
	err = rlp.DecodeBytes(scIDBytes, &scID)
	if err != nil {
		log.Error("failed to decode rlp bytes", "error", err)
		return 0, common.Hash{}
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

func GetStorageContract(db ethdb.Database, storageContractID common.Hash) (types.StorageContract, error) {
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
func StoreStorageContract(db ethdb.Database, storageContractID common.Hash, sc types.StorageContract) error {
	return storeWithPrefix(db, storageContractID, sc, PrefixStorageContract)
}

func DeleteStorageContract(db ethdb.Database, storageContractID common.Hash) error {
	return deleteWithPrefix(db, storageContractID, PrefixStorageContract)
}

// StorageContractID ==》[]byte{}
func StoreExpireStorageContract(db ethdb.Database, storageContractID common.Hash, windowEnd uint64) error {
	windowStr := strconv.FormatUint(uint64(windowEnd), 10)
	return storeWithPrefix(db, storageContractID, []byte{}, PrefixExpireStorageContract+windowStr+"-")
}

func DeleteExpireStorageContract(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	heightStr := strconv.FormatUint(uint64(height), 10)
	return deleteWithPrefix(db, storageContractID, PrefixExpireStorageContract+heightStr+"-")
}

func StoreStorageObligation(db ethdb.Database, storageContractID common.Hash, so storagehost.StorageObligation) error {
	return storeWithPrefix(db, storageContractID, so, PrefixStorageObligation)
}

func DeleteStorageObligation(db ethdb.Database, storageContractID common.Hash) error {
	return deleteWithPrefix(db, storageContractID, PrefixStorageObligation)
}

func GetStorageObligation(db ethdb.Database, storageContractID common.Hash) (storagehost.StorageObligation, error) {
	valueBytes, err := getWithPrefix(db, storageContractID, PrefixStorageObligation)
	if err != nil {
		return storagehost.StorageObligation{}, err
	}
	var so storagehost.StorageObligation
	err = rlp.DecodeBytes(valueBytes, &so)
	if err != nil {
		return storagehost.StorageObligation{}, err
	}
	return so, nil
}
