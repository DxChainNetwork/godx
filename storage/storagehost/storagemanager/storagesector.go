package storagemanager

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

// sectorID is the id of the sector
type sectorID [12]byte

// storageSector record the sector information for
// the system
type storageSector struct {
	// index is the index of storage sector
	index uint32

	// storageFolder is the index of storage folder
	// that store this current sector
	storageFolder uint16

	// count represent the current number of virtual sector
	count uint16
}

const sectorMutex = 1 << iota

// sectorLock use to lock the sector object
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

// recoverSector recover the sector object through wal
func (sm *storageManager) recoverSector(sectorMeta []sectorPersist) {
	// loop through the sector meta data
	for _, ss := range sectorMeta {
		// check if according the sector meta, the folder exist
		sf, exists := sm.folders[ss.Folder]
		// if not exist or the folder is damaged, return
		if !exists || atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
			// TODO: log sever
			fmt.Println("Unable to get storage folder")
			return
		}

		// compute the sector index
		usageIndex := ss.Index / granularity
		bitIndex := ss.Index % granularity

		// if the sector is marked as removed, clear the usage
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

		// map the sector id to the sector object
		sm.sectors[ss.ID] = storageSector{
			index:         ss.Index,
			storageFolder: ss.Folder,
			count:         ss.Count,
		}
	}
}

// findProcessedSectorUpdate loop through given entries and find
// the finished sector updat information
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

// writeSectorMeta write the sectors metadata to given file at specific location
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
