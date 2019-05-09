package storagemanager

import (
	"os"
	"sync"
	"testing"
)

func Test_AddStorageFolderBasic(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	//defer removeFolders("./testdata/", t)

	// create a new storage manager for testing mode
	sm, err := New("./testdata/", TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// folders for adding
	// NOTE: there is not expected to handle the abs path,
	// where in the program of testing mode skip the checking the path
	addfolders := []string{
		// normal add
		"./testdata/folders1",
		"./testdata/folders4",
		"./testdata/folders5",
		// duplicate add
		"./testdata/folders10",
		"./testdata/folders7",
		"./testdata/folders8",
		"./testdata/folders15",
		"./testdata/folders18",
	}

	var wg sync.WaitGroup
	// add all the specify folders
	for _, f := range addfolders {
		wg.Add(1)
		//create the storage folder
		go func(f string) {
			defer wg.Done()
			if err := sm.AddStorageFolder(f, SectorSize*64); err != nil {
				// if the error is not caused by the already existence of error
				if err != ErrFolderAlreadyExist {
					t.Fatal(err.Error())
					return
				}
			}
		}(f)

	}

	wg.Wait()

	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	//config := &configPersist{}
	//if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFileTmp), config); err != nil {
	//	t.Error(err.Error())
	//}

	//spew.Dump(config)

}

// helper function to clear the data file before and after a test case execute
func removeFolders(persistDir string, t *testing.T) {
	// clear the testing data
	if err := os.RemoveAll(persistDir); err != nil {
		t.Error("cannot remove the data when testing")
	}
}

func TestRename(t *testing.T) {
	file, _ := os.Create("testdata/faketmp.sys")
	file.Write([]byte("hello, I love you"))
	file.Sync()
	_ = os.Rename("testdata/faketmp.sys", "testdata/fake.sys")

	file, _ = os.Create("testdata/faketmp.sys")

}

func TestRunremove(t *testing.T) {
	removeFolders("testdata", t)
}
