// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/DxChainNetwork/godx/common/unit"

	"github.com/DxChainNetwork/godx/storage"
)

var (
	// erasure code related parameters. Default to params set in storage module.
	// The values are redeclared here only for test cases
	defaultMinSectors = storage.DefaultMinSectors
	defaultNumSectors = storage.DefaultNumSectors
)

// hostMarket is the interface implemented by storageHostManager
type hostMarket interface {
	GetMarketPrice() storage.MarketPrice
}

// SetRentPayment will set the rent payment to the value passed in by the user
// through the command line interface
func (cm *ContractManager) SetRentPayment(rent storage.RentPayment, market hostMarket) (err error) {
	if cm.b.Syncing() {
		return errors.New("setRentPayment can only be done once the block chain finished syncing")
	}

	// Calculate the expected sizes in rent payment.
	prices := market.GetMarketPrice()
	rent = estimateRentPaymentSizes(rent, prices)

	// validate the rentPayment, making sure that fields are not empty
	if err = RentPaymentValidation(rent); err != nil {
		return
	}

	// getting the old payment
	cm.lock.Lock()
	oldCurrentPeriod := cm.currentPeriod
	oldRent := cm.rentPayment
	cm.rentPayment = rent
	cm.lock.Unlock()

	// if error is not nil, revert the settings back to the
	// original settings
	defer func() {
		if err != nil {
			cm.lock.Lock()
			cm.rentPayment = oldRent
			cm.currentPeriod = oldCurrentPeriod
			cm.lock.Unlock()
		}
	}()

	// indicates the contracts have been canceled previously
	// or it is client's first time signing the storage contract
	if reflect.DeepEqual(oldRent, storage.RentPayment{}) {
		// update the current period
		cm.lock.Lock()
		cm.currentPeriod = cm.blockHeight
		cm.lock.Unlock()

		// reuse the active canceled contracts
		if err = cm.resumeContracts(); err != nil {
			cm.log.Error("SetRentPayment failed, error resuming the active storage contracts", "err", err.Error())
			return
		}
	}

	// update storage host manager rentPayment payment
	// which updates the storage host evaluation function
	if err = cm.hostManager.SetRentPayment(rent); err != nil {
		cm.log.Error("SetRentPayment failed, failed to set the rent payment for host manager", "err", err.Error())
		return
	}

	// save all the settings
	if err = cm.saveSettings(); err != nil {
		cm.log.Error("SetRentPayment failed, unable to save settings while setting the rent payment", "err", err.Error())
		// set the storage host's rentPayment back to original value
		_ = cm.hostManager.SetRentPayment(oldRent)
		return fmt.Errorf("failed to save settings persistently: %s", err.Error())
	}

	// if the maintenance process is running, stop it
	cm.lock.Lock()
	if cm.maintenanceRunning {
		cm.maintenanceStop <- struct{}{}
	}
	cm.lock.Unlock()

	// wait util the current maintenance finished execution, and start new maintenance
	go func() {
		cm.maintenanceWg.Wait()
		cm.contractMaintenance()
	}()

	return
}

// AcquireRentPayment will return the RentPayment settings
func (cm *ContractManager) AcquireRentPayment() (rentPayment storage.RentPayment) {
	return cm.rentPayment
}

// RentPaymentValidation will validate the rentPayment. All fields must be
// non-zero value
func RentPaymentValidation(rent storage.RentPayment) (err error) {
	switch {
	case rent.StorageHosts == 0:
		return errors.New("amount of storage hosts cannot be set to 0")
	case rent.Period == 0:
		return errors.New("storage period cannot be set to 0")
	case rent.ExpectedStorage == 0:
		return errors.New("expected storage cannot be set to 0")
	case rent.ExpectedUpload == 0:
		return errors.New("expectedUpload cannot be set to 0")
	case rent.ExpectedDownload == 0:
		return errors.New("expectedDownload cannot be set to 0")
	case rent.ExpectedRedundancy == 0:
		return errors.New("expectedRedundancy cannot be set to 0")
	case storage.RenewWindow > rent.Period:
		return fmt.Errorf("storage period must be greater than %v", unit.FormatTime(storage.RenewWindow))
	default:
		return
	}
}

// estimateRentPaymentSizes estimate the sizes in rent payment based on fund settings and the
// input market price. Currently, the contract fund are split among the storage fund, upload
// fund and download fund. The sizes follows the ratio defined in defaults.go
func estimateRentPaymentSizes(rent storage.RentPayment, prices storage.MarketPrice) storage.RentPayment {
	// Estimate the redundancy
	redundancy := float64(defaultNumSectors) / float64(defaultMinSectors)

	// Estimate the sizes
	fundPerContract := rent.Fund.DivUint64(rent.StorageHosts)
	storageRatio := prices.StoragePrice.MultFloat64(redundancy).MultUint64(rent.Period).MultFloat64(storageSizeRatio)
	downloadRatio := prices.DownloadPrice.MultFloat64(uploadSizeRatio)
	uploadRatio := prices.UploadPrice.MultFloat64(downloadSizeRatio)
	ratioSum := storageRatio.Add(downloadRatio).Add(uploadRatio)

	// Calculate the sizes
	sizeBase := fundPerContract.Div(ratioSum).Float64()
	expectedStorage := uint64(sizeBase * storageSizeRatio * redundancy)
	expectedDownload := uint64(sizeBase * downloadSizeRatio)
	expectedUpload := uint64(sizeBase * uploadSizeRatio)

	// Apply the calculated values to rent Payment
	rent.ExpectedRedundancy = redundancy
	rent.ExpectedStorage = expectedStorage
	rent.ExpectedUpload = expectedUpload
	rent.ExpectedDownload = expectedDownload
	return rent
}
