package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/DxChainNetwork/godx/common"
)

type (
	// folderManager is the map from folder id to storage folder
	folderManager struct {
		sfs  map[folderID]*storageFolder
		lock sync.Mutex
	}

	storageFolder struct {
		// storageFolder has an random id
		id folderID

		// unavailable is the atomic field mark if the folder is damaged or not
		// 0 - available
		// 1 - unavailable
		unavailable uint32

		// Path represent the Path of the folder
		path string

		// Usage mark the Usage, every 64 sector come to form a BitVector
		// represent in decimal, but use as binary
		usage []BitVector

		// TODO: Remove or add back freeSectors
		//// free Sectors mark the sector actually free but marked as used
		//// in Usage
		//freeSectors map[sectorID]uint32

		// sector is the number of sector in this folder
		numSectors uint64

		// folderLock locked the storage folder to prevent racing
		lock *common.TryLock

		// dataFile is the file where all the data sectors locates
		dataFile *os.File
	}

	// storageFolderPersist defines the persist data to be stored in database
	// The data is stored as "storagefolder_${folderID}" -> storageFolderPersist
	storageFolderPersist struct {
		Path       string
		Usage      []BitVector
		NumSectors uint64
	}

	folderID uint32
)

// EncodeRLP defines the encode rule of the storage folder
func (sf *storageFolder) EncodeRLP(w io.Writer) (err error) {
	sfp := storageFolderPersist{
		Path:       sf.path,
		Usage:      sf.usage,
		NumSectors: sf.numSectors,
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
	sf.path, sf.usage, sf.numSectors = sfp.Path, sfp.Usage, sfp.NumSectors
	sf.unavailable = folderAvailable
	return nil
}

// load load the storage folder data file.
func (sf *storageFolder) load() (err error) {
	datafilePath := filepath.Join(sf.path, dataFileName)
	fileInfo, err := os.Stat(datafilePath)
	if os.IsNotExist(err) {
		atomic.StoreUint32(&sf.unavailable, folderUnavailable)
		err = errors.New("data file not exist")
		return
	}
	if fileInfo.Size() < int64(len(sf.usage)-1)*int64(storage.SectorSize)*64 {
		atomic.StoreUint32(&sf.unavailable, folderUnavailable)
		err = errors.New("file size too small")
		return
	}
	if sf.dataFile, err = os.Open(datafilePath); err != nil {
		atomic.StoreUint32(&sf.unavailable, folderUnavailable)
		return
	}
	return
}

// close close all files in the storage folders
func (fm *folderManager) close() (err error) {
	fm.lock.Lock()
	defer fm.lock.Unlock()

	for _, sf := range fm.sfs {
		err = common.ErrCompose(err, sf.dataFile.Close())
	}
	return
}
