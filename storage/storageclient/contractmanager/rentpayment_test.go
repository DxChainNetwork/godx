// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

var rentPaymentTest = storage.RentPayment{
	Fund:               common.NewBigInt(1000000),
	StorageHosts:       3,
	Period:             10,
	RenewWindow:        5,
	ExpectedStorage:    100,
	ExpectedUpload:     100,
	ExpectedDownload:   100,
	ExpectedRedundancy: 3,
}

func TestContractManager_SetRentPayment(t *testing.T) {
	// create new contract manager
	cm, err := NewFakeContractManager(positiveTestBackend)
	if err != nil {
		t.Fatalf("failed to create contract manager: %s", err.Error())
	}

	// set the block height for contract expiration
	cm.blockHeight = 100

	// choose how many contracts to insert into the activeContracts list
	amount := 1000
	if testing.Short() {
		amount = 1000
	}

	// insert some expired contracts into the contractManager active contracts field
	// create and insert expired contracts
	var expiredContracts []contractset.ContractHeader
	for i := 0; i < amount; i++ {
		expiredContract := randomContractGenerator(cm.blockHeight / 2)
		_, err := cm.activeContracts.InsertContract(expiredContract, randomRootsGenerator(10))
		if err != nil {
			t.Fatalf("failed to insert contract: %s", err.Error())
		}
		expiredContracts = append(expiredContracts, expiredContract)
	}

	// simulate the contractMaintenance running situation
	cm.lock.Lock()
	cm.maintenanceRunning = true
	cm.maintenanceWg.Add(1)
	cm.lock.Unlock()

	// once the stop signal was sent, mark the wait event as done
	go func() {
		select {
		case <-cm.maintenanceStop:
			cm.lock.Lock()
			cm.maintenanceRunning = false
			cm.lock.Unlock()
			cm.maintenanceWg.Done()
		}
	}()

	// set the rent payment
	if err := cm.SetRentPayment(rentPaymentTest); err != nil {
		t.Fatalf("failed to set the rent payment: %s", err.Error())
	}

	// validation, check the rentPayment first for both storage host and contract manager
	cm.lock.RLock()
	rentPayment := cm.rentPayment
	cm.lock.RUnlock()

	if rentPayment.ExpectedDownload != rentPaymentTest.ExpectedDownload {
		t.Fatalf("expected rentPayment to be set as %+v, instead got %+v",
			rentPaymentTest, rentPayment)
	}

	if cm.hostManager.RetrieveRentPayment().ExpectedUpload != rentPaymentTest.ExpectedUpload {
		t.Fatalf("expected rentPayment for storage host manager to be set as %+v, got %+v",
			cm.hostManager.RetrieveRentPayment(), rentPaymentTest)
	}

	time.Sleep(time.Second)

	cm.lock.RLock()
	expiredContractList := cm.expiredContracts
	cm.lock.RUnlock()

	for _, contract := range expiredContracts {
		if _, exists := expiredContractList[contract.ID]; !exists {
			t.Fatalf("maitenance failed, the contract should be in the expired contract list")
		}
	}

}
