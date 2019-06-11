package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common"
	"golang.org/x/crypto/sha3"
)

type (
	sectorID common.Hash
)

// calculateSectorID hash the sector salt and the merkle root to get the sector id
func (sm *storageManager) calculateSectorID(mk common.Hash) (id sectorID) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(sm.sectorSalt[:])
	hasher.Write(mk[:])
	hasher.Sum(id[:0])
	return id
}
