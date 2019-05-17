package storagemanager

import (
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/pkg/errors"
)

const TestPath = "./testdata/"

// Test to check if the extraction would change the value of
// storage manager or not
func TestExtractPersist(t *testing.T) {
	// get the mocking data
	sm := mockStorageManager()
	// try to extract the storage manager
	config := extractConfig(*sm)

	// loop through the config and check if the usage is rest
	for fi, fp := range config.Folders {
		for ui, u := range fp.Usage {
			// the usage should not be zero in storage manager
			if u != 0 {
				t.Error("the usage in persist should be reset to 0")
			}
			//
			if sm.folders[uint16(fi)].usage[ui] == 0 {
				t.Error("the usage in storage manager should not be reset to 0")
			}
		}
	}
}

// TestFLock_TryLock test try lock of folderLock
func TestFLock_TryLock(t *testing.T) {
	obj := struct {
		lock *folderLock
	}{
		lock: &folderLock{},
	}
	obj.lock.TryLock()
	obj.lock.Unlock()

	// no lock, try lock should return true indicating lock success
	if !obj.lock.TryLock() {
		t.Error("try lock fail")
	}

	// try lock above already lock, this try lock expected to return false
	if obj.lock.TryLock() {
		t.Error("try lock fail")
	}

	// unlock should work as expected
	obj.lock.Unlock()
}

// TestMergeEmptyWal test if only wal exist, the
// merge work as expected
func TestMergeEmptyWal(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// prepare the metadata for files
	wal, tmp := mockWal(t)

	// only write entry for wal tmp
	tmp, entries := mockWroteEntries(tmp, filepath.Join(TestPath, walFileTmp), t)

	// merge the files
	if err := mergeWal(wal, tmp); err != nil {
		t.Errorf(err.Error())
	}

	// close the resources
	if err := wal.Close(); err != nil {
		t.Errorf(err.Error())
	}

	if err := tmp.Close(); err != nil {
		t.Errorf(err.Error())
	}

	// check if the files are as expected
	if err := isExpectedWal(entries); err != nil {
		t.Errorf(err.Error())
	}
}

// TestMergeEmptyWalTmp test if only wal tmp exist, the
// merge work work as expected
func TestMergeEmptyWalTmp(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// prepare the metadata for files
	wal, tmp := mockWal(t)

	// only write entry for wal
	wal, entries := mockWroteEntries(wal, filepath.Join(TestPath, walFile), t)

	// merge the files
	if err := mergeWal(wal, tmp); err != nil {
		t.Errorf(err.Error())
	}

	// close the resources
	if err := wal.Close(); err != nil {
		t.Errorf(err.Error())
	}

	if err := tmp.Close(); err != nil {
		t.Errorf(err.Error())
	}

	// check if the files are as expected
	if err := isExpectedWal(entries); err != nil {
		t.Errorf(err.Error())
	}
}

// TestMergeWalBoth test if both wal exist, the merge work as expected
func TestMergeWalBoth(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// prepare the metadata for files
	wal, tmp := mockWal(t)

	// write entry for wal
	wal, _ = mockWroteEntries(wal, filepath.Join(TestPath, walFile), t)
	// write entry for wal tmp
	tmp, entries := mockWroteEntries(tmp, filepath.Join(TestPath, walFileTmp), t)

	// merge the files
	if err := mergeWal(wal, tmp); err != nil {
		t.Errorf(err.Error())
	}

	// close the resources
	if err := wal.Close(); err != nil {
		t.Errorf(err.Error())
	}

	if err := tmp.Close(); err != nil {
		t.Errorf(err.Error())
	}

	// check if the files are as expected
	if err := isExpectedWal(entries); err != nil {
		t.Errorf(err.Error())
	}
}

// mock the value of storage manager for testing of extraction
// generate the storage manager which all usage are marked as
// used, and all free sector are also marked
func mockStorageManager() *storageManager {
	numFolder := 5 // number of folder
	numUsage := 3  // number of usage array length

	// generate object for storage manager
	sm := &storageManager{
		folders: make(map[uint16]*storageFolder),
	}

	// give the storage manager a random sector salt
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Read(sm.sectorSalt[:])

	// generate each folder content
	for i := 0; i < numFolder; i++ {
		// init folder object
		sm.folders[uint16(i)] = &storageFolder{
			index:       uint16(i),
			path:        strconv.Itoa(i),
			usage:       make([]BitVector, numUsage),
			freeSectors: make(map[sectorID]uint32),
		}

		// fill all the usage
		for j := 0; j < numUsage; j++ {
			sm.folders[uint16(i)].usage[j] = math.MaxUint64
		}

		// mark all the sector to be free sector
		for k := 0; k < numUsage*granularity; k++ {
			var id [12]byte
			rand.Read(id[:])
			// check if the id is already used
			_, exist := sm.folders[uint16(i)].freeSectors[id]
			if exist {
				k--
				continue
			}
			sm.folders[uint16(i)].freeSectors[id] = uint32(k)
		}
	}

	return sm
}

