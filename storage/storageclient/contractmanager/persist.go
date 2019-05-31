// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"os"
	"path/filepath"
)

var settingsMetadata = common.Metadata{
	Header:  PersistContractManagerHeader,
	Version: PersistContractManagerVersion,
}

type persistence struct {
	Rent                 storage.RentPayment                       `json:"rentPayment"`
	BlockHeight          uint64                                    `json:"blockheight"`
	CurrentPeriod        uint64                                    `json:"currentperiod"`
	ExpiredContracts     []storage.ContractMetaData                `json:"expiredcontracts"`
	RecoverableContracts []storage.RecoverableContract             `json:"recoverablecontracts"`
	RenewedFrom          map[storage.ContractID]storage.ContractID `json:"renewedfrom"`
	RenewedTo            map[storage.ContractID]storage.ContractID `json:"renewedto"`
}

func (cm *ContractManager) persistUpdate() (persist persistence) {
	persist = persistence{
		Rent:          cm.rentPayment,
		BlockHeight:   cm.blockHeight,
		CurrentPeriod: cm.currentPeriod,
		RenewedFrom:   cm.renewedFrom,
		RenewedTo:     cm.renewedTo,
	}

	for _, ec := range cm.expiredContracts {
		persist.ExpiredContracts = append(persist.ExpiredContracts, ec)
	}

	for _, rc := range cm.recoverableContracts {
		persist.RecoverableContracts = append(persist.RecoverableContracts, rc)
	}

	return
}

func (cm *ContractManager) saveSettings() (err error) {
	cm.lock.Lock()
	data := cm.persistUpdate()
	cm.lock.Unlock()

	return common.SaveDxJSON(settingsMetadata, filepath.Join(cm.persistDir, PersistFileName), data)
}

// loadSettings will load the storage contract manager settings saved
func (cm *ContractManager) loadSettings() (err error) {
	// make directory
	err = os.MkdirAll(cm.persistDir, 0700)
	if err != nil {
		return
	}

	// load data from the json file
	var data persistence
	err = common.LoadDxJSON(settingsMetadata, filepath.Join(cm.persistDir, PersistFileName), data)
	if err != nil {
		return
	}

	// data initialization
	cm.lock.Lock()
	cm.rentPayment = data.Rent
	cm.blockHeight = data.BlockHeight
	cm.currentPeriod = data.CurrentPeriod
	cm.renewedFrom = data.RenewedFrom
	cm.renewedTo = data.RenewedTo

	// update expired contract list and hostToContract mapping
	for _, ec := range data.ExpiredContracts {
		cm.expiredContracts[ec.ID] = ec
		cm.hostToContract[ec.EnodeID] = ec.ID
	}

	// update recoverable contracts list
	for _, rc := range data.RecoverableContracts {
		cm.recoverableContracts[rc.ID] = rc
	}
	cm.lock.Unlock()

	return
}
