package newstoragemanager

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"io"
	"os"
	"path/filepath"
)

type (
	storageFolder struct {
		// id is a uint32 associated with a folder. It is randomly generated
		// unique key of a folder.
		id folderID

		// status is the atomic field mark if the folder is damaged or not
		// folderAvailable / folderUnavailable
		status uint32

		// Path represent the Path of the folder
		path string

		// Usage mark the Usage, every 64 sector come to form a bitVector
		// represent in decimal, but use as binary
		usage []bitVector

		// TODO: Remove or add back freeSectors
		//// free Sectors mark the sector actually free but marked as used
		//// in Usage
		//freeSectors map[sectorID]uint32

		// sector is the total number of sector in this folder
		numSectors uint64

		// StoredSectors is the number of sectors stored in the folder
		storedSectors uint64

		// folderLock locked the storage folder to prevent racing
		lock common.TryLock

		// dataFile is the file where all the data sectors locates
		dataFile *os.File
	}

	// storageFolderPersist defines the persist data to be stored in database
	// The data is stored as "storagefolder_${folderID}" -> storageFolderPersist
	storageFolderPersist struct {
		ID            uint32
		Path          string
		Usage         []bitVector
		NumSectors    uint64
		StoredSectors uint64
	}

	folderID uint32
)

// EncodeRLP defines the encode rule of the storage folder
func (sf *storageFolder) EncodeRLP(w io.Writer) (err error) {
	sfp := storageFolderPersist{
		ID:            uint32(sf.id),
		Path:          sf.path,
		Usage:         sf.usage,
		NumSectors:    sf.numSectors,
		StoredSectors: sf.storedSectors,
	}
	return rlp.Encode(w, sfp)
}

// DecodeRLP defines the decode rule of the storage folder.
// Note the decoded storageFolder index field is not filled by the rlp decode rule
func (sf *storageFolder) DecodeRLP(st *rlp.Stream) (err error) {
	var sfp storageFolderPersist
	if err = st.Decode(&sfp); err != nil {
		return
	}
	sf.id, sf.path, sf.usage, sf.numSectors, sf.storedSectors = folderID(sfp.ID), sfp.Path, sfp.Usage, sfp.NumSectors, sfp.StoredSectors
	sf.status = folderAvailable
	return
}

// load load the storage folder data file.
func (sf *storageFolder) load() (err error) {
	datafilePath := filepath.Join(sf.path, dataFileName)
	fileInfo, err := os.Stat(datafilePath)
	if os.IsNotExist(err) {
		sf.status = folderUnavailable
		err = errors.New("data file not exist")
		return
	}
	if fileInfo.Size() < int64(sf.numSectors)*int64(storage.SectorSize) {
		sf.status = folderUnavailable
		err = errors.New("file size too small")
		return
	}
	if sf.dataFile, err = os.Open(datafilePath); err != nil {
		sf.status = folderUnavailable
		return
	}
	return
}

// freeSectorIndex randomly find a free slot to insert the sector.
// If cannot find such a slot, return errFolderAlreadyFull
// Note this function must be called with lock protected
func (sf *storageFolder) freeSectorIndex() (index uint64, err error) {
	if sf.storedSectors >= sf.numSectors {
		return 0, errFolderAlreadyFull
	}
	startIndex := randomUint64() % sf.numSectors
	index = startIndex
	// Loop through all indexes
	for {
		usageIndex := index / bitVectorGranularity
		bitIndex := index % bitVectorGranularity
		if sf.usage[usageIndex] == math.MaxUint64 {
			// Skip to the next usage
			index = (usageIndex + 1) * bitVectorGranularity
			if index >= sf.numSectors {
				index = 0
			}
			continue
		}
		if sf.usage[usageIndex].isFree(bitIndex) {
			return
		}
		index++
		if index >= sf.numSectors {
			index = 0
		}
		// If index has returned to the starting point, break and return
		if index == startIndex {
			break
		}
	}
	return 0, errFolderAlreadyFull
}

// setFreeSectorSlot set the slot specified by the index to free.
// If the slot is already freed, report an error
// Note the storage folder must be locked to use this function
func (sf *storageFolder) setFreeSectorSlot(index uint64) (err error) {
	usageIndex := index / bitVectorGranularity
	bitIndex := index % bitVectorGranularity
	if sf.usage[usageIndex].isFree(bitIndex) {
		err = fmt.Errorf("sector slot %d already free", index)
		return
	}
	sf.usage[usageIndex].clearUsage(bitIndex)
	sf.storedSectors--
	return
}

// setUsedSectorSlot set the slot specified by the index to used
// If the slot is already used, report an error
// Note the storage folder must be locked to use this function
func (sf *storageFolder) setUsedSectorSlot(index uint64) (err error) {
	usageIndex := index / bitVectorGranularity
	bitIndex := index % bitVectorGranularity
	if !sf.usage[usageIndex].isFree(bitIndex) {
		err = fmt.Errorf("sector slot %d already occupied", index)
		return
	}
	sf.usage[usageIndex].setUsage(bitIndex)
	sf.storedSectors++
	return
}

// sizeToNumSectors convert the size to number of sectors
func sizeToNumSectors(size uint64) (numSectors uint64) {
	numSectors = size / storage.SectorSize
	return
}

// numSectorsToSize convert the numSectors to size.
func numSectorsToSize(numSectors uint64) (size uint64) {
	size = numSectors * storage.SectorSize
	return
}

// randomUint64 create a random uint64
func randomUint64() (num uint64) {
	b := make([]byte, 8)
	rand.Read(b)
	return binary.LittleEndian.Uint64(b)
}
