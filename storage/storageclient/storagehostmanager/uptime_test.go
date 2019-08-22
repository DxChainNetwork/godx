// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

// TestUptimeInitiate test the initiate for uptimeInitiate
func TestUptimeInitiate(t *testing.T) {
	tests := []struct {
		uptime           float64
		downtime         float64
		expectedUptime   float64
		expectedDowntime float64
		timeUpdated      bool
	}{
		{0, 0, initialAccumulatedUptime, initialAccumulatedDowntime, true},
		{10, 10, 10, 10, false},
	}
	for _, test := range tests {
		info := storage.HostInfo{
			AccumulatedUptime:   test.uptime,
			AccumulatedDowntime: test.downtime,
		}
		uptimeInitiate(&info)
		if info.AccumulatedUptime != test.expectedUptime {
			t.Errorf("uptime not expected")
		}
		if info.AccumulatedDowntime != test.expectedDowntime {
			t.Errorf("downtime not expected")
		}
		if test.timeUpdated && info.LastCheckTime == 0 {
			t.Errorf("time not updated")
		}
	}
}

// TestUpdateScanRecord test the functionality of updateScanRecord
func TestUpdateScanRecord(t *testing.T) {
	tests := []struct {
		numRecords      int
		expectedRecords int
	}{
		{0, 1}, {uptimeMaxNumScanRecords, uptimeMaxNumScanRecords},
	}
	for _, test := range tests {
		info := storage.HostInfo{}
		for i := 0; i != test.numRecords; i++ {
			info.ScanRecords = append(info.ScanRecords, storage.HostPoolScan{
				Timestamp: time.Now(),
				Success:   true,
			})
		}
		updateScanRecord(&info, true, uint64(time.Now().Unix()))
		if len(info.ScanRecords) != test.expectedRecords {
			t.Errorf("scan record number not expected. Got %v, Expect %v", len(info.ScanRecords), test.expectedRecords)
		}
	}
}

// TestCalcUptimeUpdate test calculate uptime update
func TestCalcUptimeUpdate(t *testing.T) {
	tests := []struct {
		success         bool
		upRateIncreased bool
	}{
		{true, true},
		{false, false},
	}

	for _, test := range tests {
		info := storage.HostInfo{
			AccumulatedUptime:   1000,
			AccumulatedDowntime: 1000,
			LastCheckTime:       uint64(time.Now().Unix() - 1000),
		}
		prevRate := getHostUpRate(info)

		newInfo := calcUptimeUpdate(info, test.success, uint64(time.Now().Unix()))
		newRate := getHostUpRate(newInfo)

		if test.upRateIncreased && prevRate >= newRate {
			t.Error("uprate not incrementing")
		}
		if !test.upRateIncreased && prevRate <= newRate {
			t.Error("uprate not decreasing")
		}
	}
}
