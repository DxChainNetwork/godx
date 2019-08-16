// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"fmt"
	"os"

	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
)

const fakeContractManagerPersist = "testdata/fakecontractpersist"

// NewFakeContractManager will create and initialize a new fake contract manager
func NewFakeContractManager() (*ContractManager, error) {
	// clear the previously saved data before initialization
	clearOldData()

	// create fake storage host manager
	shm, err := storagehostmanager.NewFakeHostManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create the contract manager: %s", err.Error())
	}

	// create and initialize contract manager
	cm, err := New(fakeContractManagerPersist, shm)
	if err != nil {
		return nil, fmt.Errorf("failed to create the contract manager: %s", err.Error())
	}

	// start the contract manager
	if err := cm.Start(&storage.FakeClientBackend{}); err != nil {
		return nil, fmt.Errorf("failed to create the contract manager: %s", err.Error())
	}

	return cm, nil
}

// clearOldData will clear the previously saved old data
func clearOldData() {
	_ = os.RemoveAll("testdata/fakecontractpersist/")
}
