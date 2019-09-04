// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"github.com/Pallinder/go-randomdata"
)

var shmtest1 = New("test")
var hostInfo = hostInfoGenerator()

func TestStorageHostManager_Insert(t *testing.T) {
	shmtest1.filterMode = WhitelistFilter

	err := shmtest1.insert(hostInfo)
	if err != nil {
		t.Fatalf("insert failed: %s", err.Error())
	}

	_, exist := shmtest1.storageHostTree.RetrieveHostInfo(hostInfo.EnodeID)
	if !exist {
		t.Fatalf("failed to insert the host information into the storage host tree")
	}

	shmtest1.filteredHosts[hostInfo.EnodeID] = struct{}{}
	err = shmtest1.insert(hostInfo)
	if err != nil && err != storagehosttree.ErrHostExists {
		t.Fatalf("insert failed: %s", err.Error())
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID)
	if !exist {
		t.Fatalf("the host information should be inserted, it is contained in the filtered host field")
	}

}

func TestStorageHostManager_Remove(t *testing.T) {
	shmtest1.filterMode = WhitelistFilter
	shmtest1.filteredHosts = make(map[enode.ID]struct{})

	err := shmtest1.remove(hostInfo.EnodeID)
	if err != nil {
		t.Fatalf("remove failed: %s", err.Error())
	}

	_, exist := shmtest1.storageHostTree.RetrieveHostInfo(hostInfo.EnodeID)
	if exist {
		t.Fatalf("failed to remove the host information into the storage host tree")
	}

	shmtest1.filteredHosts[hostInfo.EnodeID] = struct{}{}
	err = shmtest1.remove(hostInfo.EnodeID)
	if err != nil && err != storagehosttree.ErrHostNotExists {
		t.Fatalf("insert failed: %s", err.Error())
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID)
	if exist {
		t.Fatalf("the host information should be removed, it is contained in the filtered host field")
	}
}

func TestStorageHostManager_FilterIPViolationHosts(t *testing.T) {
	unsavedHost := hostInfoGeneratorForIPViolation(randomdata.IpV4Address(), time.Now())
	hostEarlierIP := hostInfoGeneratorForIPViolation("196.5.4.3", time.Now())
	hostLaterIP := hostInfoGeneratorForIPViolation("196.5.4.4", time.Now())

	hostIDs := []enode.ID{unsavedHost.EnodeID, hostEarlierIP.EnodeID, hostLaterIP.EnodeID}
	insertHostInfo := []storage.HostInfo{hostEarlierIP, hostLaterIP}

	for _, info := range insertHostInfo {
		if err := shmtest1.insert(info); err != nil {
			t.Fatalf("failed to insert the storage host information")
		}
	}

	badHosts := shmtest1.FilterIPViolationHosts(hostIDs)
	if len(badHosts) != 0 {
		t.Fatalf("the filter mode is not enabled, the number of bad hosts is expected to be 0, got %v",
			len(badHosts))
	}

	// enable ip violation check
	shmtest1.SetIPViolationCheck(true)
	badHosts = shmtest1.FilterIPViolationHosts(hostIDs)
	if len(badHosts) != 2 {
		t.Fatalf("the filter mode is enabled, the number of bad hosts is expected to be 2, got %v",
			len(badHosts))
	}

	for _, id := range badHosts {

		if id != unsavedHost.EnodeID && id != hostLaterIP.EnodeID {
			t.Fatalf("the bad host is expected to be one of the unsavedHost of hostLasterIP. Got ID: %v, expected to be one of these two ids: %v, %v",
				id, unsavedHost.EnodeID, hostLaterIP.EnodeID)
		}
	}

}

func hostInfoGeneratorForIPViolation(ip string, changeTime time.Time) storage.HostInfo {
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts:     true,
			Deposit:                common.NewBigInt(100),
			ContractPrice:          common.RandomBigInt(),
			DownloadBandwidthPrice: common.RandomBigInt(),
			StoragePrice:           common.RandomBigInt(),
			UploadBandwidthPrice:   common.RandomBigInt(),
			SectorAccessPrice:      common.RandomBigInt(),
			RemainingStorage:       100,
		},
		IP:                  ip,
		EnodeID:             id,
		EnodeURL:            fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
		LastIPNetWorkChange: changeTime,
	}
}

// TestStorageHostManager_isInitialScanFinished test the functionality of StorageHostManager.
// isInitialScanFinished
func TestStorageHostManager_isInitialScanFinished(t *testing.T) {
	tests := []struct {
		finished bool
	}{
		{true}, {false},
	}
	for _, test := range tests {
		shm := StorageHostManager{
			initialScanFinished: make(chan struct{}),
		}
		if test.finished {
			close(shm.initialScanFinished)
		}

		res := shm.isInitialScanFinished()
		if res != test.finished {
			t.Errorf("isInitialScanFinished return unexpected value. Expect %v, got %v", test.finished,
				res)
		}
	}
}

// TestStorageHostManager_finishInitialScan test the functionality of StorageHostManager.finishInitialScan
func TestStorageHostManager_finishInitialScan(t *testing.T) {
	tests := []struct {
		closed bool
	}{
		{true}, {false},
	}
	for _, test := range tests {
		shm := StorageHostManager{initialScanFinished: make(chan struct{})}
		if test.closed {
			close(shm.initialScanFinished)
		}
		shm.finishInitialScan()
		closed := shm.isInitialScanFinished()
		if !closed {
			t.Errorf("After finishInitialScan, the channel still not closed")
		}
	}
}

// TestStorageHostManager_waitUntilInitialScanFinished test the functionality of
// StorageHostManager.waitUntilInitialScanFinished. Two cases are tested:
//   1. no time out
//   2. timed out
func TestStorageHostManager_waitUntilInitialScanFinished(t *testing.T) {
	tests := []struct {
		scanDelay time.Duration
		timeout   time.Duration
		err       error
	}{
		{time.Millisecond, 10 * time.Millisecond, nil},
		{10 * time.Millisecond, time.Millisecond, errors.New("timeout")},
	}
	for _, test := range tests {
		shm := StorageHostManager{initialScanFinished: make(chan struct{})}
		// After timeout, close the channel
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			time.Sleep(test.scanDelay)
			shm.finishInitialScan()
			wg.Done()
		}()
		var err error
		go func() {
			time.Sleep(test.timeout)
			err = shm.waitUntilInitialScanFinished(test.timeout)
			wg.Done()
		}()
		wg.Wait()
		if (err == nil) != (test.err == nil) {
			t.Errorf("expect error %v\nGot error %v", test.err, err)
		}
	}
}
