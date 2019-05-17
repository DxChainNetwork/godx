package storagemanager

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"

	"github.com/DxChainNetwork/godx/log"
)

// storageManager is used to manage the storage such as
// operation of adding a folder, delete a folder, receive
// new data and store in to sector
type storageManager struct {
	sectorSalt [32]byte

	// folders map the folder index to folder object created
	folders map[uint16]*storageFolder

	// sector map the sector id to sector object created
	sectors map[sectorID]storageSector

	// manage to lock the sector ID
	sectorLocks map[sectorID]*sectorLock

	log        log.Logger
	persistDir string
	wal        *writeAheadLog

	// stopChan is a for stopping the maintenance thread
	stopChan chan struct{}
	// atomicSwitch is a switch for manage to receive an operation or not
	atomicSwitch uint64
	// wg is wait group to avoid shut down without finishing of all operations
	wg *sync.WaitGroup
}

// New give caller an interface of Storage Manager, which
// implemented by the storage manager, and also start the
// maintenance loop for keep track of operation commit
func New(persistDir string, mode ...int) (StorageManager, error) {
	// check the mode, and init the value corresponding the mode
	if mode != nil && len(mode) != 0 {
		buildSetting(mode[0])
	}

	// TODO: if error, close all the resources
	return newStorageManager(persistDir)
}

// AddStorageFolder include mainly two steps: first check the validity of a folder
// then try to process the storage folder addition
func (sm *storageManager) AddStorageFolder(path string, size uint64) error {

	// if the switch is top, indicate the storage manager is shut down,
	// would no longer receive any operation request, return an error
	if atomic.LoadUint64(&sm.atomicSwitch) == 1 {
		return errors.New("storage manager is shut down")
	}

	// manage add thread group, indicate the operation is running,
	// even a shut down of storage manager is called, this process would
	// be finished then allow the manager to shut down
	sm.wg.Add(1)
	defer sm.wg.Done()

	var err error

	// prepareAddStorageFolder: first steps for commit, check the validity of
	// the folder path and size, and get record before process the adding operation
	sf, err := sm.prepareAddStorageFolder(path, size)
	if sf == nil || err != nil {
		return err
	}

	// process the adding operation, do file creation and truncation, after all things
	// done as expected, operation would be recorded in wal, and ready for final commit
	err = sm.processAddStorageFolder(sf)

	return err
}

// AddSector add the sector to the random folder at random index
func (sm *storageManager) AddSector(root [32]byte, sectorData []byte) error {
	// if the switch is top, indicate the storage manager is shut down,
	// would no longer receive any operation request, return an error
	if atomic.LoadUint64(&sm.atomicSwitch) == 1 {
		return errors.New("storage manager is shut down")
	}

	sm.wg.Add(1)
	defer sm.wg.Done()

	var err error
	// get the id for the sector
	// TODO: in test case, getSectorID may give existing id but different data
	id := sm.getSectorID(root)

	sm.lockSector(id)
	defer sm.unlockSector(id)

	sm.wal.lock.Lock()
	sector, exists := sm.sectors[id]
	sm.wal.lock.Unlock()

	if exists {
		err = sm.addVirtualSector(id, sector)
	} else {
		err = sm.addStorageSector(id, sectorData)
	}

	if err != nil {
		return err
	}

	return nil
}

// ReadSector read a sector by given root
func (sm *storageManager) ReadSector(root [32]byte) ([]byte, error) {
	// if the switch is top, indicate the storage manager is shut down,
	// would no longer receive any operation request, return an error
	if atomic.LoadUint64(&sm.atomicSwitch) == 1 {
		return nil, errors.New("storage manager is shut down")
	}

	sm.wg.Add(1)
	defer sm.wg.Done()

	var err error
	id := sm.getSectorID(root)

	sm.lockSector(id)
	defer sm.unlockSector(id)

	// get the sector
	ss, exists1 := sm.sectors[id]
	sf, exists2 := sm.folders[ss.storageFolder]

	if !exists1 {
		return nil, errors.New("unable to find sector meta")
	}

	if !exists2 {
		return nil, errors.New("unable to load storage folder")
	}

	if atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
		return nil, errors.New("unable to open the folder")
	}

	sectorData, err := readSector(sf.sectorData, ss.index)

	return sectorData, err
}

// Remove the sector
func (sm *storageManager) RemoveSector(id sectorID) error {
	return sm.removeSector(id)
}

// use the thread manager to close all the related process
func (sm *storageManager) Close() error {

	// check if already closed
	if atomic.LoadUint64(&sm.atomicSwitch) == 1 {
		return errors.New("storage manager already closed")
	}
	// close the switch, indicating the storage manager would no longer receive
	// any operation
	sm.wal.lock.Lock()
	atomic.StoreUint64(&sm.atomicSwitch, 1)
	sm.wal.lock.Unlock()

	// signal the maintenance threads to shut down
	// TODO: if this is called when the maintenance not running,
	//  may result an infinite waiting,
	sm.stopChan <- struct{}{}

	// wait until all running operation finish their job
	sm.wg.Wait()

	return nil
}

func readSector(file *os.File, sectorIndex uint32) ([]byte, error) {
	b := make([]byte, SectorSize)
	_, err := file.ReadAt(b, int64(uint64(sectorIndex)*SectorSize))
	if err != nil {
		return nil, errors.New("fail to read sector")
	}

	return b, nil
}
