// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagemanager

import (
	"io"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/rlp"
	"golang.org/x/crypto/sha3"
)

type (
	// sector is the metadata of a sector
	sector struct {
		// each sector is associated with an ID
		id sectorID

		// location field. The combination of folder and index field
		// could locate the location of the sector
		folderID folderID
		index    uint64

		// count is the number of times the sector is used
		count uint64
	}

	// sectorID is the type of sector ID, which is the common hash
	sectorID common.Hash

	// sectorPersist is the structure to be stored in database.
	sectorPersist struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

// calculateSectorID hash the sector salt and the merkle root to get the sector id
func (sm *storageManager) calculateSectorID(root common.Hash) (id sectorID) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(sm.sectorSalt[:])
	hasher.Write(root[:])
	hasher.Sum(id[:0])
	return id
}

// EncodeRLP defines the encode rule of the sector structure
// Note the id field is not encoded
func (s *sector) EncodeRLP(w io.Writer) (err error) {
	sp := sectorPersist{
		FolderID: s.folderID,
		Index:    s.index,
		Count:    s.count,
	}
	return rlp.Encode(w, sp)
}

// DecodeRLP defines the decode rule of the sector structure.
// Note the id field is not decoded
func (s *sector) DecodeRLP(st *rlp.Stream) (err error) {
	var sp sectorPersist
	if err = st.Decode(&sp); err != nil {
		return
	}
	s.folderID, s.index, s.count = sp.FolderID, sp.Index, sp.Count
	return
}
