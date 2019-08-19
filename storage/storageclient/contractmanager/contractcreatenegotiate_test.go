// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"testing"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

// NOTE: the purpose of this test case is not to locate errors, it is just used to
// see if the faked data works as expected
func TestContractManager_ContractCreateNegotiate(t *testing.T) {
	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()

	params := simulation.ContractParamsGenerator()
	cm, err := NewFakeContractManager(negotiateTestBackend)
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}

	_, err = cm.ContractCreateNegotiate(params)
	if err != nil {
		t.Fatalf("contract create negotiation failed: %s", err.Error())
	}
}
