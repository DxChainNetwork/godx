package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/ethdb"
	"github.com/DxChainNetwork/godx/rlp"
)

// openDB opens the db specified by path. If the db file not exist, create a new one
func openDB(path string) (*ethdb.LDBDatabase, error) {
	// open / create a new db
	// TODO: What is the params here? Is using an ethdb necessary?
	return ethdb.NewLDBDatabase(path, 0, 0)
}

//putStorageResponsibility storage storageResponsibility from DB
func putStorageResponsibility(db ethdb.Database, storageContractID common.Hash, so StorageResponsibility) error {
	scdb := ethdb.StorageContractDB{db}
	data, err := rlp.EncodeToBytes(so)
	if err != nil {
		return err
	}
	return scdb.StoreWithPrefix(storageContractID, data, prefixStorageResponsibility)
}

func (h *StorageHost) deleteStorageResponsibilities(soids []common.Hash) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	for _, soid := range soids {
		err := deleteStorageResponsibility(h.db, soid)
		if err != nil {
			return err
		}
	}
	return nil
}

//deleteStorageResponsibility delete storageResponsibility from DB
func deleteStorageResponsibility(db ethdb.Database, storageContractID common.Hash) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(storageContractID, prefixStorageResponsibility)
}

//getStorageResponsibility get storageResponsibility from DB
func getStorageResponsibility(db ethdb.Database, storageContractID common.Hash) (StorageResponsibility, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(storageContractID, prefixStorageResponsibility)
	if err != nil {
		return StorageResponsibility{}, err
	}
	var so StorageResponsibility
	err = rlp.DecodeBytes(valueBytes, &so)
	if err != nil {
		return StorageResponsibility{}, err
	}
	return so, nil
}

//StoreHeight storage task by block height
func StoreHeight(db ethdb.Database, storageContractID common.Hash, height uint64) error {
	scdb := ethdb.StorageContractDB{db}

	existingItems, err := GetHeight(db, height)
	if err != nil {
		existingItems = make([]byte, 0)
	}

	existingItems = append(existingItems, storageContractID[:]...)

	return scdb.StoreWithPrefix(height, existingItems, prefixHeight)
}

//DeleteHeight delete task by block height
func DeleteHeight(db ethdb.Database, height uint64) error {
	scdb := ethdb.StorageContractDB{db}
	return scdb.DeleteWithPrefix(height, prefixHeight)
}

//GetHeight get the task by block height
func GetHeight(db ethdb.Database, height uint64) ([]byte, error) {
	scdb := ethdb.StorageContractDB{db}
	valueBytes, err := scdb.GetWithPrefix(height, prefixHeight)
	if err != nil {
		return nil, err
	}

	return valueBytes, nil
}
