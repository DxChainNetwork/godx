package storagehostmanager

import (
	"math/rand"
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

func (shm *StorageHostManager) scanStart() {
	err := shm.tm.Add()
	if err != nil {
		return
	}
	defer shm.tm.Done()

	// wait until the node is fully synced
	shm.waitSync()

	// get all storage hosts who have not been scanned before or no historical information
	allStorageHosts := shm.storageHostTree.All()
	shm.lock.Lock()
	for _, host := range allStorageHosts {
		if len(host.ScanRecords) == 0 && host.HistoricDowntime == 0 && host.HistoricUptime == 0 {
			shm.queueScan(host)
		}
	}
	shm.lock.Unlock()

	// wait until all current scan tasks to be finished
	shm.waitScanFinish()

	// finished the first scan
	shm.lock.Lock()
	shm.initialScanFinished = true
	shm.lock.Unlock()

	// schedule scan
	shm.scanSchedule()
}

// scanSchedule will filter out the online and offline hosts, and getting them
// into the scanning queue, prepare to be scanned
func (shm *StorageHostManager) scanSchedule() {
	for {
		var onlineHosts, offlineHosts []storage.HostInfo
		allStorageHosts := shm.storageHostTree.All()
		for _, host := range allStorageHosts {
			if len(onlineHosts) >= scanQuantity && len(offlineHosts) >= scanQuantity {
				break
			}

			// check if the storage host is online or offline
			// making sure the online hosts has higher chance to be scanned than offline hosts
			//  1. online: scanRecord > 0, last scan is success
			//  2. otherwise, offline
			scanRecordsLen := len(host.ScanRecords)
			online := scanRecordsLen > 0 && host.ScanRecords[scanRecordsLen-1].Success
			if online && len(onlineHosts) < scanQuantity {
				onlineHosts = append(onlineHosts, host)
			} else if !online && len(offlineHosts) < scanQuantity {
				offlineHosts = append(offlineHosts, host)
			}
		}

		// queued for scan
		shm.lock.Lock()
		for _, host := range onlineHosts {
			shm.queueScan(host)
		}

		for _, host := range offlineHosts {
			shm.queueScan(host)
		}
		shm.lock.Unlock()

		// sleep for a random amount of time, then schedule scan again
		rand.Seed(time.Now().UTC().UnixNano())
		randomSleepTime := time.Duration(rand.Intn(int(maxScanSleep-minScanSleep)) + int(minScanSleep))

		// sleep random amount of time
		select {
		case <-shm.tm.StopChan():
			return
		case <-time.After(randomSleepTime):
		}
	}
}

// queueScan will scan the storage host added
func (shm *StorageHostManager) queueScan(hi storage.HostInfo) {
	// TODO: (mzhang) to be implemented
}

// waitSync will pause the current go routine and wait until the
// node is fully synced
func (shm *StorageHostManager) waitSync() {
	// TODO: (mzhang) need to test this implementation
	for {
		sync, _ := shm.ethInfo.Syncing()
		syncing, ok := sync.(bool)
		if ok && !syncing {
			break
		}

		select {
		case <-shm.tm.StopChan():
			return
		case <-time.After(scanCheckDuration):
		}
	}
}

func (shm *StorageHostManager) waitScanFinish() {
	for {
		shm.lock.Lock()
		scanningTasks := len(shm.scanningList)
		shm.lock.Unlock()

		if scanningTasks == 0 {
			break
		}

		select {
		case <-shm.tm.StopChan():
		case <-time.After(scanCheckDuration):
		}
	}
}
