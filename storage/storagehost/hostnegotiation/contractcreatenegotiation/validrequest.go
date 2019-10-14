// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractcreatenegotiation

import (
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost/hostnegotiation"
)

// contractCreateReqValidation validates the contract create request. Currently, it only
// checks if the storage host has enough balance to pay for the deposit.
func contractCreateReqValidation(np hostnegotiation.Protocol, req storage.ContractCreateRequest) error {
	// data preparation
	hostAddress := req.StorageContract.ValidProofOutputs[validProofPaybackHostAddressIndex].Address
	hostDeposit := req.StorageContract.HostCollateral.Value

	// storage host balance validation
	if err := hostBalanceValidation(np, hostAddress, hostDeposit); err != nil {
		return err
	}

	return nil
}

// hostBalanceValidation checks if the storage host has enough balance to pay
// for the contract deposit
func hostBalanceValidation(np hostnegotiation.Protocol, hostAddress common.Address, hostDeposit *big.Int) error {
	// retrieve the stateDB
	stateDB, err := np.GetStateDB()
	if err != nil {
		return negotiationError(err.Error())
	}

	// validate the storage host balance
	if stateDB.GetBalance(hostAddress).Cmp(hostDeposit) < 0 {
		return errHostInsufficientBalance
	}

	return nil
}
