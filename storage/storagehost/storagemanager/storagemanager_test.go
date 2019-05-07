package storagemanager

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/common"
)

func Test_AddStorageFolderBasic(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// create a new storage manager for testing mode
	sm, err := New("./testdata/", TST)
	if err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// folders for adding
	// NOTE: there is not expected to handle the abs path,
	// where in the program of testing mode skip the checking the path
	addfolders := []string{
		// normal add
		"./testdata/folders1",
		"./testdata/folders2",
		"./testdata/folders3",
		// duplicate add
		"./testdata/folders1",
		"./testdata/folders2",
		"./testdata/folders3",
		"./testdata/folders2",
		"./testdata/folders1",
	}

	// add all the specify folders
	for _, f := range addfolders {
		// create the storage folder
		if err := sm.AddStorageFolder(f, SectorSize*64); err != nil {
			// if the error is not caused by the already existence of error
			if err != ErrFolderAlreadyExist {
				t.Error(err.Error())
			}
		}
	}

	// force synchronize all the things
	if err := sm.syncConfig(); err != nil {
		t.Error(err)
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	persist := &persistence{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), persist); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, persist.Folders); err != nil {
		t.Error(err.Error())
	}

}

func Test_AddExistingStorageFolder(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// create the folder by given path
	err := os.MkdirAll("./testdata/folder", 0700)
	if err != nil {
		// the error is given by the system, not by the program, ignore and jump out
		t.Logf("fail to create folder for testing use")
		return
	}

	// create a new storage manager for testing mode
	sm, err := New("./testdata/", TST)
	if err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// try to add the folder, which given as the already existed folder
	if err := sm.AddStorageFolder("./testdata/folder", SectorSize*64); err != nil {
		t.Error(err.Error())
	}

	// force synchronize all the things
	if err := sm.syncConfig(); err != nil {
		t.Error(err)
	}

	// load from the synchronized data to check if the
	// folders information are preserved
	persist := &persistence{}
	if err := common.LoadDxJSON(configMetadata, filepath.Join(sm.persistDir, configFile), persist); err != nil {
		t.Error(err.Error())
	}

	// check if the persist folder is all valid
	if err := isExpectedFolderList(sm, persist.Folders); err != nil {
		t.Error(err.Error())
	}
}

func Test_AddFolderWithExistingSectorsFile(t *testing.T) {
	// clear the saved data for testing
	removeFolders("./testdata/", t)
	defer removeFolders("./testdata/", t)

	// create the folder by given path
	err := os.MkdirAll("./testdata/foldershouldfail", 0700)
	_, errMeta := os.Create(filepath.Join("./testdata/foldershouldfail", sectorMetaFileName))
	//_, errSector := os.Create(filepath.Join("./testdata/foldershouldfail", sectorDataFileName))
	err = common.ErrCompose(err, errMeta)
	if err != nil {
		// the error is given by the system, not by the program, ignore and jump out
		t.Logf("fail to create folder for testing use")
		return
	}

	// create a new storage manager for testing mode
	sm, err := New("./testdata/", TST)
	if err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// try to add the folder, which given as the already existed folder
	if err := sm.AddStorageFolder("./testdata/foldershouldfail", SectorSize*64); err != nil {
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

		// check if the folder in memory contains the same informations
		if !reflect.DeepEqual(sm.folders[folder.Index].usage, folder.Usage) ||
			folder.Path != sm.folders[folder.Index].path {
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
