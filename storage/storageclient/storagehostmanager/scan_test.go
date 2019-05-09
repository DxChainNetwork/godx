package storagehostmanager

import (
	"crypto/rand"
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
	"github.com/Pallinder/go-randomdata"
	"testing"
	"time"
)

func TestStorageHostManager_Scan(t *testing.T) {
	shm := newHostManagerTestData()

	// exhaustive test
	//err := testDataInsert(100000000, shm)

	// regular test
	err := testDataInsert(10000, shm)

	if err != nil {
		t.Fatalf("error creating the object %s", err.Error())
	}
	go shm.scan()

	for {
		if shm.initialScan {
			return
		}
		time.Sleep(time.Second)
	}
}

func TestStorageHostManager_WaitScanFinish(t *testing.T) {
	shm := newHostManagerTestData()
	shm.scanWaitList = append(shm.scanWaitList, hostInfoGenerator())
	shm.scanWaitList = append(shm.scanWaitList, hostInfoGenerator())
	go func() {
		for {
			if len(shm.scanWaitList) == 0 {
				break
			}
			shm.lock.Lock()
			shm.scanWaitList = shm.scanWaitList[1:]
			shm.lock.Unlock()
			time.Sleep(1 * time.Microsecond)
		}
	}()
	go func() {
		select {
		case <-time.After(4 * time.Second):
			if len(shm.scanWaitList) == 0 {
				t.Fatalf("error: failed to unlock the process")
			}
		}
	}()
	shm.waitScanFinish()
}

func TestStorageHostManager_ScanValidation(t *testing.T) {
	shm := newHostManagerTestData()
	info1 := hostInfoGenerator()
	info2 := info1
	shm.scanLookup[info1.EnodeID.String()] = struct{}{}
	shm.scanValidation(info2)
	go func() {
		time.Sleep(1 * time.Second)
		t.Fatalf("the read lock failed to release, and the function scan validation should return immediately")
	}()
	shm.lock.RLock()
	shm.lock.RUnlock()
}

/*
 _____  _____  _______      __  _______ ______          ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|        |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__           | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|          |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____         | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|        |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

type storageClientBackendTestData struct{}

func newHostManagerTestData() *StorageHostManager {
	shm := &StorageHostManager{
		b: &storageClientBackendTestData{},

		rent: storage.DefaultRentPayment,

		scanLookup:    make(map[string]struct{}),
		filteredHosts: make(map[string]enode.ID),
	}

	shm.evalFunc = shm.calculateEvaluationFunc(shm.rent)
	shm.storageHostTree = storagehosttree.New(shm.evalFunc)
	shm.filteredTree = shm.storageHostTree
	shm.log = log.New()

	return shm
}

func testDataInsert(num int, shm *StorageHostManager) error {
	for i := 0; i < num; i++ {
		if err := shm.insert(hostInfoGenerator()); err != nil {
			return err
		}
	}
	return nil
}

func hostInfoGenerator() storage.HostInfo {
	ip := randomdata.IpV4Address()
	id := enodeIDGenerator()
	return storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			AcceptingContracts: true,
		},
		IP:       ip,
		EnodeID:  id,
		EnodeURL: fmt.Sprintf("enode://%s:%s:3030", id.String(), ip),
	}
}

func (st *storageClientBackendTestData) Online() bool {
	return true
}

func (st *storageClientBackendTestData) Syncing() bool {
	return false
}

func (st *storageClientBackendTestData) GetStorageHostSetting(peerID string, config *storage.HostExtConfig) error {
	config = &storage.HostExtConfig{
		AcceptingContracts: true,
		Deposit:            common.NewBigInt(10),
		MaxDeposit:         common.NewBigInt(100),
	}
	return nil
}

func (st *storageClientBackendTestData) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (st *storageClientBackendTestData) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return nil, nil
}

func enodeIDGenerator() enode.ID {
	id := make([]byte, 32)
	rand.Read(id)
	var result [32]byte
	copy(result[:], id[:32])
	return enode.ID(result)
}
