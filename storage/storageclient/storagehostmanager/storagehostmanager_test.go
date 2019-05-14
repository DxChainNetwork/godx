// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"testing"

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

	_, exist := shmtest1.storageHostTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if !exist {
		t.Fatalf("failed to insert the host information into the storage host tree")
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if exist {
		t.Fatalf("the host information should not be inserted, it is not contained in the filtered host field")
	}

	shmtest1.filteredHosts[hostInfo.EnodeID.String()] = hostInfo.EnodeID
	err = shmtest1.insert(hostInfo)
	if err != nil && err != storagehosttree.ErrHostExists {
		t.Fatalf("insert failed: %s", err.Error())
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if !exist {
		t.Fatalf("the host information should be inserted, it is contained in the filtered host field")
	}

}

func TestStorageHostManager_Remove(t *testing.T) {
	shmtest1.filterMode = WhitelistFilter
	shmtest1.filteredHosts = make(map[string]enode.ID)

	err := shmtest1.remove(hostInfo.EnodeID.String())
	if err != nil {
		t.Fatalf("remove failed: %s", err.Error())
	}

	_, exist := shmtest1.storageHostTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if exist {
		t.Fatalf("failed to remove the host information into the storage host tree")
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if !exist {
		t.Fatalf("the host information should not be removed, it is not contained in the filtered host field")
	}

	shmtest1.filteredHosts[hostInfo.EnodeID.String()] = hostInfo.EnodeID
	err = shmtest1.remove(hostInfo.EnodeID.String())
	if err != nil && err != storagehosttree.ErrHostNotExists {
		t.Fatalf("insert failed: %s", err.Error())
	}

	_, exist = shmtest1.filteredTree.RetrieveHostInfo(hostInfo.EnodeID.String())
	if exist {
		t.Fatalf("the host information should be removed, it is contained in the filtered host field")
	}
}
