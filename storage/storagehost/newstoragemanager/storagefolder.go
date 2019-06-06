package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/rlp"
	"io"

	"github.com/DxChainNetwork/godx/common"
)

type (
	storageFolder struct {
		id folderID

		// atomicUnavailable mark if the folder is damaged or not
		atomicUnavailable uint64

		// path represent the path of the folder
		path string
		// usage mark the usage, every 64 sector come to form a BitVector
		// represent in decimal, but use as binary
		usage []BitVector

		// TODO: Remove or add back freeSectors
		//// free Sectors mark the sector actually free but marked as used
		//// in usage
		//freeSectors map[sectorID]uint32

		// sector is the number of sector in this folder
		numSectors uint64

		// folderLock locked the storage folder to prevent racing
		lock *common.TryLock
	}

	// storageFolderPersist defines the persist data to be stored in database
	storageFolderPersist struct {
		path       string
		usage      []BitVector
		numSectors uint64
	}

	folderID uint32
)

// EncodeRLP defines the encode rule of the storage folder
func (sf *storageFolder) EncodeRLP(w io.Writer) (err error) {
	sfp := storageFolderPersist{
		path:       sf.path,
		usage:      sf.usage,
		numSectors: sf.numSectors,
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
	sf.path, sf.usage, sf.numSectors = sfp.path, sfp.usage, sfp.numSectors
	return nil
}
