// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storagehostmanager

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
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
		shm.lock.Lock()
		is := shm.initialScan
		shm.lock.Unlock()

		if is {
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
	shm.scanLookup[info1.EnodeID] = struct{}{}
	shm.startScanning(info2)
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
		b:             &storageClientBackendTestData{},
		rent:          storage.DefaultRentPayment,
		scanLookup:    make(map[enode.ID]struct{}),
		filteredHosts: make(map[enode.ID]struct{}),
	}

	shm.hostEvaluator = newDefaultEvaluator(shm, shm.rent)
	shm.storageHostTree = storagehosttree.New()
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

func (st *storageClientBackendTestData) Online() bool {
	return true
}

func (st *storageClientBackendTestData) Syncing() bool {
	return false
}

func (st *storageClientBackendTestData) GetStorageHostSetting(hostEnodeID enode.ID, peerID string, config *storage.HostExtConfig) error {
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

func (st *storageClientBackendTestData) ChainConfig() *params.ChainConfig {
	return nil
}

func (st *storageClientBackendTestData) CurrentBlock() *types.Block {
	return nil
}

func (st *storageClientBackendTestData) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return nil
}

func (st *storageClientBackendTestData) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return nil, nil
}

func (st *storageClientBackendTestData) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return 0, nil
}

func (st *storageClientBackendTestData) AccountManager() *accounts.Manager {
	return nil
}

func (st *storageClientBackendTestData) SetupConnection(enodeURL string) (storage.Peer, error) {
	return nil, nil
}

func (st *storageClientBackendTestData) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (st *storageClientBackendTestData) GetHostAnnouncementWithBlockHash(blockHash common.Hash) (hostAnnouncements []types.HostAnnouncement, number uint64, errGet error) {
	return
}

func (st *storageClientBackendTestData) TryToRenewOrRevise(hostID enode.ID) bool {
	return false
}

func (st *storageClientBackendTestData) GetPaymentAddress() (common.Address, error) {
	return common.Address{}, nil
}

func (st *storageClientBackendTestData) RevisionOrRenewingDone(hostID enode.ID) {}

func (st *storageClientBackendTestData) CheckAndUpdateConnection(peerNode *enode.Node) {}

func (st *storageClientBackendTestData) SelfEnodeURL() string {
	return ""
}
