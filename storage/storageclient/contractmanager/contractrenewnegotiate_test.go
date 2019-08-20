// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"reflect"
	"testing"

	"github.com/DxChainNetwork/godx/storage/storageclient/contractmanager/simulation"
)

func TestContractManager_ContractRenewNegotiate(t *testing.T) {
	// create a fake contract manager backend
	var negotiateTestBackend = simulation.NewFakeContractManagerBackend()

	// generate contract params and contract manager
	params := simulation.ContractParamsGenerator()
	cm, err := NewFakeContractManager(negotiateTestBackend)
	if err != nil {
		t.Fatalf("failed to fake the contract manager: %s", err.Error())
	}

	// generate old contract information, and insert into db
	// update hostID for contractParams
	contract := simulation.ContractGenerator(100)
	params.Host.EnodeID = contract.EnodeID
	meta, err := cm.activeContracts.InsertContract(contract, nil)
	if err != nil {
		t.Fatalf("failed to insert contract into database: %s", err.Error())
	}

	// retrieve the contract from the db
	oldContract, exists := cm.activeContracts.Acquire(meta.ID)
	if !exists {
		t.Fatalf("contract inserted does not exist, please double check the insert method")
	}

	// contract renew negotiation
	renewedMeta, err := cm.ContractRenewNegotiate(oldContract, params)
	if err != nil {
		t.Fatalf("failed to renew the contract: %s", err.Error())
	}

	// check if the renewed contract information can be found in database
	retrieveRenewedMeta, exists := cm.RetrieveActiveContract(renewedMeta.ID)
	if !exists {
		t.Fatalf("the renewed contract cannot be found in the db")
	}

	if !reflect.DeepEqual(retrieveRenewedMeta, renewedMeta) {
		t.Fatalf("the contract retrieved from the db does not match with the one inserted")
	}

	// check if the old contract can be found in the database, the old contract should still in
	// the active contract list because the update function is not triggered
	oldContractMeta, exists := cm.RetrieveActiveContract(meta.ID)
	if !exists {
		t.Fatalf("the old contract should not be removed from the database")
	}

	if !reflect.DeepEqual(oldContractMeta, meta) {
		t.Fatalf("the oldContractMeta data retrieved from the database does not match with the meta data inserted into the db")
	}
}
