// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"fmt"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/Pallinder/go-randomdata"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
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
