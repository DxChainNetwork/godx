package storagehost

import (
	"testing"

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

	err := StoreStorageResponsibility(db, so.OriginStorageContract.RLPHash(), so)
	if err != nil {
		t.Error(err)
	}

	sos, err := getStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}

	if sos.OriginStorageContract.WindowEnd != 144 || sos.OriginStorageContract.WindowStart != 1 || sos.OriginStorageContract.RevisionNumber != 1 {
		t.Error("error")
	}

	err = deleteStorageResponsibility(db, so.OriginStorageContract.RLPHash())
	if err != nil {
		t.Error(err)
	}
}
