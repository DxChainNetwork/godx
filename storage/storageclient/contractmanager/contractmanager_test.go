// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/DxChainNetwork/godx/accounts"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/core"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/event"
	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/params"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

func TestContractManager_Start(t *testing.T) {
	shm, err := newStorageHostManagerTest()
	if err != nil {
		t.Fatalf("error creating storage host manager: %s", err.Error())
	}

	cm, err := newContractManagerTest(shm)
	defer cm.activeContracts.Close()

	if err := cm.Start(&storageClientBackendContractManager{}); err != nil {
		t.Fatalf("failed to start the contract manager: %s", err.Error())
	}
}

/*
 _____  _____  _______      __  _______ ______          ______ _    _ _   _  _____ _______ _____ ____  _   _
|  __ \|  __ \|_   _\ \    / /\|__   __|  ____|        |  ____| |  | | \ | |/ ____|__   __|_   _/ __ \| \ | |
| |__) | |__) | | |  \ \  / /  \  | |  | |__           | |__  | |  | |  \| | |       | |    | || |  | |  \| |
|  ___/|  _  /  | |   \ \/ / /\ \ | |  |  __|          |  __| | |  | | . ` | |       | |    | || |  | | . ` |
| |    | | \ \ _| |_   \  / ____ \| |  | |____         | |    | |__| | |\  | |____   | |   _| || |__| | |\  |
|_|    |_|  \_\_____|   \/_/    \_\_|  |______|        |_|     \____/|_| \_|\_____|  |_|  |_____\____/|_| \_|

*/

func createNewContractManager() (cm *ContractManager, err error) {
	shm, err := newStorageHostManagerTest()
	if err != nil {
		err = fmt.Errorf("failed to create storage host manager while create contract manager: %s",
			err.Error())
		return
	}

	cm, err = newContractManagerTest(shm)

	return
}

var testRentPayment = storage.RentPayment{
	Fund:         common.NewBigIntUint64(18446744073709551615).MultUint64(18446744073709551615).MultUint64(18446744073709551615).MultUint64(18446744073709551615),
	StorageHosts: 50,
	Period:       3 * unit.BlocksPerMonth,

	ExpectedStorage:    1e12,                                // 1 TB
	ExpectedUpload:     uint64(200e9) / unit.BlocksPerMonth, // 200 GB per month
	ExpectedDownload:   uint64(100e9) / unit.BlocksPerMonth, // 100 GB per month
	ExpectedRedundancy: 3.0,
}

func newContractManagerTest(hm *storagehostmanager.StorageHostManager) (cm *ContractManager, err error) {
	// create and initialize host manager
	cm = &ContractManager{
		b:                &storageClientBackendContractManager{},
		persistDir:       "test",
		hostManager:      hm,
		maintenanceStop:  make(chan struct{}),
		expiredContracts: make(map[storage.ContractID]storage.ContractMetaData),
		renewedFrom:      make(map[storage.ContractID]storage.ContractID),
		renewedTo:        make(map[storage.ContractID]storage.ContractID),
		failedRenewCount: make(map[storage.ContractID]uint64),
		hostToContract:   make(map[enode.ID]storage.ContractID),
		quit:             make(chan struct{}),
		log:              log.New(),
	}
	cs, err := contractset.New("test")
	if err != nil {
		err = fmt.Errorf("failed to create contract set: %s", err.Error())
		return
	}

	cm.activeContracts = cs

	cm.rentPayment = testRentPayment

	if err = cm.hostManager.SetRentPayment(testRentPayment); err != nil {
		return
	}

	return
}

func newStorageHostManagerTest() (shm *storagehostmanager.StorageHostManager, err error) {
	shm = storagehostmanager.New("test")
	if err = shm.Start(&storageClientBackendContractManager{}); err != nil {
		return
	}
	return
}

type storageClientBackendContractManager struct{}

func (st *storageClientBackendContractManager) Online() bool {
	return true
}

func (st *storageClientBackendContractManager) CheckAndUpdateConnection(peerNode *enode.Node) {
}

func (st *storageClientBackendContractManager) SelfEnodeURL() string {
	return ""
}

func (st *storageClientBackendContractManager) Syncing() bool {
	return false
}

func (st *storageClientBackendContractManager) GetStorageHostSetting(hostEnodeID enode.ID, peerID string, config *storage.HostExtConfig) error {
	config = &storage.HostExtConfig{
		AcceptingContracts: true,
		Deposit:            common.NewBigInt(10),
		MaxDeposit:         common.NewBigInt(100),
	}
	return nil
}

func (st *storageClientBackendContractManager) SubscribeChainChangeEvent(ch chan<- core.ChainChangeEvent) event.Subscription {
	return nil
}

func (st *storageClientBackendContractManager) GetTxByBlockHash(blockHash common.Hash) (types.Transactions, error) {
	return nil, nil
}

func (st *storageClientBackendContractManager) ChainConfig() *params.ChainConfig {
	return nil
}

func (st *storageClientBackendContractManager) CurrentBlock() *types.Block {
	return nil
}

func (st *storageClientBackendContractManager) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return nil
}

func (st *storageClientBackendContractManager) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return nil, nil
}

func (st *storageClientBackendContractManager) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return 0, nil
}

func (st *storageClientBackendContractManager) AccountManager() *accounts.Manager {
	return nil
}

func (st *storageClientBackendContractManager) SetupConnection(enodeURL string) (storagePeer storage.Peer, err error) {
	return nil, nil
}

func (st *storageClientBackendContractManager) SendStorageContractCreateTx(clientAddr common.Address, input []byte) (common.Hash, error) {
	return common.Hash{}, nil
}

func (st *storageClientBackendContractManager) GetHostAnnouncementWithBlockHash(blockHash common.Hash) (hostAnnouncements []types.HostAnnouncement, number uint64, errGet error) {
	return
}

func (st *storageClientBackendContractManager) GetPaymentAddress() (address common.Address, err error) {
	return
}

func (st *storageClientBackendContractManager) TryToRenewOrRevise(hostID enode.ID) bool {
	return false
}

func (st *storageClientBackendContractManager) RevisionOrRenewingDone(hostID enode.ID) {}
