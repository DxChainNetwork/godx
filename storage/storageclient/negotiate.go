// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storageclient

import (
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/core/types"
)

// calculate client and host collateral
func ClientPayoutsPreTax(host StorageHostEntry, funding, basePrice, baseCollateral *big.Int, period, expectedStorage uint64) (clientPayout, hostPayout, hostCollateral *big.Int, err error) {

	// Divide by zero check.
	if host.StoragePrice.Sign() == 0 {
		host.StoragePrice.SetInt64(1)
	}

	// Underflow check.
	if funding.Cmp(host.ContractPrice) <= 0 {
		err = errors.New("underflow detected, funding < contractPrice")
		return
	}

	// Calculate clientPayout.
	clientPayout = new(big.Int).Sub(funding, host.ContractPrice)
	clientPayout = clientPayout.Sub(clientPayout, basePrice)

	// Calculate hostCollateral
	maxStorageSizeTime := new(big.Int).Div(clientPayout, host.StoragePrice)
	maxStorageSizeTime = maxStorageSizeTime.Mul(maxStorageSizeTime, host.Collateral)
	hostCollateral = maxStorageSizeTime.Add(maxStorageSizeTime, baseCollateral)
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(period))
	host.Collateral = host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(expectedStorage))
	maxClientCollateral := host.Collateral.Mul(host.Collateral, new(big.Int).SetUint64(5))
	if hostCollateral.Cmp(maxClientCollateral) > 0 {
		hostCollateral = maxClientCollateral
	}

	// Don't add more collateral than the host is willing to put into a single
	// contract.
	if hostCollateral.Cmp(host.MaxCollateral) > 0 {
		hostCollateral = host.MaxCollateral
	}

	// Calculate hostPayout.
	hostCollateral.Add(hostCollateral, host.ContractPrice)
	hostPayout = hostCollateral.Add(hostCollateral, basePrice)
	return
}

// update current storage contract revision with its revision number incremented, and cost transferred from the client to the host.
func NewRevision(current types.StorageContractRevision, cost *big.Int) types.StorageContractRevision {
	rev := current

	rev.NewValidProofOutputs = make([]types.DxcoinCharge, 2)
	rev.NewMissedProofOutputs = make([]types.DxcoinCharge, 2)
	copy(rev.NewValidProofOutputs, current.NewValidProofOutputs)
	copy(rev.NewMissedProofOutputs, current.NewMissedProofOutputs)

	// move valid payout from client to host
	rev.NewValidProofOutputs[0].Value = current.NewValidProofOutputs[0].Value.Sub(current.NewValidProofOutputs[0].Value, cost)
	rev.NewValidProofOutputs[1].Value = current.NewValidProofOutputs[1].Value.Add(current.NewValidProofOutputs[1].Value, cost)

	// move missed payout from client to void, mean that will burn missed payout of client
	rev.NewMissedProofOutputs[0].Value = current.NewMissedProofOutputs[0].Value.Sub(current.NewMissedProofOutputs[0].Value, cost)

	// increment revision number
	rev.NewRevisionNumber++

	return rev
}
