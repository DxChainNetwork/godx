package storagemanager

import (
	"encoding/binary"
	"github.com/pkg/errors"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"unsafe"

	"os"
)

type sectorID [12]byte

type storageSector struct {
	index         uint32
	storageFolder uint16
	count         uint16
}

const sectorMutex = 1 << iota

type sectorLock struct {
	lock    sync.Mutex
	waiting int
}

func (sl *sectorLock) Lock() {
	sl.lock.Lock()
}

func (sl *sectorLock) Unlock() {
	sl.lock.Unlock()
}

func (sl *sectorLock) tryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&sl.lock)), 0, sectorMutex)
}

func (sm *storageManager) lockSector(id sectorID) {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	sl, exists := sm.sectorLocks[id]
	if exists {
		sl.waiting++
	} else {
		sl = &sectorLock{
			waiting: 1,
		}
		sm.sectorLocks[id] = sl
	}

	sl.Lock()
}

func (sm *storageManager) unlockSector(id sectorID) {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	sl, exists := sm.sectorLocks[id]
	if !exists {
		// TODO: log severe
		return
	}

	sl.waiting--
	sl.Unlock()

	if sl.waiting == 0 {
		delete(sm.sectorLocks, id)
	}
}

// TODO: currently use go hash32
// TODO: not a valid sha. mock right now
func (sm *storageManager) getSectorID(sectorRoot [32]byte) (id sectorID) {
	h := fnv.New32a()
	data := append(sectorRoot[:], sm.sectorSalt[:]...)
	_, err := h.Write(data)
	if err != nil {
		// TODO: consider panic or log or redo
	}

	data = h.Sum(data[:0])

	copy(id[:], data[:])
	return id
}

// addStorageSector add the storage sector to the system
func (sm *storageManager) addStorageSector(id sectorID, data []byte) error {
	// check if the data is the same as the sector size
	if uint64(len(data)) != SectorSize {
		return errors.New("sector data does not match the expected size")
	}

	// lock the wal while adding the sector to folder
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// find the existing folder which is health
	folders := sm.existingStorageFolder()

	// loop through the folder, find the first available one that could
	// be used to store the sector
	for len(folders) >= 1 {
		var folderIndex int
		err := func() error {
			// get a random storage folder with its index
			sf, idx := randomAvailableFolder(folders)
			folderIndex = idx
			if sf == nil || idx == -1 {
				// the error is sever, indicating the config is not consistent with the memory
				return errors.New("no available storage folder to store the sector data")
			}

			// the folder is already locked in randomAvailableFolder, defer the unlock of folder
			defer sf.fLock.Unlock()

			// then find a random available sector
			sectorIndex, err := randomFreeSector(sf.usage)
			if err != nil {
				// directly return the error, which is really severe
				return err
			}

			// manage the sector index
			usageIdx := sectorIndex / granularity
			bitIndex := sectorIndex % granularity
			sf.usage[usageIdx].setUsage(uint16(bitIndex))
			sf.sectors++

			// mark the sector as actually free
			sf.freeSectors[id] = sectorIndex

			// write the sector to disk
			err = writeSector(sf.sectorData, sectorIndex, data)
			if err != nil {
				sf.usage[usageIdx].clearUsage(uint16(bitIndex))
				sf.sectors--
				delete(sf.freeSectors, id)
				return err
			}

			sectorMeta := sectorPersist{
				Count:  1,
				ID:     id,
				Folder: sf.index,
				Index:  sectorIndex,
			}

			// write the meta data
			err = writeSectorMeta(sf.sectorMeta, sectorMeta)
			if err != nil {
				sf.usage[usageIdx].clearUsage(uint16(bitIndex))
				sf.sectors--
				delete(sf.freeSectors, id)
				return err
			}

			storageSector := storageSector{
				index:         sectorIndex,
				storageFolder: sf.index,
				count:         1,
			}

			// log the change
			err = sm.wal.writeEntry(logEntry{
				PrepareAddStorageSector: []sectorPersist{sectorMeta}})
			if err != nil {
				// TODO: unable to write entry to wal, consider panic
				return err
			}

			delete(sm.folders[sf.index].freeSectors, id)
			sm.sectors[id] = storageSector
			return nil
		}()

		if err != nil {
			if folderIndex == -1 {
				return err
			}
			// remove the available folder from list
			folders = append(folders[:folderIndex], folders[folderIndex+1:]...)
		}
		// if there is no error, sector is loaded and recorded
		break
	}

	return nil
}


func (sm *storageManager) addVirtualSector(id sectorID, sector storageSector) error{
	if sector.count == 65535{
		return errors.New("the number of virtual sector reach maximum")
	}

	sector.count ++

	su := sectorPersist{
		Count: sector.count,
		Folder: sector.storageFolder,
		ID: id,
		Index: sector.index,
	}

	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	sf, exists := sm.folders[su.Folder]
	if !exists || sf.atomicUnavailable == 1{
		return errors.New("storage folder cannot found")
	}

	err := sm.wal.writeEntry(logEntry{
		PrepareAddStorageSector: []sectorPersist{su}})

	sm.sectors[id] = sector

	// write the metadata again

	err = writeSectorMeta(sf.sectorMeta, su)
	if err != nil{
		// if there is error, revert the stage
		su.Count --
		sector.count --
		err = sm.wal.writeEntry(logEntry{
			PrepareAddStorageSector: []sectorPersist{su}})
		if err != nil{
			// TODO: consider panic because cannot write into wal
		}
		// update in memory
		sm.sectors[id] = sector
		return errors.New("unable to write sector metadata")
	}

	return nil
}

// write the sector data to the specific location
func writeSector(file *os.File, sectorIndex uint32, data []byte) error {
	_, err := file.WriteAt(data, int64(uint64(sectorIndex)*SectorSize))
	if err != nil {
		return err
	}

	return nil
}

func writeSectorMeta(file *os.File, sectorMeta sectorPersist) error {
	data := make([]byte, SectorMetaSize)
	copy(data, sectorMeta.ID[:])
	binary.LittleEndian.PutUint16(data[12:], sectorMeta.Count)
	_, err := file.WriteAt(data, int64(SectorMetaSize*uint64(sectorMeta.Index)))
	if err != nil {
		// TODO: log
		return err
	}
	return nil
}
