// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package clientnegotiation

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// calculatePayouts calculates both storage client and storage host payouts
func CalculatePayoutsAndHostDeposit(hostInfo storage.HostInfo, funding common.BigInt, basePrice common.BigInt, baseDeposit common.BigInt, startHeight uint64, endHeight uint64, rentPayment storage.RentPayment) (clientPayout common.BigInt, hostPayout common.BigInt, hostDeposit common.BigInt, err error) {
	// calculate the period and expectedStorage
	period := endHeight - startHeight
	expectedStorage := rentPayment.ExpectedStorage / rentPayment.StorageHosts

	// calculate the client payout
	if clientPayout, err = calculateClientPayout(hostInfo.ContractPrice, funding, basePrice); err != nil {
		return
	}

	// calculate the host payout
	hostDeposit = calculateHostDeposit(clientPayout, hostInfo.StoragePrice, hostInfo.Deposit, hostInfo.MaxDeposit, baseDeposit, period, expectedStorage)
	hostPayout = calculateHostPayout(hostDeposit, hostInfo.ContractPrice, basePrice)
	return
}

// calculateClientPayout calculates the original payout for the storage client
func calculateClientPayout(contractPrice common.BigInt, funding common.BigInt, basePrice common.BigInt) (clientPayout common.BigInt, err error) {
	// calculate clientPayout
	clientPayout = funding.Sub(contractPrice).Sub(basePrice)

	// check if the clientPayout is smaller than 0
	if clientPayout.Cmp(common.BigInt0) < 0 {
		err = fmt.Errorf("client payout should not be smaller than 0. clientPayout: %v", clientPayout)
	}

	return
}

// calculateHostDeposit calculates the host deposit that storage host needed to put into the contract
func calculateHostDeposit(clientPayout common.BigInt, storagePrice common.BigInt, deposit common.BigInt, maxDeposit common.BigInt, baseDeposit common.BigInt, period uint64, expectedStorage uint64) (hostDeposit common.BigInt) {
	// sanity check, storagePrice cannot be smaller than or equal to 0
	if storagePrice.Cmp(common.BigInt0) <= 0 {
		storagePrice = common.NewBigIntUint64(1)
	}

	// calculate the deposit
	hostDeposit = clientPayout.Div(storagePrice).Mult(deposit).Add(baseDeposit)
	maxClientDeposit := deposit.Mult(common.NewBigIntUint64(period).Mult(common.NewBigIntUint64(expectedStorage)).Mult(common.NewBigIntUint64(5)))

	// compare hostDeposit and maxClientDeposit
	if hostDeposit.Cmp(maxClientDeposit) > 0 {
		hostDeposit = maxClientDeposit
	}

	// check if the host deposit is greater than maxHostDeposit
	if hostDeposit.Cmp(maxDeposit) > 0 {
		hostDeposit = maxDeposit
	}

	return
}

// calculateHostPayout calculates the original payout for the storage host
func calculateHostPayout(hostDeposit common.BigInt, contractPrice common.BigInt, basePrice common.BigInt) (hostPayout common.BigInt) {
	hostPayout = hostDeposit.Add(contractPrice).Add(basePrice)
	return
}
