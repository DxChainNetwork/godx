// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"errors"
	"github.com/DxChainNetwork/godx/storage"
	"reflect"
)

func (cm *ContractManager) SetRentPayment(rent storage.RentPayment) (err error) {
	if cm.b.Syncing() {
		return errors.New("must finish block chain syncing first")
	}

	if err = rentPaymentValidation(rent); err != nil {
		return
	}

	// getting the old payment
	cm.lock.Lock()
	oldRent := cm.rentPayment
	cm.rentPayment = rent
	cm.lock.Unlock()

	// check if the old payment is same as new payment
	if reflect.DeepEqual(oldRent, rent) {
		return errors.New("the payment entered is same as before")
	}

	// indicates the contracts have been canceled previously
	if reflect.DeepEqual(oldRent, storage.RentPayment{}) {
		// update the current period
		cm.lock.Lock()
		cm.currentPeriod = cm.blockHeight - rent.RenewWindow
		cm.lock.Unlock()

		// reuse the active canceled contracts
		if err = cm.resumeContracts(); err != nil {
			return
		}
	}

	// save all the settings
	if err = cm.saveSettings(); err != nil {
		cm.log.Crit("unable to save settings")
	}

	// update storage host manager rentPayment payment
	// which updates the storage host evaluation function
	if err = cm.hostManager.SetRentPayment(rent); err != nil {
		return
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

func rentPaymentValidation(rent storage.RentPayment) (err error) {
	if rent.StorageHosts == 0 {
		return errors.New("amount of storage hosts cannot be set to 0")
	} else if rent.Period == 0 {
		return errors.New("storage period cannot be set to 0")
	} else if rent.RenewWindow == 0 {
		return errors.New("renew window cannot be set to 0")
	} else if rent.ExpectedStorage == 0 {
		return errors.New("expected storage cannot be set to 0")
	} else if rent.ExpectedUpload == 0 {
		return errors.New("expectedUpload cannot be set to 0")
	} else if rent.ExpectedDownload == 0 {
		return errors.New("expectedDownload cannot be set to 0")
	} else if rent.ExpectedRedundancy == 0 {
		return errors.New("expectedRedundancy cannot be set to 0")
	} else if rent.RenewWindow > rent.Period {
		return errors.New("renew window cannot be larger than the period")
	}

	return
}
