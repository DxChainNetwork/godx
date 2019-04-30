package storagehost

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/davecgh/go-spew/spew"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Test if the persist Folder could be generate as expected
// Test if the persist file could be fill using the default value
// 		when the host is the first time initialized
func TestStorageHost_DefaultFolderStatus(t *testing.T) {
	// clear the saved data for testing
	rm := func() {
		if err := remmoveFolders("./testdata/", t); err != nil {
			return
		}
	}
	rm()
	defer rm()

	// do a new host, check if the folder are all generated
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// check if the database folder is initialized
	if f, err := os.Stat("./testdata/hostdb"); err != nil {
		t.Errorf(err.Error())
	} else if !f.IsDir() {
		t.Error("the hostdb should be a directory")
	}

	// test if the host.json file is initialized
	if _, err := os.Stat("./testdata/host.json"); err != nil {
		t.Errorf(err.Error())
	}

	// close the host, the file should be synchronize
	if err := host.Close(); err != nil {
		t.Errorf("Unable to close the host")
	}

	// manually load persistence, check if the setting is the default setting
	persist := new(persistence)
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join("./testdata/", HostSettingFile), persist); err != nil {
		t.Errorf(err.Error())
	}

	// assert that the persistence file save the default setting
	if !reflect.DeepEqual(persist.Settings, loadDefaultSetting()) {
		spew.Dump(persist.Settings)
		spew.Dump(loadDefaultSetting())
		t.Errorf("the persistence file does not save the default setting as expected")
	}
}

// Test if changing setting would result the change of the setting file
func TestStorageHost_DataPreservation(t *testing.T) {
	// clear the saved data for testing
	rm := func() {
		if err := remmoveFolders("./testdata/", t); err != nil {
			return
		}
	}

	rm()
	defer rm()

	// try to do the first data json renew, use the default value
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// extract the persistence data from host
	persist1 := host.extractPersistence()

	for Iterations := 10; Iterations > 0; Iterations-- {

		// close the host to check if it can save the data as expected
		if err := host.Close(); err != nil {
			t.Errorf(err.Error())
		}

		// renew the host again, check if match the data saving before closed
		host, err = NewStorageHost("./testdata/")
		if err != nil {
			t.Errorf(err.Error())
		}

		persist2 := host.extractPersistence()

		if !reflect.DeepEqual(persist1, persist2) {
			spew.Dump(persist1)
			spew.Dump(persist2)
			t.Errorf("two persistance does not equal to each other")
		}

		// change serveral part in the setting again, use the random value
		// NOTE: this number does not make any sense, just for checking
		// 		if the related method indeed can change and persist the host

		// host primitive variable
		host.broadcast = rand.Float32() < 0.5
		host.revisionNumber = uint64(rand.Intn(100000000))
		// host setting structure
		host.settings.AcceptingContracts = rand.Float32() < 0.5
		host.settings.Deposit = *big.NewInt(int64(rand.Intn(100000000)))
		// host financial Metrics
		host.financialMetrics.ContractCount = uint64(rand.Intn(100000000))
		host.financialMetrics.StorageRevenue = *big.NewInt(int64(rand.Intn(100000000)))

		// extract the persistence information
		persist1 = host.extractPersistence()

		// make sure the persistence indeed loaded by the host
		if host.broadcast != persist1.BroadCast ||
			host.revisionNumber != persist1.RevisionNumber ||
			host.settings.AcceptingContracts != persist1.Settings.AcceptingContracts ||
			!reflect.DeepEqual(host.settings.Deposit, persist1.Settings.Deposit) ||
			host.financialMetrics.ContractCount != persist1.FinalcialMetrics.ContractCount ||
			!reflect.DeepEqual(host.financialMetrics.StorageRevenue, persist1.FinalcialMetrics.StorageRevenue) {
			t.Errorf("persistence extracted from host does not match the expected")
		}
	}

	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

func TestStorageHost_SetIntSetting(t *testing.T) {
	// clear the saved data for testing
	rm := func() {
		if err := remmoveFolders("./testdata/", t); err != nil {
			return
		}
	}
	rm()
	defer rm()

	// try to do the first data json renew, use the default value
	host, err := NewStorageHost("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}
	// initialize the revision number to 0
	host.revisionNumber = 0

	for Iterations := 10; Iterations > 0; Iterations-- {
		// selectively random a storHostIntSetting
		internalSetting := StorageHostIntSetting{
			AcceptingContracts:   rand.Float32() < 0.5,
			MaxDownloadBatchSize: uint64(rand.Intn(100000000)),
			Deposit:              *big.NewInt(int64(rand.Intn(100000000))),
			MinBaseRPCPrice:      *big.NewInt(int64(rand.Intn(100000000))),
		}

		// set the randomly generated field to host
		if err := host.SetIntSetting(internalSetting, true); err != nil {
			t.Errorf("fail to set the internal setting")
		}

		// check if the revision number is increase
		if host.revisionNumber != uint64(10-Iterations+1) {
			t.Errorf("the revision number does not increase as expected")
		}

		// // This field just hard code the comparison, if more complex data structure is added
		// // to field, simple use this may correct some error
		// check if the host internal setting is set
		//extracted := host.InternalSetting()
		//
		//if  extracted.AcceptingContracts != internalSetting.AcceptingContracts ||
		//	extracted.MaxDownloadBatchSize != internalSetting.MaxDownloadBatchSize ||
		//	!reflect.DeepEqual(extracted.Deposit, internalSetting.Deposit) ||
		//	!reflect.DeepEqual(extracted.MinBaseRPCPrice, internalSetting.MinBaseRPCPrice){
		//
		//	spew.Dump(internalSetting)
		//	spew.Dump(extracted)
		//	t.Errorf("the host setting does not match the previously setted")
		//}

		// TODO: for a more complex data structure, some field may be init as nil, but some
		//  filed would be init as an empty structre, in order to keep them consistence, more
		//  handling may needed

		if !reflect.DeepEqual(host.InternalSetting(), internalSetting) {
			spew.Dump(internalSetting)
			spew.Dump(host.InternalSetting())
			t.Errorf("the host setting does not match the previously setted")
		}
	}

	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

// helper function to clear the data file before and after a test case execute
func remmoveFolders(Persisdir string, t *testing.T) error {
	// clear the testing data
	if err := os.RemoveAll(Persisdir); err != nil {
		t.Logf("cannot remove the data when testing")
		return err
	}
	return nil
}
