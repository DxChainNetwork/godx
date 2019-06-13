package newstoragemanager

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/syndtr/goleveldb/leveldb"
)

//readSector read the sector data
func (sm *storageManager) readSector(root common.Hash) (data []byte, err error) {
	// calculate the sector id
	id := sm.calculateSectorID(root)
	// lock the sector
	sm.sectorLocks.lockSector(id)
	defer sm.sectorLocks.unlockSector(id)
	// get the sector from database
	var s *sector
	s, err = sm.db.getSector(id)
	if err != nil {
		if err == leveldb.ErrNotFound {
			err = ErrNotFound
		}
		return
	}
	folderID, index := s.folderID, s.index
	// get the folder path
	folderPath, err := sm.db.getFolderPath(folderID)
	if err != nil {
		return nil, fmt.Errorf("db data might be corrupted: %v", err)
	}
	// Get the folder from memory
	sm.folders.lock.RLock()
	folder, err := sm.folders.get(folderPath)
	sm.folders.lock.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("check folder in memory: %v", err)
	}
	defer folder.lock.Unlock()
	if folder.status == folderUnavailable {
		return nil, fmt.Errorf("folder status unavailable")
	}

	// Read the data from folder
	data = make([]byte, storage.SectorSize)
	n, err := folder.dataFile.ReadAt(data, int64(index*storage.SectorSize))
	if uint64(n) != storage.SectorSize {
		return nil, fmt.Errorf("cannot read the sector: read %v bytes, expect %v bytes", n, storage.SectorSize)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the sector: %v", err)
	}
	return
}
