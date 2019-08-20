// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

// InsertRandomActiveContracts will create some random contracts and inserted
// into active contract list
func (cm *ContractManager) InsertRandomActiveContracts(amount int) (err error) {
	for i := 0; i < amount; i++ {
		contract := simulation.ContractGenerator(100)
		if _, err = cm.activeContracts.InsertContract(contract, simulation.RootsGenerator(10)); err != nil {
			return
		}
	}
	return
}
