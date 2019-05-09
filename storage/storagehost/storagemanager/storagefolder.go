package storagemanager

import (
	"os"
	"sync"
)

type storageFolder struct {
	atomicUnavailable uint64
	//atomicConstructing	uint64

	index uint16
	path  string
	usage []BitVector

	sectorMeta *os.File
	sectorData *os.File

	lock sync.Mutex
}
