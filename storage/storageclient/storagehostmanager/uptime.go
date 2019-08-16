// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math"
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

// uptimeInitiate initiate the uptime related fields for host info
func uptimeInitiate(info *storage.HostInfo) {
	if info.AccumulatedUptime == 0 && info.AccumulatedDowntime == 0 {
		return
	}
	info.AccumulatedUptime = initialAccumulatedUptime
	info.AccumulatedDowntime = initialAccumulatedDowntime
	info.LastCheckTime = uint64(time.Now().Unix())
}

// getHostUpRate get the uptime rate of a host
func getHostUpRate(info storage.HostInfo) float64 {
	return info.AccumulatedUptime / (info.AccumulatedUptime + info.AccumulatedDowntime)
}

// calcUptimeUpdate calculate the Uptime update for the host info
func calcUptimeUpdate(info storage.HostInfo, success bool, now uint64) storage.HostInfo {
	// Calculate the decay form time
	timePassed := now - info.LastInteractionTime
	decay := math.Pow(uptimeDecay, float64(timePassed))

	// Apply the decay
	info.AccumulatedUptime *= decay
	info.AccumulatedDowntime *= decay
	info.LastCheckTime = now

	// Calculate the accumulated time with decay factor
	// The amount is defined by integral of decayFactor^x * dx
	timeIncrease := (1 - math.Pow(uptimeDecay, timePassed)) / math.Log(uptimeDecay)
	if success {
		info.AccumulatedUptime += timeIncrease
	} else {
		info.AccumulatedDowntime += timeIncrease
	}
	updateScanRecord(&info, success, now)
	return info
}

// updateScanRecord add a scan record to host info
// If the scan record is larger than 5, cap the list to size 5
func updateScanRecord(info *storage.HostInfo, success bool, now uint64) {
	info.ScanRecords = append(info.ScanRecords, storage.HostPoolScan{
		Timestamp: time.Unix(int64(now), 0),
		Success:   success,
	})
	if len(info.ScanRecords) > uptimeMaxNumScanRecords {
		info.ScanRecords = info.ScanRecords[len(info.ScanRecords)-uptimeMaxNumScanRecords:]
	}
}

// applyNewHostInfoToStoredHostInfo apply the new host config to stored host info.
// Only necessary fields are updated
func applyNewHostInfoToStoredHostInfo(new, stored storage.HostInfo) storage.HostInfo {
	stored.HostExtConfig = new.HostExtConfig
	stored.IPNetwork = new.IPNetwork
	stored.LastIPNetWorkChange = new.LastIPNetWorkChange
}
