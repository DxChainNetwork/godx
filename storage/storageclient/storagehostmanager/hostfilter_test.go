package storagehostmanager

import (
	"testing"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)


func TestStorageHostManager_SetFilterMode(t *testing.T) {
	shm := New("test")
	var allHosts []storage.HostInfo

	for i := 0; i < 10; i ++ {
		host := hostInfoGenerator()
		allHosts = append(allHosts, host)
		err := shm.insert(host)
		if err != nil {
			t.Fatalf("failed to insert the host information into the tree: %s", err.Error())
		}
	}

	var whitelist []enode.ID
	for i := 0; i < 10; i ++ {
		host := hostInfoGenerator()
		allHosts = append(allHosts, host)
		err := shm.insert(host)
		if err != nil {
			t.Fatalf("failed to insert the host information into the tree : %s", err.Error())
		}
		whitelist = append(whitelist, host.EnodeID)
	}

	err := shm.SetFilterMode(100, whitelist)
	if err == nil {
		t.Fatalf("error should be returned by providing a non-existed filter mode code")
	}

	if shm.filterMode != DisableFilter {
		t.Fatalf("error, the filter mode should be disabled, instead of %s", shm.filterMode.String())
	}

	err = shm.SetFilterMode(DisableFilter, whitelist)
	if err != nil {
		t.Fatalf("error setting filter mode to be disable: %s", err.Error())
	}

	if shm.filterMode != DisableFilter {
		t.Fatalf("error, the filter mode should be disabled, instead of %s", shm.filterMode.String())
	}

	err = shm.SetFilterMode(WhitelistFilter, whitelist)
	if err != nil {
		t.Fatalf("error setting filter mode to be whitelist: %s", err.Error())
	}

	for _, host := range allHosts {
		_, exist := shm.storageHostTree.RetrieveHostInfo(host.EnodeID.String())
		if !exist {
			t.Fatalf("the host information is not contained in the storage host tree")
		}

		_, existFilter := shm.filteredTree.RetrieveHostInfo(host.EnodeID.String())
		if existFilter && !exist {
			t.Fatalf("the host information is contained in the filtered tree but not in the storage host tree")
		}

		_, filterHostExist := shm.filteredHosts[host.EnodeID.String()]
		if existFilter && !filterHostExist {
			t.Fatalf("the host information is contained in the filtered tree but not in the white list")
		}
	}

	if shm.filterMode != WhitelistFilter {
		t.Fatalf("error, the filter mode should be whitelist, instead of %s", shm.filterMode.String())
	}

}
