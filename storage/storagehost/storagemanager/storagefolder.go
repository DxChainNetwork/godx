package storagemanager

import (
	"os"
	"sync"
	"sync/atomic"
	"unsafe"
)

type storageFolder struct {
	atomicUnavailable uint64
	//atomicConstructing	uint64

	index uint16
	path  string
	usage []BitVector

	// TODO: free sector have not init
	freeSectors map[sectorID]uint32
	sectors     uint64

	sectorMeta *os.File
	sectorData *os.File

	fLock folderLock
}

const folderMutex = 1 << iota

type folderLock struct {
	lock sync.Mutex
}

func (fl *folderLock) Lock() {
	fl.lock.Lock()
}

func (fl *folderLock) Unlock() {
	fl.lock.Unlock()
}

func (fl *folderLock) TryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&fl.lock)), 0, folderMutex)
}

func (sm *storageManager) existingStorageFolder() []*storageFolder {
	sfs := make([]*storageFolder, 0)
	for _, sf := range sm.folders {
		if atomic.LoadUint64(&sf.atomicUnavailable) == 1 {
			continue
		}

		sfs = append(sfs, sf)
	}

	return sfs
}
