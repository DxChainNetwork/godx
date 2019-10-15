// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package uploadnegotiate

import (
	"testing"

	"github.com/DxChainNetwork/godx/storage"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

func TestContractManager_UploadNegotiate(t *testing.T) {
	// create fake contract manager backend
	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()

	// generate contract params and contract manager
	params := simulation.ContractParamsGenerator()
	cm, err := NewFakeContractManager(negotiateTestBackend)
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}

	// create a contract first
	_, err = cm.ContractCreateNegotiate(params)
	if err != nil {
		t.Fatalf("contract create negotiation failed: %s", err.Error())
	}

	// generate some random upload actions and start to perform upload negotiation
	uploadActions := simulation.UploadActionsGenerator(0, storage.UploadActionAppend)
	sp, _ := negotiateTestBackend.SetupConnection("fake url")
	if err := cm.UploadNegotiate(sp, uploadActions, params.Host); err != nil {
		t.Fatalf("upload failed: %s", err.Error())
	}
}
