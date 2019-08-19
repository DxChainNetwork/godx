// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

func TestContractManager_ContractCreateNegotiate(t *testing.T) {
	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()

	params := simulation.ContractParamsGenerator()
	cm, err := NewFakeContractManager(negotiateTestBackend)
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}

	meta, err := cm.ContractCreateNegotiate(params)
	if err != nil {
		t.Fatalf("contract create negotiation failed: %s", err.Error())
	}

	// check the meta data
	rmeta, exist := cm.RetrieveActiveContract(meta.ID)
	if !exist {
		t.Fatalf("based on the contract ID returned from the contract create negotiate function, failed to get the contract information from database")
	}

	if !reflect.DeepEqual(rmeta, meta) {
		t.Fatalf("the contract received is not equivlent to the contract saved in db")
	}
}
