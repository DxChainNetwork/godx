package storagemanager

import (
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestAddPhysicalSectorNormal test if the adding of sector
// basic operation work as expected
func TestStorageManager_AddSectorBasic(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// create a new storage manager for testing mode
	sm := constructTestManager(3, t)
	if sm == nil {
		t.Error("cannot initialize the storage manager")
	}

	// mock generate root and data try to load and retrieve
	// through metadata to data

	// Not the data is 1 << 12 byte
	var data1 [1 << 12]byte
	copy(data1[:], []byte("testing message: message 01"))

	root1 := [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207,
		135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	err := sm.AddSector(root1, data1[:])
	if err != nil {
		t.Error(err.Error())
	}

	var data2 [1 << 12]byte
	copy(data2[:], []byte("testing message: message 02"))

	root2 := [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 35, 98, 207, 135,
		226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	err = sm.AddSector(root2, data2[:])
	if err != nil {
		t.Error(err.Error())
	}

	var data3 [1 << 12]byte
	copy(data3[:], []byte("testing message: message 03"))

	root3 := [32]byte{2, 68, 137, 90, 127, 127, 51, 118, 68, 37, 25, 3, 98, 27, 135,
		226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	err = sm.AddSector(root3, data3[:])
	if err != nil {
		t.Error(err.Error())
	}

	err = sm.Close()
	if err != nil {
		t.Error(err.Error())
	}

	smAPI, err := New(TestPath, TST)
	sm = smAPI.(*storageManager)

	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	b, err := sm.ReadSector(root1)
	if !reflect.DeepEqual(b, data1[:]) {
		t.Error("loaded data does not match the extracted data")
	}

	b, err = sm.ReadSector(root2)
	if !reflect.DeepEqual(b, data2[:]) {
		t.Error("loaded data does not match the extracted data")
	}

	b, err = sm.ReadSector(root3)
	if !reflect.DeepEqual(b, data3[:]) {
		t.Error("loaded data does not match the extracted data")
	}

	err = sm.Close()
	if err != nil {
		t.Error(err.Error())
	}
}

// TestAddPhysicalSectorNormal test add sector which generate root
// randomly. Test the basic operation of adding a sector
func TestAddPhysicalSectorNormal(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	folderNum := 4
	// create a new storage manager for testing mode
	sm := constructTestManager(folderNum, t)
	if sm == nil {
		t.Error("cannot initialize the storage manager")
	}

	var err error
	// NOTE: when adding folder, pick every folder contain 64 sectors, so
	// maximum, the folder can contain 64 * 4 number of sectors
	roots := make([][32]byte, 64*folderNum)
	data := make([][1 << 12]byte, 64*folderNum)

	for i := 0; i < 64*4; i++ {
		var root [32]byte

		rand.Seed(time.Now().UTC().UnixNano())
		rand.Read(root[:])

		var msg [1 << 12]byte
		copy(msg[:], []byte("testing message: "+strconv.Itoa(i)))

		err = sm.AddSector(root, msg[:])
		if err != nil {
			t.Error(err.Error())
		}

		roots[i] = root
		data[i] = msg
	}

	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// reopen the storage manager again
	smAPI, err := New(TestPath, TST)
	sm = smAPI.(*storageManager)

	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	for i := 0; i < 64*folderNum; i++ {

		b, err := sm.ReadSector(roots[i])
		if err != nil {
			t.Error(err.Error())
		}

		if !reflect.DeepEqual(b, data[i][:]) {
			t.Error("loaded data does not match the extracted data")
		}
	}

	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}
}

// TestAddPhysicalSectorExhausted test adding sectors exhaustively,
// allow reaching the maximum storage folder and maximum sector number
// test if the system work when reaching the limitation
func TestAddPhysicalSectorExhausted(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)
	// create a new storage manager for testing mode
	smAPI, err := New(TestPath, TST)
	sm := smAPI.(*storageManager)

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
				if err != ErrFolderAlreadyExist {
					t.Error(err.Error())
				}
			}
		}(TestPath + "folder" + strconv.Itoa(i))
	}

	wg.Wait()

	// NOTE: when adding folder, pick every folder contain 64 sectors, so
	// maximum, the folder can contain 64 * 4 number of sectors
	roots := make([][32]byte, MaxStorageFolders*MaxSectorPerFolder)
	data := make([][1 << 12]byte, MaxStorageFolders*MaxSectorPerFolder)

	for i := 0; i < int(MaxStorageFolders*MaxSectorPerFolder); i++ {
		var root [32]byte

		rand.Seed(time.Now().UTC().UnixNano())
		rand.Read(root[:])

		var msg [1 << 12]byte
		copy(msg[:], []byte("testing message: "+strconv.Itoa(i)))

		err = sm.AddSector(root, msg[:])
		if err != nil {
			t.Error(err.Error())
		}

		roots[i] = root
		data[i] = msg
	}

	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// reopen the storage manager again
	smAPI, err = New(TestPath, TST)
	sm = smAPI.(*storageManager)

	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// go through and try to read the sector data, according to the root recorded
	// check if the data previously store can be loaded
	for i := 0; i < int(MaxStorageFolders*MaxSectorPerFolder); i++ {

		// read the sector
		b, err := sm.ReadSector(roots[i])
		if err != nil {
			t.Error(err.Error())
		}

		// if the loaded data and the data stored does not match
		if !reflect.DeepEqual(b, data[i][:]) {
			t.Error("loaded data does not match the extracted data")
		}
	}

	// shut down the storage manager, do the last commit
	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}
}

