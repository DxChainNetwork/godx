// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package vm

import (
	"bytes"
	"strconv"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/rlp"
)

const (
	PrefixStorageContract       = "storagecontract-"
	PrefixExpireStorageContract = "expirestoragecontract-"
)

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

func GetStorageContract(db ethdb.Database, storageContractID common.Hash) (types.StorageContract, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(storageContractID, PrefixStorageContract)
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
	scdb := ethdb.StorageContractDB{db}
	data, err := rlp.EncodeToBytes(sc)
	if err != nil {
		return err
	}
	return scdb.StoreWithPrefix(storageContractID, data, PrefixStorageContract)
}

func DeleteStorageContract(db ethdb.Database, storageContractID common.Hash) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, PrefixStorageContract)
}

// StorageContractID ==》[]byte{}
func StoreExpireStorageContract(db ethdb.Database, storageContractID common.Hash, windowEnd uint64) error {
	windowStr := strconv.FormatUint(uint64(windowEnd), 10)
	scdb := ethdb.StorageContractDB{db}
	return scdb.StoreWithPrefix(storageContractID, []byte{}, PrefixExpireStorageContract+windowStr+"-")
}

func DeleteExpireStorageContract(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	heightStr := strconv.FormatUint(uint64(height), 10)
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, PrefixExpireStorageContract+heightStr+"-")
}
