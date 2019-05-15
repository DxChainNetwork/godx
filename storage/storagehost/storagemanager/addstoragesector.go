package storagemanager

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"unsafe"

	"os"
)

// sectorID is the id of the sector
type sectorID [12]byte

// storageSector record the sector information for
// the system
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

// Lock lock the object
func (sl *sectorLock) Lock() {
	sl.lock.Lock()
}

// Unlock unlock the object
func (sl *sectorLock) Unlock() {
	sl.lock.Unlock()
}

// tryLock return true and lock when no lock on the object,
// return false and do nothing if there is lock on the object
func (sl *sectorLock) tryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&sl.lock)), 0, sectorMutex)
}

// lockSector try to lock the sector
func (sm *storageManager) lockSector(id sectorID) {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// check if the sector exist in the map
	sl, exists := sm.sectorLocks[id]
	// if exist, increase the waiting number
	if exists {
		sl.waiting++
	} else {
		// if does not exist, create an sector object and store in map
		sl = &sectorLock{
			waiting: 1,
		}
		sm.sectorLocks[id] = sl
	}

	// wait until the sector is available
	sl.Lock()
}

// unlockSector unlock the sector
func (sm *storageManager) unlockSector(id sectorID) {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// check if the sector lock exist in the map
	sl, exists := sm.sectorLocks[id]
	if !exists {
		// the sector should exist, because this is an operation for unlock
		// TODO: log severe
		return
	}

	// decrease the waiting number
	sl.waiting--
	sl.Unlock()

	// check if the waiting number is zero, if does, remove the sector lock from map
	if sl.waiting == 0 {
		delete(sm.sectorLocks, id)
	}
}

// TODO: currently use go hash32, not sure how exactly which hash would be used
// getSectorID calculate the hash value given the sector root and sector salt
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

			// log the change
			err = sm.wal.writeEntry(logEntry{
				SectorUpdate: []sectorPersist{sectorMeta}})
			if err != nil {
				// TODO: unable to write entry to wal, consider panic
				return err
			}

			if Mode == TST && MockFails["Add_SECTOR_EXIT"] {
				return ErrMock
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

			delete(sm.folders[sf.index].freeSectors, id)
			sm.sectors[id] = storageSector
			return nil
		}()

		if err != nil {
			if err == ErrMock {
				return ErrMock
			}

			if folderIndex == -1 {
				return err
			}
			// remove the available folder from list
			folders = append(folders[:folderIndex], folders[folderIndex+1:]...)
		}
		// if there is no error, sector is loaded and recorded
		break
	}

	if len(folders) < 1 {
		return errors.New("not enough storage remaining to accept sector")
	}

	return nil
}

func (sm *storageManager) addVirtualSector(id sectorID, sector storageSector) error {
	if sector.count == 65535 {
		return errors.New("the number of virtual sector reach maximum")
	}

	sector.count++

	sectorMeta := sectorPersist{
		Count:  sector.count,
		Folder: sector.storageFolder,
		ID:     id,
		Index:  sector.index,
	}

	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	sf, exists := sm.folders[sectorMeta.Folder]
	if !exists || sf.atomicUnavailable == 1 {
		return errors.New("storage folder cannot found")
	}

	err := sm.wal.writeEntry(logEntry{
		SectorUpdate: []sectorPersist{sectorMeta}})

	sm.sectors[id] = sector

	// write the metadata again

	err = writeSectorMeta(sf.sectorMeta, sectorMeta)
	if err != nil {
		// if there is error, revert the stage
		sectorMeta.Count--
		sector.count--
		err = sm.wal.writeEntry(logEntry{
			SectorUpdate: []sectorPersist{sectorMeta}})
		if err != nil {
			// TODO: consider panic because cannot write into wal
		}
		// update in memory
		sm.sectors[id] = sector
		return errors.New("unable to write sector metadata")
	}

	return nil
}

func (sm *storageManager) removeSector(id sectorID) error {
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// find the sector by given id
	sector, exist := sm.sectors[id]
	if !exist {
		return errors.New("cannot find the sector by given id")
	}

	// find the folder by given sector
	folder, exist := sm.folders[sector.storageFolder]
	// if the folder does not exist or the folder marked as cannot use
	if !exist || atomic.LoadUint64(&folder.atomicUnavailable) == 1 {
		return errors.New("cannot find the folder, or the folder is damaged")
	}

	// decrease the number of sector
	sector.count--

	sectorMeta := sectorPersist{
		Count:  sector.count,
		ID:     id,
		Folder: sector.storageFolder,
		Index:  sector.index,
	}

	// log the sector update to wal entry
	err := sm.wal.writeEntry(logEntry{
		SectorUpdate: []sectorPersist{sectorMeta}})
	if err != nil {
		// TODO: consider panic because cannot write into wal
	}

	if sector.count == 0 {
		delete(sm.sectors, id)
		// TODO: mark the free sector, handling
		folder.freeSectors[id] = sector.index

		// clear the usage and handle the free sector recording
		usageIndex := sector.index / granularity
		bitIndex := sector.index % granularity
		folder.usage[usageIndex].clearUsage(uint16(bitIndex))

		// clear the recorded free sector
		delete(folder.freeSectors, id)
	} else {
		// update the sector
		sm.sectors[id] = sector

		// update the sector metadata
		// TODO: recover the metadata according the entry
		err = writeSectorMeta(folder.sectorMeta, sectorMeta)
		if err != nil {
			// if write the sector metadata fail, revert
			sectorMeta.Count++
			sector.count++

			err := sm.wal.writeEntry(logEntry{
				SectorUpdate: []sectorPersist{sectorMeta}})
			if err != nil {
				// TODO: consider panic because cannot write into wal
			}

			// load back the change to memory
			sm.sectors[id] = sector
			return errors.New("fail to write sector metadata")
		}
	}

	return nil
}

// recoverSector recover the sector object through wal
func (sm *storageManager) recoverSector(sectorMeta []sectorPersist) {

	for _, ss := range sectorMeta {
		sf, exists := sm.folders[ss.Folder]
		if !exists || atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
			// TODO: log sever
			fmt.Println("Unable to get storage folder")
			return
		}

		usageIndex := ss.Index / granularity
		bitIndex := ss.Index % granularity

		if ss.Count == 0 {
			sf.usage[usageIndex].clearUsage(uint16(bitIndex))
			return
		}

		// write the sector metadata again
		err := writeSectorMeta(sf.sectorMeta, ss)
		if err != nil {
			// TODO: log sever
			return
		}

		// set the usage back
		sf.usage[usageIndex].setUsage(uint16(bitIndex))

		sm.sectors[ss.ID] = storageSector{
			index:         ss.Index,
			storageFolder: ss.Folder,
			count:         ss.Count,
		}
	}
}

func findProcessedSectorUpdate(entries []logEntry) []sectorPersist {
	sectorsUpdate := make([]sectorPersist, 0)
	for _, entry := range entries {
		for _, ss := range entry.SectorUpdate {
			sectorsUpdate = append(sectorsUpdate, ss)
		}
	}

	return sectorsUpdate
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