// TestAddVirtualSectorBasic test if the virtual sector could be add successfully
// include the test of reload the config and checking the number of virtual sector
func TestAddVirtualSectorBasic(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// create a new storage manager for testing mode
	sm := constructTestManager(3, t)
	if sm == nil {
		t.Error("cannot initialize the storage manager")
	}

	var err error
	root := [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207,
		135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}
	var data [1 << 12]byte
	copy(data[:], []byte("testing message"))

	addNum := 3

	// attempt to add numbers of virtual sectors
	for i := 0; i < addNum; i++ {
		if err := sm.AddSector(root, data[:]); err != nil {
			t.Error(err.Error())
		}
	}

	// get the id
	id := sm.getSectorID(root)

	// check if the count is the same as number of sector
	if int(sm.sectors[id].count) != addNum {
		t.Errorf("number of virtual sector does not match as the expected")
	}

	// close the storage manager
	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}

	// reopen the storage manager, check if the number of virtual sector is recorded
	smAPI, err := New(TestPath, TST)
	sm = smAPI.(*storageManager)

	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	if sm.sectors[id].count != 3 {
		t.Errorf("number of virtual sector does not match as the expected")
	}

	// close the storage manager
	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}
}

func TestRemoveSectorBasic(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	// create a new storage manager for testing mode
	// create a new storage manager for testing mode
	sm := constructTestManager(3, t)
	if sm == nil {
		t.Error("cannot initialize the storage manager")
	}

	root := [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207,
		135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}
	var data [1 << 12]byte
	copy(data[:], []byte("testing message"))

	addNum := 3
	// attempt to add numbers of virtual sectors
	for i := 0; i < addNum; i++ {
		if err := sm.AddSector(root, data[:]); err != nil {
			t.Error(err.Error())
		}
	}

	// get the id
	id := sm.getSectorID(root)

	// attempt to remove numbers of virtual sectors
	for addNum > 0 {
		// check if the count is the same as number of sector
		if int(sm.sectors[id].count) != addNum {
			t.Errorf("number of virtual sector does not match as the expected")
		}
		if err := sm.removeSector(id); err != nil {
			t.Errorf(err.Error())
		}
		addNum--
	}

	// the sector should be removed right now
	if _, exist := sm.sectors[id]; exist {
		t.Error("the sector should not exist in the memory")
	}

	// close the storage manager
	if err := sm.Close(); err != nil {
		t.Error(err.Error())
	}
}

func TestRecoverAddSector(t *testing.T) {
	removeFolders(TestPath, t)
	defer removeFolders(TestPath, t)

	root := [32]byte{213, 68, 137, 90, 127, 127, 51, 118, 68, 37, 215, 35, 98, 207,
		135, 226, 162, 53, 124, 124, 127, 126, 160, 101, 170, 102, 114, 75, 161, 66, 233, 163}

	var data [1 << 12]byte
	copy(data[:], []byte("testing message"))

	var id sectorID

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// create a new storage manager for testing mode
		smAPI, err := New(TestPath, TST)
		sm := smAPI.(*storageManager)

		if sm == nil || err != nil {
			t.Error("cannot initialize the storage manager: ", err.Error())
		}

		for i := 0; i <= 3; i++ {
			f := TestPath + strconv.Itoa(i)

			if err := sm.AddStorageFolder(f, SectorSize*128); err != nil {
				t.Error(err.Error())
			}
		}

		MockFails["EXIT"] = true
		MockFails["Add_SECTOR_EXIT"] = true

		// get the id
		id = sm.getSectorID(root)

		if err := sm.AddSector(root, data[:]); err != nil && err != ErrMock {
			t.Error(err.Error())
		}

		// close the storage manger to gain clean shut down
		if err := sm.Close(); err != nil {
			t.Error(err.Error())
		}
	}()

	wg.Wait()

	// reopen the storage manager, expected to start the recover process
	smAPI, err := New(TestPath, TST)
	sm := smAPI.(*storageManager)

	if sm == nil || err != nil {
		t.Error("cannot initialize the storage manager: ", err.Error())
	}

	// check if the sector object exist in the system
	sector, exist := sm.sectors[id]
	if !exist {
		t.Error("recover process fail")
	}

	// then try to read the sector data
	b, err := sm.ReadSector(root)
	if err != nil {
		t.Error(err.Error())
	}

	// check if the data loaded is the same as the data to store before crashing
	if !reflect.DeepEqual(b, data[:]) {
		t.Error("loaded data does not match the extracted data")
	}

	//check if the folder is recovered
	folder, exist := sm.folders[sector.storageFolder]
	if !exist {
		t.Error("recover process fail")
	}

	usageIndex := sector.index / granularity
	bitIndex := sector.index % granularity

	// assert the usage is not 0
	if folder.usage[usageIndex].isFree(uint16(bitIndex)) {
		t.Error("the sector index should be used")
	}

	// assert the free sector is not recorded
	_, exist = folder.freeSectors[id]
	if exist {
		t.Error("free sector should not record the sector id")
	}

	// close the storage manager
	if err = sm.Close(); err != nil {
		t.Error(err.Error())
	}
}

// creates folder by given number in the storage manager
func constructTestManager(folderNum int, t *testing.T) *storageManager {
	// create a new storage manager for testing mode
	smAPI, err := New(TestPath, TST)
	sm := smAPI.(*storageManager)
	if err != nil {
		t.Error(err.Error())
		return nil
	}

	var wg sync.WaitGroup
	// add all the specify folders
	for i := 0; i < folderNum; i++ {
		wg.Add(1)
		//create the storage folder
		go func(f string) {
			defer wg.Done()
			if err := sm.AddStorageFolder(f, SectorSize*64); err != nil {
				// if the error is not caused by the already existence of error
				if err != ErrFolderAlreadyExist {
					t.Error(err.Error())
				}
			}
		}(TestPath + "folders" + strconv.Itoa(i))
	}

	wg.Wait()

	return sm
}
