package storagemanager

import (
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

// storage Folder is an object representation of folder
// for storing the files and sectors
type storageFolder struct {
	// atomicUnavailable mark if the folder is damaged or not
	atomicUnavailable uint64

	// index represent the index of the folder
	index uint16
	// path represent the path of the folder
	path string
	// usage mark the usage, every 64 sector come to form a BitVector
	// represent in decimal, but use as binary
	usage []BitVector

	// free Sectors mark the sector actually free but marked as used
	// in usage
	freeSectors map[sectorID]uint32

	// sector is the number of sector in this folder
	sectors uint64

	// meta data point to an os file storing the metadata of sector
	sectorMeta *os.File

	// data point to an os file storing the data of sector
	sectorData *os.File

	// folderLock locked the storage folder to prevent racing
	folderLock *folderLock
}

const folderMutex = 1 << iota

// folder lock to lock a folder, implement locker
type folderLock struct {
	lock sync.Mutex
}

func (fl *folderLock) Lock() {
	fl.lock.Lock()
}

func (fl *folderLock) Unlock() {
	fl.lock.Unlock()
}

// TryLock try to lock the folder if there is no lock and return true
// if it is already locked, do nothing and return false
func (fl *folderLock) TryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&fl.lock)), 0, folderMutex)
}

// existingStorageFolder find all the existing storage folder
func (sm *storageManager) existingStorageFolder() []*storageFolder {
	sfs := make([]*storageFolder, 0)
	// loop through the folder map, add all available folder to the slice
	for _, sf := range sm.folders {
		// check if the folder is available or not
		if atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
			continue
		}
		sfs = append(sfs, sf)
	}

	return sfs
}
