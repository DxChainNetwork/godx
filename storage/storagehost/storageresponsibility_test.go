package storagehost

import (
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/ethdb"
)

func TestStoreStorageResponsibility(t *testing.T) {
	db := ethdb.NewMemDatabase()
	defer db.Close()
	so := StorageResponsibility{
		OriginStorageContract: types.StorageContract{
			WindowStart:    1,
			RevisionNumber: 1,
			WindowEnd:      144,
		},
	}

	err := putStorageResponsibility(db, so.OriginStorageContract.RLPHash(), so)
	if err != nil {
		t.Error(err)
	}

	sos, err := getStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}

	if sos.OriginStorageContract.WindowEnd != 144 || sos.OriginStorageContract.WindowStart != 1 || sos.OriginStorageContract.RevisionNumber != 1 {
		t.Error("DB persistence error")
	}

	err = deleteStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}
}

func TestStoreHeight(t *testing.T) {
	db := ethdb.NewMemDatabase()
	var height uint64
	height = 10
	defer db.Close()
	sc1 := types.StorageContract{
		WindowStart:    1,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	sc2 := types.StorageContract{
		WindowStart:    2,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	sc3 := types.StorageContract{
		WindowStart:    3,
		RevisionNumber: 1,
		WindowEnd:      144,
	}
	err := storeHeight(db, sc1.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}
	data, err := getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 32 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data) != sc1.RLPHash() {
			t.Error("DB persistence error")
		}
	}

	err = storeHeight(db, sc2.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}

	data, err = getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 64 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data[:32]) != sc1.RLPHash() || common.BytesToHash(data[32:]) != sc2.RLPHash() {
			t.Error("DB persistence error")
		}
	}

	err = storeHeight(db, sc3.RLPHash(), height)
	if err != nil {
		t.Error(err)
	}

	data, err = getHeight(db, height)
	if err != nil {
		t.Error(err)
	} else {
		if len(data) != 96 {
			t.Log(data)
			t.Log(len(data))
			t.Error("DB persistence error")
		}
		if common.BytesToHash(data[:32]) != sc1.RLPHash() || common.BytesToHash(data[32:64]) != sc2.RLPHash() || common.BytesToHash(data[64:]) != sc3.RLPHash() {
			t.Error("DB persistence error")
		}
	}
}
