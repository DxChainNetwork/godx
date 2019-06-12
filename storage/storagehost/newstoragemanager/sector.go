package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"golang.org/x/crypto/sha3"
)

type (
	// sector is the metadata of a sector
	sector struct {
		// each sector is associated with an ID
		id sectorID

		// location field. The combination of folder and index field
		// could locate the location of the sector
		folder string
		index  uint64

		// count is the number of times the sector is used
		count uint64
	}

	// sectorID is the type of sector ID, which is the common hash
	sectorID common.Hash

	// sectorPersist is the structure to be stored in database.
	sectorPersist struct {
		Folder string
		Index  uint64
		Count  uint64
	}
)

// calculateSectorID hash the sector salt and the merkle root to get the sector id
func (sm *storageManager) calculateSectorID(mk common.Hash) (id sectorID) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(sm.sectorSalt[:])
	hasher.Write(mk[:])
	hasher.Sum(id[:0])
	return id
}