// @ Require: wal file is closed
func isExpectedWal(tmpEntries []logEntry) error {
	wal, err := os.OpenFile(filepath.Join(TestPath, walFile), os.O_RDWR, 0700)
	if err != nil {
		return err
	}

	// walEntries store all entry information
	//var walEntries []logEntry
	walEntries := make([]logEntry, 0)

	decoder := json.NewDecoder(wal)
	for err == nil {
		var entry logEntry
		err = decoder.Decode(&entry)
		if err != nil {
			break
		}
		walEntries = append(walEntries, entry)
	}

	// assert the error is caused by the ending of file reading
	if err != io.EOF {
		return err
	}

	// check entries from end to front
	tmpIdx, walIdx := len(tmpEntries)-1, len(walEntries)-1
	for tmpIdx >= 0 && walIdx >= 0 {
		// if the things is not equal, return error
		if !reflect.DeepEqual(tmpEntries[tmpIdx], walEntries[walIdx]) {
			return errors.New("the entry expected to match")
		}
		tmpIdx, walIdx = tmpIdx-1, walIdx-1
	}

	// if there are entries in tmp not read out, fail
	if tmpIdx >= 0 {
		return errors.New("the entry expected to match")
	}

	return nil
}

func mockWal(t *testing.T) (*os.File, *os.File) {
	// make the directory
	if err := os.MkdirAll(TestPath, 0700); err != nil {
		t.Errorf(err.Error())
	}
	// create the temporary file and write meta data
	tmp, err := os.Create(filepath.Join(TestPath, walFileTmp))
	if err != nil {
		t.Errorf(err.Error())
	}
	if err := mockWriteWALMeta(tmp); err != nil {
		t.Errorf(err.Error())
	}

	// create the wal file and write metadata
	wal, err := os.Create(filepath.Join(TestPath, walFile))
	if err != nil {
		t.Errorf(err.Error())
	}
	if err := mockWriteWALMeta(wal); err != nil {
		t.Errorf(err.Error())
	}
	// close files to get refresh
	err = common.ErrCompose(err, wal.Close())
	err = common.ErrCompose(err, tmp.Close())
	if err != nil {
		t.Errorf(err.Error())
	}

	// reopen the wal file
	wal, err = os.OpenFile(filepath.Join(TestPath, walFile), os.O_RDWR, 0700)
	if err != nil {
		t.Errorf(err.Error())
	}
	// open the tmp file
	tmp, err = os.OpenFile(filepath.Join(TestPath, walFileTmp), os.O_RDWR, 0700)
	if err != nil {
		t.Errorf(err.Error())
	}

	return wal, tmp
}

// @ effect: mock write some random entry to the file,
// and the file would be closed at the end
func mockWroteEntries(file *os.File, path string, t *testing.T) (*os.File, []logEntry) {
	rand.Seed(time.Now().UTC().UnixNano())
	itr := rand.Int31n(10)
	var entries []logEntry

	for itr > 0 {
		// seek the last ending position, and insert
		n, err := file.Seek(0, 2)
		if err != nil {
			t.Errorf("error caused by io")
		}

		// mock the entry
		entry := logEntry{PrepareAddStorageFolder: []folderPersist{
			{
				Index: uint16(itr),
				Path:  path + strconv.Itoa(int(itr)),
			}}}

		// write the change byte to the end
		changeBytes, err := json.MarshalIndent(entry, "", "\t")
		if err != nil {
			t.Errorf(err.Error())
		}
		if _, err = file.WriteAt(changeBytes, n); err != nil {
			t.Errorf(err.Error())
		}

		entries = append(entries, entry)
		itr--
	}

	if err := file.Close(); err != nil {
		// if the file close is not nil
		t.Errorf(err.Error())
	}

	// reopen the file
	file, err := os.OpenFile(path, os.O_RDWR, 0700)
	if err != nil {
		t.Errorf(err.Error())
	}

	return file, entries
}

// mock to write the metadata for files
func mockWriteWALMeta(file *os.File) error {
	// marshal the metadata to json format
	changeBytes, err := json.MarshalIndent(walMetadata, "", "\t")
	if err != nil {
		return err
	}
	// simply write at the beginning
	_, err = file.WriteAt(changeBytes, 0)
	return err
}

// removeFolders is a helper function to clear the data file before and after a test case execute
func removeFolders(persistDir string, t *testing.T) {
	// clear the testing data
	if err := os.RemoveAll(persistDir); err != nil {
		t.Error("cannot remove the data when testing")
	}
}
