package storagehost

import (
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/davecgh/go-spew/spew"
)

var dumper = spew.ConfigState{DisableMethods: true, Indent: "    "}

// tempDir removes and creates the folder named dxfile under the temp directory.
func tempDir(dirs ...string) string {
	path := filepath.Join(os.TempDir(), "storagehost", filepath.Join(dirs...))
	err := os.RemoveAll(path)
	if err != nil {
		panic(fmt.Sprintf("cannot remove all files under %v: %v", path, err))
	}
	err = os.MkdirAll(path, 0777)
	if err != nil {
		panic(fmt.Sprintf("cannot create directory %v", path))
	}
	return path
}

func newTestStorageHost(t *testing.T) *StorageHost {
	dir := tempDir(t.Name())
	h, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// load the default settings
	if err = h.load(); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestStorageHost_Load(t *testing.T) {
	h := newTestStorageHost(t)
	// Check whether the file has default settings
	if err := checkHostConfigFile(filepath.Join(h.persistDir, HostSettingFile), defaultConfig()); err != nil {
		t.Fatal(err)
	}
	//

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
	if err := h.Close(); err != nil {
		t.Errorf("Unable to close the host")
	}

	// manually load persistence, check if the setting is the default setting
	persist := new(persistence)
	if err := common.LoadDxJSON(storageHostMeta, filepath.Join("./testdata/", HostSettingFile), persist); err != nil {
		t.Errorf(err.Error())
	}

	// assert that the persistence file save the default setting
	if !reflect.DeepEqual(persist.Config, defaultConfig()) {
		spew.Dump(persist.Config)
		spew.Dump(defaultConfig())
		t.Errorf("the persistence file does not save the default setting as expected")
	}
}

func checkHostConfigFile(path string, expect storage.HostIntConfig) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file stat: %v", err)
	}
	var config storage.HostIntConfig
	if err := common.LoadDxJSON(storageHostMeta, path, &config); err != nil {
		return fmt.Errorf("cannot load DxJSON: %v", err)
	}
	if !reflect.DeepEqual(config, expect) {
		return fmt.Errorf("config not expected. \n\tExpect %vGot%v", dumper.Sdump(expect), dumper.Sdump(config))
	}
}

// Test if changing setting would result the change of the setting file
func TestStorageHost_DataPreservation(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// try to do the first data json renew, use the default value
	host, err := New("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}

	// extract the persistence data from host
	persist1 := host.extractPersistence()

	for Iterations := 10; Iterations > 0; Iterations-- {

		// close the host to check if it can save the data as expected
		//if err := host.Close(); err != nil {
		//	t.Errorf(err.Error())
		//}

		// renew the host again, check if match the data saving before closed
		//host, err = New("./testdata/")
		//if err != nil {
		//	t.Errorf(err.Error())
		//}

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
		host.revisionNumber = uint64(rand.Intn(RANDRANGE))
		// host setting structure
		host.config.AcceptingContracts = rand.Float32() < 0.5
		host.config.Deposit = *big.NewInt(int64(rand.Intn(RANDRANGE)))
		// host financial Metrics
		host.financialMetrics.ContractCount = uint64(rand.Intn(RANDRANGE))
		host.financialMetrics.StorageRevenue = common.NewBigInt(RANDRANGE)

		// extract the persistence information
		persist1 = host.extractPersistence()

		// make sure the persistence indeed loaded by the host
		if host.broadcast != persist1.BroadCast ||
			host.revisionNumber != persist1.RevisionNumber ||
			host.config.AcceptingContracts != persist1.Config.AcceptingContracts ||
			!reflect.DeepEqual(host.config.Deposit, persist1.Config.Deposit) ||
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
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// try to do the first data json renew, use the default value
	host, err := New("./testdata/")
	if err != nil {
		t.Errorf(err.Error())
	}
	// initialize the revision number to 0
	host.revisionNumber = 0

	for Iterations := 10; Iterations > 0; Iterations-- {
		// selectively random a storHostIntSetting
		internalSetting := storage.HostIntConfig{
			AcceptingContracts:   rand.Float32() < 0.5,
			MaxDownloadBatchSize: uint64(rand.Intn(RANDRANGE)),
			Deposit:              *big.NewInt(int64(rand.Intn(RANDRANGE))),
			MinBaseRPCPrice:      *big.NewInt(int64(rand.Intn(RANDRANGE))),
		}

		// set the randomly generated field to host
		if err := host.SetIntConfig(internalSetting, true); err != nil {
			t.Errorf("fail to set the internal setting")
		}

		// check if the revision number is increase
		if host.revisionNumber != uint64(10-Iterations+1) {
			t.Errorf("the revision number does not increase as expected")
		}

		// // This field just hard code the comparison, if more complex data structure is added
		// // to field, simple use this may correct some error
		// check if the host internal setting is set
		//extracted := host.getInternalConfig()
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
		//  filed would be init as an empty structure, in order to keep them consistence, more
		//  handling may needed

		if !reflect.DeepEqual(host.getInternalConfig(), internalSetting) {
			spew.Dump(internalSetting)
			spew.Dump(host.getInternalConfig())
			t.Errorf("the host setting does not match the previously setted")
		}
	}

	if err := host.Close(); err != nil {
		t.Errorf(err.Error())
	}
}

// helper function to clear the data file before and after a test case execute
func removeFolders(persisDir string, t *testing.T) {
	// clear the testing data
	if err := os.RemoveAll(persisDir); err != nil {
		t.Error("cannot remove the data when testing")
	}
}
