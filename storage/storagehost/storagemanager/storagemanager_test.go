package storagemanager

import (
	"errors"
	"github.com/DxChainNetwork/godx/common"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

const TESTPATH = "./testdata/"

func Test_AddStorageFolderBasic(t *testing.T) {
	// clear the saved data for testing
	removeFolders(TESTPATH, t)
	defer removeFolders(TESTPATH, t)

	// create a new storage manager for testing mode
	sm, err := New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// folders for adding
	// NOTE: there is not expected to handle the abs path,
	// where in the program of testing mode skip the checking the path
	addfolders := []string{
		// normal add
		TESTPATH + "folders1",
		TESTPATH + "folders2",
		TESTPATH + "folders3",
		// duplicate add
		TESTPATH + "folders4",
		TESTPATH + "folders1",
		TESTPATH + "folders2",
		TESTPATH + "folders3",
		TESTPATH + "folders4",
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
				// TODO: to handle more exception such as size too large and more
				if err != ErrFolderAlreadyExist {
					t.Error(err.Error())
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
	config := &configPersist{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), config); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, config.Folders); err != nil {
		t.Error(err.Error())
	}
}

func Test_AddFolderWithExistingSectorsFile(t *testing.T) {
	// clear the saved data for testing
	removeFolders(TESTPATH, t)
	defer removeFolders(TESTPATH, t)

	// create the folder by given path
	err := os.MkdirAll(TESTPATH+"foldershouldfail", 0700)
	_, errMeta := os.Create(filepath.Join(TESTPATH+"foldershouldfail", sectorMetaFileName))
	_, errSector := os.Create(filepath.Join(TESTPATH+"foldershouldfail", sectorDataFileName))
	err = common.ErrCompose(err, errMeta, errSector)
	if err != nil {
		// the error is given by the system, not by the program, ignore and jump out
		t.Logf("fail to create folder for testing use")
		return
	}

	// create a new storage manager for testing mode
	sm, err := New(TESTPATH, TST)
	if err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// try to add the folder, which given as the already existed folder
	if err := sm.AddStorageFolder(TESTPATH+"foldershouldfail", SectorSize*64); err == nil {
		t.Error("sector and metadata should be checked and result of cancellation of operation")
	} else if err != ErrDataFileAlreadExist {
		// if the error is not caused by the existing of sector and data files
		t.Error(err.Error())
	}

	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	config := &configPersist{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), config); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, config.Folders); err != nil {
		t.Error(err.Error())
	}
}

func Test_ReloadConfig(t *testing.T) {
	// clear the saved data for testing
	removeFolders(TESTPATH, t)
	defer removeFolders(TESTPATH, t)

	// create a new storage manager for testing mode
	sm, err := New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	var wg sync.WaitGroup
	// add all the specify folders
	for i := 0; i < int(MaxStorageFolders); i++ {
		wg.Add(1)
		//create the storage folder
		go func(f string) {
			defer wg.Done()
			if err := sm.AddStorageFolder(f, SectorSize*MaxSectorPerFolder); err != nil {
				// if the error is not caused by the already existence of error
				// TODO: to handle more exception such as size too large and more
				if err != ErrFolderAlreadyExist {
					t.Error(err.Error())
				}
			}
		}(TESTPATH + "folder" + strconv.Itoa(i))
	}

	wg.Wait()

	// Close the storage manager
	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// create a new storage manager for testing mode
	sm, err = New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// Check if all folders could be reconstructed
	if len(sm.folders) != int(MaxStorageFolders) {
		t.Error("number of folders does not match the expected")
	}

	// close the storage manger to gain clean shut down
	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	config := &configPersist{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), config); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, config.Folders); err != nil {
		t.Error(err.Error())
	}
}

func Test_RevertWhenAdd(t *testing.T) {
	removeFolders(TESTPATH, t)
	defer removeFolders(TESTPATH, t)

	// create a new storage manager for testing mode
	sm, err := New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	failStart := int(MaxStorageFolders) / 2

	for i := 0; i < int(MaxStorageFolders); i++ {
		f := TESTPATH + strconv.Itoa(i)

		if i == failStart {
			MockFails["ADD_FAIL"] = true
		}

		if err := sm.AddStorageFolder(f, SectorSize*MaxSectorPerFolder); err != nil {
			if err != ErrMock {
				t.Error(err.Error())
			}
		}
	}

	if len(sm.folders) != failStart {
		t.Error("number of folders does not match the expected")
	}

	// close the storage manger to gain clean shut down
	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	config := &configPersist{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), config); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, config.Folders); err != nil {
		t.Error(err.Error())
	}
}

func Test_DisruptAdd(t *testing.T) {
	removeFolders(TESTPATH, t)
	defer removeFolders(TESTPATH, t)

	MockFails["EXIT"] = true

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// create a new storage manager for testing mode
		sm, err := New(TESTPATH, TST)
		if sm == nil || err != nil {
			t.Error("cannot initialize the storage manager: ", err.Error())
		}

		failStart := int(MaxStorageFolders) / 2

		for i := 0; i <= failStart; i++ {
			f := TESTPATH + strconv.Itoa(i)

			if i == failStart {
				MockFails["ADD_EXIT"] = true
			}

			if err := sm.AddStorageFolder(f, SectorSize*MaxSectorPerFolder); err != nil {
				if err != ErrMock {
					return
				}
			}
		}

		// close the storage manger to gain clean shut down
		if err := sm.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	wg.Wait()

	// restart a new storage manager for testing mode, check if recover or not
	sm, err := New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// close the storage manger to gain clean shut down
	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}
}


// isExpectedFolderList check if the group of folder record
// contains duplicate index or duplicate folder path,
// also make sure the folders are all created
func isExpectedFolderList(sm *storageManager, folders []folderPersist) error {
	// check if the size is as expected
	if len(sm.folders) != len(folders) {
		return errors.New("number of persisted folder does not match the number of folder in memory")
	}

	// a setIndex to contains the used index
	setIndex := make(map[uint16]bool)

	// TODO: should able to check from full path
	setFolder := make(map[string]bool)

	// loop each folders, ensure all the index appear only once and in the range of uint16
	for _, folder := range folders {
		// check if in the memory the folder exist
		_, exist := sm.folders[folder.Index]
		if !exist {
			return errors.New("the folder persisted does not exist in memory")
		}

		// check if the path recorded is the same
		if folder.Path != sm.folders[folder.Index].path {
			return errors.New("the folder persisted does not match the folder recorded in memory	")
		}

		_, exist = setIndex[folder.Index]
		// check if the folder contains duplicate
		if exist {
			return errors.New("folder index contains duplicate")
		}

		// check if the folder are created as expected
		_, err := os.Stat(folder.Path)
		if err != nil {
			return err
		}

		// get the absolute path of from the record
		absPath, err := filepath.Abs(folder.Path)
		if err != nil {
			return err
		}

		// check if the absolute path is exist
		_, exist = setFolder[absPath]
		if exist {
			return errors.New("folder path contains duplicate")
		}

		// because the Index is uint of uint16, is always in the range
		// then put the index and the checked folder path to the map
		setIndex[folder.Index] = true
		setFolder[absPath] = true
	}

	return nil
}

// helper function to clear the data file before and after a test case execute
func removeFolders(persistDir string, t *testing.T) {
	// clear the testing data
	if err := os.RemoveAll(persistDir); err != nil {
		t.Error("cannot remove the data when testing")
	}
}
