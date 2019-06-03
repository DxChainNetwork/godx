// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
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
	Rent             storage.RentPayment           `json:"rentPayment"`
	BlockHeight      uint64                        `json:"blockheight"`
	CurrentPeriod    uint64                        `json:"currentperiod"`
	ExpiredContracts []storage.ContractMetaData    `json:"expiredcontracts"`
	RenewedFrom      map[string]storage.ContractID `json:"renewedfrom"`
	RenewedTo        map[string]storage.ContractID `json:"renewedto"`
}

func (cm *ContractManager) persistUpdate() (persist persistence) {
	persist = persistence{
		Rent:          cm.rentPayment,
		BlockHeight:   cm.blockHeight,
		CurrentPeriod: cm.currentPeriod,
		RenewedFrom:   make(map[string]storage.ContractID),
		RenewedTo:     make(map[string]storage.ContractID),
	}

	// update the renewedFrom
	for key, value := range cm.renewedFrom {
		persist.RenewedFrom[key.String()] = value
	}

	// update the renewedTo
	for key, value := range cm.renewedTo {
		persist.RenewedTo[key.String()] = value
	}

	// update the expiredContracts
	for _, ec := range cm.expiredContracts {
		persist.ExpiredContracts = append(persist.ExpiredContracts, ec)
	}

	return
}

// saveSettings will store all the persistence data into the JSON file
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

	// update the RenewedFrom
	for key, value := range data.RenewedFrom {
		// convert the string to contract id
		id, err := storage.StringToContractID(key)
		if err != nil {
			cm.log.Warn(fmt.Sprintf("contractmanager loadsettings renewedFrom: %s", err.Error()))
			continue
		}

		// update the renewedFrom
		cm.renewedFrom[id] = value
	}

	// update the RenewedTo
	for key, value := range data.RenewedTo {
		// convert the string to contract id
		id, err := storage.StringToContractID(key)
		if err != nil {
			cm.log.Warn(fmt.Sprintf("contractmanager loadsettings renewedTo: %s", err.Error()))
			continue
		}

		// update the renewedFrom
		cm.renewedTo[id] = value
	}

	// update expired contract list and hostToContract mapping
	for _, ec := range data.ExpiredContracts {
		cm.expiredContracts[ec.ID] = ec
		cm.hostToContract[ec.EnodeID] = ec.ID
	}
	cm.lock.Unlock()

	return
}
