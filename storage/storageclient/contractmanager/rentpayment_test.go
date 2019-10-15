// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
	"github.com/DxChainNetwork/godx/storage/storageclient/contractset"
)

// In testing, erasure code related settings are redefined here
func init() {
	defaultMinSectors = 1
	defaultNumSectors = 2
}

var rentPaymentTest = storage.RentPayment{
	Fund:         common.NewBigInt(1000000),
	StorageHosts: 3,
	Period:       unit.BlocksPerDay,
}

type fakeHostMarket struct {
	prices storage.MarketPrice
}

// getMarketPrice of fakeHostMarket directly return the prices
func (fhm *fakeHostMarket) GetMarketPrice() storage.MarketPrice {
	return fhm.prices
}

func TestContractManager_SetRentPayment(t *testing.T) {
	// create new contract manager
	cm, err := NewFakeContractManager(testContractManagerBackend)
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
		expiredContract := simulation.ContractGenerator(cm.blockHeight / 2)
		_, err := cm.activeContracts.InsertContract(expiredContract, simulation.RootsGenerator(10))
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
	market := &fakeHostMarket{
		storage.MarketPrice{
			StoragePrice:  common.NewBigInt(1),
			UploadPrice:   common.NewBigInt(1),
			DownloadPrice: common.NewBigInt(1),
		},
	}
	if err := cm.SetRentPayment(rentPaymentTest, market); err != nil {
		t.Fatalf("failed to set the rent payment: %s", err.Error())
	}

	// validation, check the rentPayment first for both storage host and contract manager
	cm.lock.RLock()
	rentPayment := cm.rentPayment
	cm.lock.RUnlock()

	if err := checkRentPaymentEqual(rentPayment, rentPaymentTest); err != nil {
		t.Fatal(err)
	}
	hmRent := cm.hostManager.RetrieveRentPayment()
	if err := checkRentPaymentEqual(hmRent, rentPaymentTest); err != nil {
		t.Fatal(err)
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

func TestEstimateRentPaymentSizes(t *testing.T) {
	prices := storage.MarketPrice{
		StoragePrice:  common.NewBigInt(1),
		UploadPrice:   common.NewBigInt(1),
		DownloadPrice: common.NewBigInt(1),
	}
	rent := storage.RentPayment{
		Fund:         common.NewBigInt(100000000000000),
		StorageHosts: 5,
	}
	res := estimateRentPaymentSizes(rent, prices)
	// Check the results
	expectedRedundancy := float64(defaultNumSectors) / float64(defaultMinSectors)
	if res.ExpectedRedundancy != expectedRedundancy {
		t.Errorf("unexpected redundancy. Expect %v, Got %v", expectedRedundancy, res.ExpectedRedundancy)
	}
	if res.ExpectedDownload == 0 || res.ExpectedUpload == 0 || res.ExpectedStorage == 0 {
		t.Errorf("zero results")
	}
	if float64(res.ExpectedDownload)/float64(res.ExpectedUpload) != downloadSizeRatio/uploadSizeRatio {
		t.Errorf("upload download ratio not expected. %v : %v != %v : %v", res.ExpectedDownload,
			res.ExpectedUpload, downloadSizeRatio, uploadSizeRatio)
	}
	if float64(res.ExpectedStorage)/float64(res.ExpectedDownload) != storageSizeRatio*expectedRedundancy/downloadSizeRatio {
		t.Errorf("storage to download ratio not expected. %v : %v != %v : %v", float64(res.ExpectedStorage)*
			expectedRedundancy, res.ExpectedDownload, storageSizeRatio*expectedRedundancy, downloadSizeRatio)
	}
}

// checkRentPaymentEqual checks whether the two input rent payments are the same.
// The checked fields does not include the size fields
func checkRentPaymentEqual(rent1, rent2 storage.RentPayment) error {
	if rent1.Fund.Cmp(rent2.Fund) != 0 {
		return fmt.Errorf("fund not equal. %v != %v", rent1.Fund, rent2.Fund)
	}
	if rent1.Period != rent2.Period {
		return fmt.Errorf("period not equal. %v != %v", rent1.Period, rent2.Period)
	}
	if rent1.StorageHosts != rent2.StorageHosts {
		return fmt.Errorf("storage host number not equal. %v != %v", rent1.StorageHosts, rent2.StorageHosts)
	}
	return nil
}
