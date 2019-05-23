package storagehostmanager

import (
	"fmt"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"math"
	"time"
)

// hostInfoUpdate will update the storage host information based on the external settings
func (shm *StorageHostManager) hostInfoUpdate(hi storage.HostInfo, err error) {

	if err != nil && !shm.b.Online() {
		return
	}

	// get the storage host from the tree and update the storage host information
	storedInfo, exists := shm.storageHostTree.RetrieveHostInfo(hi.EnodeID)
	if exists {
		storedInfo.HostExtConfig = hi.HostExtConfig
		storedInfo.IPNetwork = hi.IPNetwork
		storedInfo.LastIPNetWorkChange = hi.LastIPNetWorkChange
	} else {
		storedInfo = hi
	}

	// update the recent interaction status
	if err != nil {
		storedInfo.RecentFailedInteractions++
	} else {
		storedInfo.RecentSuccessfulInteractions++
	}

	// update scan record, make sure the scan record has at least two scans
	if len(storedInfo.ScanRecords) < 2 {
		earliestScanTime := time.Now().Add(time.Hour * 7 * 24 * -1)
		suggestedScanTime := time.Now().Add(time.Minute * 10 * time.Duration(shm.blockHeight-hi.FirstSeen+1) * -1)
		if suggestedScanTime.Before(earliestScanTime) {
			suggestedScanTime = earliestScanTime
		}

		storedInfo.ScanRecords = storage.HostPoolScans{
			{Timestamp: suggestedScanTime, Success: err == nil},
			{Timestamp: time.Now(), Success: err == nil},
		}
	} else {
		currentTimeStamp := time.Now()
		prevScanTime := storedInfo.ScanRecords[len(storedInfo.ScanRecords)-1].Timestamp
		if prevScanTime.After(currentTimeStamp) {
			currentTimeStamp = prevScanTime.Add(time.Second)
		}
		newestScan := storage.HostPoolScan{Timestamp: currentTimeStamp, Success: err == nil}
		storedInfo.ScanRecords = append(storedInfo.ScanRecords, newestScan)
	}

	// if the host is not upp and the exceed the max host downtime, then remove it and return
	recentUp := err == nil
	if !recentUp && len(storedInfo.ScanRecords) > minScans &&
		time.Now().Sub(storedInfo.ScanRecords[0].Timestamp) > maxDowntime {
		err := shm.remove(storedInfo.EnodeID)
		if err != nil {
			log.Error("failed to remove the storage host from the tree: %s", storedInfo.EnodeID.String())
		}
		return
	}

	// update the scan records, for record that stayed for long time, add it to
	// update the historic uptime and historic downtime, and remove them from the
	// scan records
	for len(storedInfo.ScanRecords) > minScans &&
		time.Now().Sub(storedInfo.ScanRecords[0].Timestamp) > maxDowntime {
		timePassed := storedInfo.ScanRecords[1].Timestamp.Sub(storedInfo.ScanRecords[0].Timestamp)
		if storedInfo.ScanRecords[0].Success {
			storedInfo.HistoricUptime += timePassed
		} else {
			storedInfo.HistoricUptime += timePassed
		}

		storedInfo.ScanRecords = storedInfo.ScanRecords[1:]
	}

	// update the storage host tree
	if !exists {
		err := shm.insert(storedInfo)
		if err != nil {
			log.Error("failed to insert the storage host information: ", err.Error())
		}
	} else {
		err := shm.modify(storedInfo)
		if err != nil {
			log.Error("failed to modify the storage host information: ", err.Error())
		}
	}

}

// hostHistoricInteractionsUpdate will update storage host historical interactions
func hostHistoricInteractionsUpdate(hi *storage.HostInfo, blockHeight uint64) {
	// avoid update that happened in the future
	// and avoid update that complete already
	if hi.LastHistoricUpdate >= blockHeight {
		return
	}

	// at least one block is passed, apply decay for the block
	hsi := hi.HistoricSuccessfulInteractions
	hfi := hi.HistoricFailedInteractions
	hsi *= historicInteractionDecay
	hfi *= historicInteractionDecay

	// to avoid downgrade the influence of recent interactions, adjustments need to be made
	rsi := float64(hi.RecentSuccessfulInteractions)
	rfi := float64(hi.RecentFailedInteractions)
	if hsi+hfi > historicInteractionDecayLimit {
		if rsi+rfi > recentInteractionWeightLimit {
			adjustment := recentInteractionWeightLimit * (hsi + hfi) / (rsi + rfi)
			rsi *= adjustment
			rfi *= adjustment
		}
	} else {
		if rsi+rfi > recentInteractionWeightLimit*historicInteractionDecayLimit {
			adjustment := recentInteractionWeightLimit * historicInteractionDecayLimit / (rsi + rfi)
			rsi *= adjustment
			rfi *= adjustment
		}
	}

	hsi += rsi
	rfi += rfi

	// check how many blocks it passed by since the last update
	blocks := blockHeight - hi.LastHistoricUpdate
	if blocks > 1 && hsi+hfi > historicInteractionDecayLimit {
		decay := math.Pow(historicInteractionDecay, float64(blocks-1))
		hsi *= decay
		hfi *= decay
	}

	// update the storage host interaction information
	hi.HistoricSuccessfulInteractions = hsi
	hi.HistoricFailedInteractions = hfi
	hi.RecentSuccessfulInteractions = 0
	hi.RecentFailedInteractions = 0

	hi.LastHistoricUpdate = blockHeight
}

// IncrementSuccessfulInteractions will update both storage host's historical interactions
// and recent successful interactions
func (shm *StorageHostManager) IncrementSuccessfulInteractions(id enode.ID) {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	// get the storage host
	host, exists := shm.storageHostTree.RetrieveHostInfo(id)
	if !exists {
		shm.log.Warn("failed to get the storage host information while trying to increase the successful interactions")
		return
	}
	// update the historical interactions
	hostHistoricInteractionsUpdate(&host, shm.blockHeight)

	// update the recent successful interactions, recalculate the storage host evaluation
	host.RecentSuccessfulInteractions++

	if err := shm.storageHostTree.HostInfoUpdate(host); err != nil {
		shm.log.Error(fmt.Sprintf("failed to update successful interactions %s", err.Error()))
	}
}

// IncrementFailedInteractions will update both storage host's historical interactions
// and recent failed interactions
func (shm *StorageHostManager) IncrementFailedInteractions(id enode.ID) {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	// get the storage host information
	host, exists := shm.storageHostTree.RetrieveHostInfo(id)
	if !exists || !shm.b.Online() {
		return
	}

	// update the historical interactions
	hostHistoricInteractionsUpdate(&host, shm.blockHeight)

	// update the recent failed interactions, recalculate the storage host evaluation
	host.RecentFailedInteractions++
	if err := shm.storageHostTree.HostInfoUpdate(host); err != nil {
		shm.log.Error(fmt.Sprintf("failed to update failed interactions %s", err.Error()))
	}
}
