package newstoragemanager

import (
	"errors"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/DxChainNetwork/godx/common"
)

type (
	storageFolder struct {
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
		lock *common.TryLock

		// dataFile is the file where all the data sectors locates
		dataFile *os.File
	}

	// storageFolderPersist defines the persist data to be stored in database
	// The data is stored as "storagefolder_${folderID}" -> storageFolderPersist
	storageFolderPersist struct {
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
		return err
	}
	sf.path, sf.usage, sf.numSectors, sf.storedSectors = sfp.Path, sfp.Usage, sfp.NumSectors, sfp.StoredSectors
	sf.status = folderAvailable
	sf.lock = common.NewTryLock()
	return nil
}

// load load the storage folder data file.
func (sf *storageFolder) load() (err error) {
	datafilePath := filepath.Join(sf.path, dataFileName)
	fileInfo, err := os.Stat(datafilePath)
	if os.IsNotExist(err) {
		atomic.StoreUint32(&sf.status, folderUnavailable)
		err = errors.New("data file not exist")
		return
	}
	if fileInfo.Size() < int64(sf.numSectors)*int64(storage.SectorSize) {
		atomic.StoreUint32(&sf.status, folderUnavailable)
		err = errors.New("file size too small")
		return
	}
	if sf.dataFile, err = os.Open(datafilePath); err != nil {
		atomic.StoreUint32(&sf.status, folderUnavailable)
		return
	}
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
