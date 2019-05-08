package storagehostmanager

import (
	"fmt"
	"github.com/DxChainNetwork/errors"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"math/rand"
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

func (shm *StorageHostManager) scan() {
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
			shm.scanQueue(host)
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
			shm.scanQueue(host)
		}

		for _, host := range offlineHosts {
			shm.scanQueue(host)
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

// scanQueue will scan the storage host added
func (shm *StorageHostManager) scanQueue(hi storage.HostInfo) {
	// verify if thestorage host is already in scan pool
	_, exists := shm.scanPool[hi.EnodeID.String()]
	if exists {
		return
	}

	// if not, add it to the pool and scanning list in a random position
	shm.scanPool[hi.EnodeID.String()] = struct{}{}
	shm.scanWaitList = append(shm.scanWaitList, hi)
	scanningLen := len(shm.scanWaitList)
	if scanningLen > 1 {
		rand.Seed(time.Now().UTC().UnixNano())
		randIndex := rand.Intn(scanningLen - 1)
		shm.scanWaitList[scanningLen-1], shm.scanWaitList[randIndex] = shm.scanWaitList[randIndex], shm.scanWaitList[scanningLen-1]
	}

	// wait for another routine to scan the list
	if shm.scanWait {
		return
	}

	// start the scanning process
	go shm.scanStart()
}

func (shm *StorageHostManager) scanStart() {
	scanWorker := make(chan storage.HostInfo)
	defer close(scanWorker)

	if err := shm.tm.Add(); err != nil {
		return
	}

	defer shm.tm.Done()

	starterRoutine := false

	for {
		shm.lock.Lock()

		// no scan in the wait list
		if len(shm.scanWaitList) == 0 {
			shm.scanWait = false
			shm.lock.Unlock()
			return
		}

		// get the host information
		hostinfo := shm.scanWaitList[0]
		shm.scanWaitList = shm.scanWaitList[1:]
		delete(shm.scanPool, hostinfo.EnodeID.String())

		// based on the host information, get the recent information stored in the tree
		storedInfo, exists := shm.storageHostTree.RetrieveHostInfo(hostinfo.EnodeID.String())
		if exists {
			hostinfo = storedInfo
		}

		// send the host information to the worker
		select {
		case scanWorker <- hostinfo:
			shm.lock.Unlock()
			continue
		default:
		}

		// making sure the start routine will always get access
		if shm.scanningRoutines < maxScanningRoutines || !starterRoutine {
			starterRoutine = true
			shm.scanningRoutines++
			if err := shm.tm.Add(); err != nil {
				shm.lock.Unlock()
				return
			}

			// start the scan execution
			go func() {
				defer shm.tm.Done()
				shm.scanExecute(scanWorker)
				shm.lock.Lock()
				shm.scanningRoutines--
				shm.lock.Unlock()
			}()
		}

		// unlock is needed if failed to go through the if statement
		shm.lock.Unlock()

		// block until the worker is available
		select {
		case scanWorker <- hostinfo:
		case <-shm.tm.StopChan():
			return
		}
	}
}

// scanExecute will check the local node online status, and start to updateHostSettings
func (shm *StorageHostManager) scanExecute(scanWorker <-chan storage.HostInfo) {
	for info := range scanWorker {
		shm.waitOnline()
		shm.updateHostConfig(info)
	}
}

// updateHostSettings will connect to the host, grabbing the settings,
// and update the host pool
func (shm *StorageHostManager) updateHostConfig(hi storage.HostInfo) {
	shm.log.Info("Started scanning the storage host %s", hi.EnodeURL)

	// get the IP network and check if it is changed
	ipnet, err := storagehosttree.IPNetwork(hi.IP)

	if err == nil && ipnet.String() != hi.IPNetwork {
		hi.IPNetwork = ipnet.String()
		hi.LastIPNetWorkChange = time.Now()
	}

	if err != nil {
		log.Debug("updateHostConfig: failed to get the IP network information", err.Error())
	}

	// update the historical interactions
	shm.lock.RLock()
	hostHistoricInteractionsUpdate(&hi, shm.blockHeight)
	shm.lock.RUnlock()

	// retrieve storage host external settings
	hostConfig, err := shm.retrieveHostConfig(hi)
	if err != nil {
		log.Error("failed to get storage host %v external setting: %s", hi.EnodeURL, err.Error())
	} else {
		hi.HostExtConfig = hostConfig
	}

	shm.lock.Lock()
	defer shm.lock.Unlock()

	// update the host information
	shm.hostInfoUpdate(hi, err)
}

// retrieveHostSetting will establish connection to the corresponded storage host
// and get its configurations
func (shm *StorageHostManager) retrieveHostConfig(hi storage.HostInfo) (storage.HostExtConfig, error) {
	// establish connection to the peer
	node, err := enode.ParseV4(hi.EnodeURL)
	if err != nil {
		return storage.HostExtConfig{}, errors.New("invalid enode")
	}
	shm.p2pServer.AddPeer(node)

	// extract peer ID
	peerID := fmt.Sprintf("%x", hi.EnodeID.Bytes()[:8])

	var config storage.HostExtConfig

	// send message, and get host setting
	err = shm.b.GetStorageHostSetting(peerID, &config)
	return config, err
}

// waitOnline will pause the current process and wait until the
// local node is connected with some peers (meaning the local node
// is online)
func (shm *StorageHostManager) waitOnline() {
	for {
		shm.lock.RLock()
		online := shm.netInfo.PeerCount() > 0
		shm.lock.RUnlock()

		if online {
			break
		}

		select {
		case <-time.After(scanOnlineCheckDuration):
		case <-shm.tm.StopChan():
		}
	}
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
