package storagehostmanager

import (
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/storage"
	"math"
	"time"
)

func (shm *StorageHostManager) hostInfoUpdate(hi storage.HostInfo, err error) {
	// check if the error is caused by the storage client is not online
	if err != nil && shm.netInfo.PeerCount() == 0 {
		return
	}

	// get the storage host from the tree and update the storage host information
	storedInfo, exists := shm.storageHostTree.RetrieveHostInfo(hi.EnodeID.String())
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
		err := shm.remove(storedInfo.EnodeID.String())
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
