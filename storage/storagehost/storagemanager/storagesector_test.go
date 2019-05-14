package storagemanager

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestIsFree(t *testing.T) {
	var vec BitVector = 1468 // 10110111100

	for i := 0; i < 11; i++ {
		fmt.Println(vec.isFree(uint16(i)))
	}
}

func TestSetUsage(t *testing.T) {
	var vec BitVector = 1468 // 10110111100

	if !vec.isFree(6) {
		t.Error("is not free")
	}

	fmt.Println(vec.isFree(6))

	vec.setUsage(6)

	fmt.Println(vec.isFree(6))
}

func TestClearUsage(t *testing.T) {
	var vec BitVector = 1468 // 10110111100
	fmt.Println(vec.isFree(3))

	vec.clearUsage(3)

	fmt.Println(vec.isFree(3))

	fmt.Println(vec)

	vec.clearUsage(4)

	vec.clearUsage(5)
	vec.clearUsage(6)

	vec.clearUsage(7)

	vec.clearUsage(0)
	vec.clearUsage(1)

	vec.clearUsage(8)

	vec.setUsage(0)

	fmt.Println(vec)
}

func TestFLock_TryLock(t *testing.T) {
	obj := struct {
		lock *folderLock
	}{lock: &folderLock{}}

	//obj.lock.Lock()

	//fmt.Println(obj.lock.TryLock())
	//obj.lock.Lock()
	obj.lock.TryLock()
	obj.lock.Unlock()
}

func TestHash(t *testing.T) {
	sm := &storageManager{}

	rand.Seed(time.Now().UTC().UnixNano())
	if _, err := rand.Read(sm.sectorSalt[:]); err != nil {

	}

	fmt.Println(sm.sectorSalt)

	fmt.Println(sm.getSectorID(sm.sectorSalt))
}

func TestStorageManager_AddSector(t *testing.T) {
	removeFolders(TESTPATH, t)
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

	fmt.Println("folder create done")

	var root [32]byte
	var data [1 << 12]byte
	copy(data[:], []byte("testing message"))

	//rand.Seed(time.Now().UTC().UnixNano())
	//rand.Read(root[:])

	root = [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	//fmt.Println(root)

	err = sm.AddSector(root, data[:])
	if err != nil {
		t.Error(err.Error())
	}

	root = [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 35, 98, 207, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	copy(data[:], []byte("testing message22222"))
	err = sm.AddSector(root, data[:])
	if err != nil {
		t.Error(err.Error())
	}

	root = [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 3, 98, 27, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	copy(data[:], []byte("testing message3333"))
	err = sm.AddSector(root, data[:])
	if err != nil {
		t.Error(err.Error())
	}

	err = sm.Close()
	if err != nil {
		t.Error(err.Error())
	}
}

func TestStorageManager_again(t *testing.T) {
	sm, err := New(TESTPATH, TST)
	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	root := [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	b, err := sm.ReadSector(root)
	fmt.Println(string(b))

	root = [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 3, 98, 27, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}
	b, err = sm.ReadSector(root)
	fmt.Println(string(b))

	root = [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 35, 98, 207, 135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}
	b, err = sm.ReadSector(root)
	fmt.Println(string(b))

	for _, sf := range sm.folders {
		fmt.Println(sf.path, sf.sectors)
	}
}
