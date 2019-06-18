// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math/rand"
	"time"

	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

// scan will start the initial storage host scan and activate the auto scan service
func (shm *StorageHostManager) scan() {
	if err := shm.tm.Add(); err != nil {
		shm.log.Warn("Failed to launch scan task when initializing storage host manager")
		return
	}
	defer shm.tm.Done()

	// wait until the node is fully synced (no block chain change)
	// the information acquired will be more stable
	shm.waitSync()

	// get all storage hosts who have not been scanned before or no historical information
	allStorageHosts := shm.storageHostTree.All()
	for _, host := range allStorageHosts {
		if len(host.ScanRecords) == 0 {
			shm.scanValidation(host)
		}
	}

	// indicate the initial scan is finished
	shm.waitScanFinish()
	shm.lock.Lock()
	shm.initialScan = true
	shm.lock.Unlock()

	// scan automatically in a time range
	shm.autoScan()
}

// scanSchedule will filter out the online and offline hosts, and getting them
// into the scanning queue, prepare to be scanned
func (shm *StorageHostManager) autoScan() {
	for {
		var onlineHosts, offlineHosts []storage.HostInfo
		allStorageHosts := shm.storageHostTree.All()
		for _, host := range allStorageHosts {

			// check if the number of online hosts or the length of offlineHosts exceed
			// the max scan quantity
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

		// queued for scan, online storage host has higher
		// priority to be scanned than offline storage host
		for _, host := range onlineHosts {
			shm.scanValidation(host)
		}

		for _, host := range offlineHosts {
			shm.scanValidation(host)
		}

		// sleep for a random amount of time, then schedule scan again
		rand.Seed(time.Now().UTC().UnixNano())
		randomSleepTime := time.Duration(rand.Intn(int(maxScanSleep-minScanSleep)) + int(minScanSleep))
		shm.log.Debug("Random Sleep Time:", randomSleepTime)

		// sleep random amount of time
		select {
		case <-shm.tm.StopChan():
			return
		case <-time.After(randomSleepTime):
		}
	}
}

// scanValidation will scan the storage host added
func (shm *StorageHostManager) scanValidation(hi storage.HostInfo) {
	shm.log.Debug("Started Scan Validation")

	// verify if the storage host is already in scan pool
	shm.lock.Lock()
	defer shm.lock.Unlock()
	_, exists := shm.scanLookup[hi.EnodeID]
	if exists {
		return
	}

	// if not, add it to the scan look up and scan wait list
	shm.scanLookup[hi.EnodeID] = struct{}{}
	shm.scanWaitList = append(shm.scanWaitList, hi)

	// wait for another routine to scan the list
	if shm.scanWait {
		return
	}

	// start the scanning process
	go shm.scanStart()
}

// scanStart will update the scan wait list and scan look up map
// afterwards, the host needed to be scanned will be passed in through channel
// NOTE: multiple go routines will be activated to handle scan request
func (shm *StorageHostManager) scanStart() {
	// add go routine
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	scanWorker := make(chan storage.HostInfo)
	// used for scanExecute termination, once the channel closed
	// all the worker will be terminated
	defer close(scanWorker)

	for {
		shm.lock.Lock()
		// if there are no tasks need to be scanned anymore, exit
		if len(shm.scanWaitList) == 0 {
			shm.scanWait = false
			shm.lock.Unlock()
			return
		}

		// update the scan wait list and scan look up
		hostInfoTask := shm.scanWaitList[0]
		shm.scanWaitList = shm.scanWaitList[1:]
		delete(shm.scanLookup, hostInfoTask.EnodeID)
		workers := shm.scanningWorkers
		shm.lock.Unlock()

		// start the scan execution
		if workers < maxWorkersAllowed {
			go shm.scanExecute(scanWorker)
		}

		// send the task to the worker
		select {
		case scanWorker <- hostInfoTask:
		case <-shm.tm.StopChan():
			return
		}
	}
}

// scanExecute will check the local node online status, and start to updateHostSettings
// it will terminate along with termination of scan start
func (shm *StorageHostManager) scanExecute(scanWorker <-chan storage.HostInfo) {
	shm.log.Debug("Started Scan Execution")

	// add one more go routine
	if err := shm.tm.Add(); err != nil {
		return
	}
	defer shm.tm.Done()

	shm.lock.Lock()
	shm.scanningWorkers++
	shm.lock.Unlock()

	// keep reading the host information from the worker
	// and start to update its configuration
	for info := range scanWorker {
		shm.waitOnline()
		shm.updateHostConfig(info)
	}
	shm.lock.Lock()
	shm.scanningWorkers--
	shm.lock.Unlock()
}

// updateHostSettings will connect to the host, grabbing the settings,
// and update the host pool
func (shm *StorageHostManager) updateHostConfig(hi storage.HostInfo) {
	shm.log.Info("Started updating the storage host", "Host ID", hi.EnodeURL)

	// get the IP network and check if it is changed
	// this is needed because the storage host can change its settings directly
	ipnet, err := storagehosttree.IPNetwork(hi.IP)

	if err == nil && ipnet.String() != hi.IPNetwork {
		hi.IPNetwork = ipnet.String()
		hi.LastIPNetWorkChange = time.Now()
	} else if err != nil {
		shm.log.Error("failed to get the IP network information", "err", err.Error())
	}

	// update the historical interactions
	shm.lock.RLock()
	info := &hi
	blockHeight := shm.blockHeight
	shm.lock.RUnlock()

	hostHistoricInteractionsUpdate(info, blockHeight)

	// retrieve storage host external settings
	hostConfig, err := shm.retrieveHostConfig(hi)
	if err != nil {
		shm.log.Warn("failed to get storage host external setting", "hostID", hi.EnodeID, "err", err.Error())
	} else {
		hi.HostExtConfig = hostConfig
	}

	shm.lock.Lock()
	defer shm.lock.Unlock()

	// update the host information
	shm.hostInfoUpdate(hi, err)

	shm.log.Debug("Storage Host Information Updated", "enodeID", hi.EnodeID)
}

// retrieveHostSetting will establish connection to the corresponded storage host
// and get its configurations
func (shm *StorageHostManager) retrieveHostConfig(hi storage.HostInfo) (storage.HostExtConfig, error) {
	var config storage.HostExtConfig

	// send message, and get host setting
	err := shm.b.GetStorageHostSetting(hi.EnodeURL, &config)
	return config, err
}

// waitOnline will pause the current process and wait until the
// local node is connected with some peers (meaning the local node
// is online)
func (shm *StorageHostManager) waitOnline() {
	for {
		if shm.b.Online() {
			break
		}

		select {
		case <-time.After(scanOnlineCheckDuration):
		case <-shm.tm.StopChan():
			return
		}
	}
}

// waitSync will pause the current go routine and wait until the
// node is fully synced
func (shm *StorageHostManager) waitSync() {
	for {
		if !shm.b.Syncing() {
			break
		}

		select {
		case <-shm.tm.StopChan():
			return
		case <-time.After(scanCheckDuration):
		}
	}
}

// waitScanFinish will pause the current process until all the host stored in the scanWaitList
// got executed
func (shm *StorageHostManager) waitScanFinish() {
	for {
		shm.lock.Lock()
		scanningTasks := len(shm.scanWaitList)
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
