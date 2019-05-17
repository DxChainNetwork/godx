package storagemanager

import (
	"errors"
	"sync/atomic"
)

// addStorageSector add the storage sector to the system
func (sm *storageManager) addStorageSector(id sectorID, data []byte) error {
	// check if the data is the same as the sector size
	if uint64(len(data)) != SectorSize {
		return errors.New("sector data does not match the expected size")
	}

	// lock the wal while adding the sector to folder
	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// find the existing folder which is health
	folders := sm.existingStorageFolder()

	// loop through the folder, find the first available one that could
	// be used to store the sector
	for len(folders) >= 1 {
		var folderIndex int
		err := func() error {
			// get a random storage folder with its index
			sf, idx := randomAvailableFolder(folders)
			folderIndex = idx
			if sf == nil || idx == -1 {
				// the error is sever, indicating the config is not consistent with the memory
				return errors.New("no available storage folder to store the sector data")
			}

			// the folder is already locked in randomAvailableFolder, defer the unlock of folder
			defer sf.folderLock.Unlock()

			// then find a random available sector
			sectorIndex, err := randomFreeSector(sf.usage)
			if err != nil {
				// directly return the error, which is really severe
				return err
			}

			// manage the sector index
			usageIdx := sectorIndex / granularity
			bitIndex := sectorIndex % granularity
			sf.usage[usageIdx].setUsage(uint16(bitIndex))
			sf.sectors++

			// mark the sector as actually free
			sf.freeSectors[id] = sectorIndex

			// TODO: not robust enough, it is better to record the
			//  attempt to change wal also before start the write sector

			// write the sector to disk
			err = writeSector(sf.sectorData, sectorIndex, data)
			if err != nil {
				sf.usage[usageIdx].clearUsage(uint16(bitIndex))
				sf.sectors--
				delete(sf.freeSectors, id)
				return err
			}

			sectorMeta := sectorPersist{
				Count:  1,
				ID:     id,
				Folder: sf.index,
				Index:  sectorIndex,
			}

			// log the change
			err = sm.wal.writeEntry(logEntry{
				SectorUpdate: []sectorPersist{sectorMeta}})
			if err != nil {
				// TODO: unable to write entry to wal, consider panic
				return err
			}

			if Mode == TST && MockFails["Add_SECTOR_EXIT"] {
				return ErrMock
			}

			// write the meta data
			err = writeSectorMeta(sf.sectorMeta, sectorMeta)
			if err != nil {
				sf.usage[usageIdx].clearUsage(uint16(bitIndex))
				sf.sectors--
				delete(sf.freeSectors, id)
				return err
			}

			storageSector := storageSector{
				index:         sectorIndex,
				storageFolder: sf.index,
				count:         1,
			}

			delete(sm.folders[sf.index].freeSectors, id)
			sm.sectors[id] = storageSector
			return nil
		}()

		if err != nil {
			if err == ErrMock {
				return ErrMock
			}

			if folderIndex == -1 {
				return err
			}
			// remove the available folder from list
			folders = append(folders[:folderIndex], folders[folderIndex+1:]...)
		}
		// if there is no error, sector is loaded and recorded
		break
	}

	if len(folders) < 1 {
		return errors.New("not enough storage remaining to accept sector")
	}

	return nil
}

// add the virtual sector
func (sm *storageManager) addVirtualSector(id sectorID, sector storageSector) error {
	if sector.count == 65535 {
		return errors.New("the number of virtual sector reach maximum")
	}

	// increase the virtual sector count
	sector.count++

	sectorMeta := sectorPersist{
		Count:  sector.count,
		Folder: sector.storageFolder,
		ID:     id,
		Index:  sector.index,
	}

	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// check if the folder recorded in the meta data exist or not
	sf, exists := sm.folders[sectorMeta.Folder]
	if !exists || sf.atomicUnavailable == 1 {
		return errors.New("storage folder cannot found")
	}

	//
	err := sm.wal.writeEntry(logEntry{
		SectorUpdate: []sectorPersist{sectorMeta}})

	sm.sectors[id] = sector

	// write the metadata again

	err = writeSectorMeta(sf.sectorMeta, sectorMeta)
	if err != nil {
		// if there is error, revert the stage
		sectorMeta.Count--
		sector.count--
		err = sm.wal.writeEntry(logEntry{
			SectorUpdate: []sectorPersist{sectorMeta}})
		if err != nil {
			// TODO: consider panic because cannot write into wal
			return err
		}
		// update in memory
		sm.sectors[id] = sector
		return errors.New("unable to write sector metadata")
	}

	return nil
}

// removeSector remove the sector by given sector id
func (sm *storageManager) removeSector(id sectorID) error {
	// if the switch is top, indicate the storage manager is shut down,
	// would no longer receive any operation request, return an error
	if atomic.LoadUint64(&sm.atomicSwitch) == 1 {
		return errors.New("storage manager is shut down")
	}

	sm.wg.Add(1)
	defer sm.wg.Done()

	sm.wal.lock.Lock()
	defer sm.wal.lock.Unlock()

	// find the sector by given id
	sector, exist := sm.sectors[id]
	if !exist {
		return errors.New("cannot find the sector by given id")
	}

	// find the folder by given sector
	folder, exist := sm.folders[sector.storageFolder]
	// if the folder does not exist or the folder marked as cannot use
	if !exist || atomic.LoadUint64(&folder.atomicUnavailable) == 1 {
		return errors.New("cannot find the folder, or the folder is damaged")
	}

	// decrease the number of sector
	sector.count--

	sectorMeta := sectorPersist{
		Count:  sector.count,
		ID:     id,
		Folder: sector.storageFolder,
		Index:  sector.index,
	}

	// log the sector update to wal entry
	err := sm.wal.writeEntry(logEntry{
		SectorUpdate: []sectorPersist{sectorMeta}})
	if err != nil {
		// TODO: consider panic because cannot write into wal
		return err
	}

	if sector.count == 0 {
		delete(sm.sectors, id)

		// clear the usage and handle the free sector recording
		usageIndex := sector.index / granularity
		bitIndex := sector.index % granularity
		folder.usage[usageIndex].clearUsage(uint16(bitIndex))

		// clear the recorded free sector
		delete(folder.freeSectors, id)
	} else {
		// update the sector
		sm.sectors[id] = sector

		// update the sector metadata
		err = writeSectorMeta(folder.sectorMeta, sectorMeta)
		if err != nil {
			// if write the sector metadata fail, revert
			sectorMeta.Count++
			sector.count++

			err := sm.wal.writeEntry(logEntry{
				SectorUpdate: []sectorPersist{sectorMeta}})
			if err != nil {
				// TODO: consider panic because cannot write into wal
				return err
			}

			// load back the change to memory
			sm.sectors[id] = sector
			return errors.New("fail to write sector metadata")
		}
	}

	return nil
}
