// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// renewCostEstimation will estimate the estimated cost for the contract period after the renew
func (cm *ContractManager) renewCostEstimation(host storage.HostInfo, contract storage.ContractMetaData, blockHeight uint64, rent storage.RentPayment) (estimation common.BigInt) {
	// get the cost for current storage
	amountDataStored := contract.LatestContractRevision.NewFileSize
	storageCost := host.StoragePrice.MultUint64(rent.Period).MultUint64(amountDataStored)

	// add all upload and download cost regarding to this contract
	// NOTE: only within the current period
	currentID := contract.ID
	prevContractTotalUploadCost := contract.UploadCost
	prevContractTotalDownloadCost := contract.DownloadCost

	cm.lock.RLock()

	// prevent loop from running forever
	for i := 0; i < 10e5; i++ {
		// get the previous contractID
		prevContractID, exists := cm.renewedFrom[currentID]
		if !exists {
			break
		}

		// get the previous contract information
		prevContract, exists := cm.expiredContracts[prevContractID]
		if !exists {
			break
		}

		// verify the start height to check if the start height is within the current period
		if prevContract.StartHeight < cm.currentPeriod {
			break
		}

		// update the upload and download cost. Note: this cost is spent within the current period
		prevContractTotalUploadCost = prevContractTotalUploadCost.Add(prevContract.UploadCost)
		prevContractTotalDownloadCost = prevContractTotalDownloadCost.Add(prevContract.DownloadCost)
		currentID = prevContractID
	}
	cm.lock.RUnlock()

	// amount of data uploaded within the current period
	prevDataUploaded := common.NewBigIntUint64(amountDataStored)
	if host.UploadBandwidthPrice.Cmp(common.BigInt0) > 0 {
		prevDataUploaded = prevContractTotalUploadCost.Div(host.UploadBandwidthPrice)
	}

	// if the data uploaded is greater than the total data stored in the contract
	if prevDataUploaded.Cmp(common.NewBigIntUint64(amountDataStored)) > 0 {
		prevDataUploaded = common.NewBigIntUint64(amountDataStored)
	}

	// upload cost + the storage cost for the newly uploaded data
	newUploadCost := prevContractTotalUploadCost.Add(prevDataUploaded.MultUint64(rent.Period).Mult(host.StoragePrice))

	// assume the download cost does not change for the new contract period
	newDownloadCost := prevContractTotalDownloadCost

	// contract price will be stayed the same
	contractPrice := host.ContractPrice

	// TODO (mzhang): Gas fee estimation is ignored currently

	estimation = storageCost.Add(newUploadCost).Add(newDownloadCost).Add(contractPrice)

	// rise the estimation up by 33%
	estimation = estimation.Add(estimation.DivUint64(3))

	// calculate the minimum rentPayment fund for the contract
	minRentFund := rent.Fund.MultFloat64(minContractPaymentFactor).DivUint64(rent.StorageHosts)

	if estimation.Cmp(minRentFund) < 0 {
		estimation = minRentFund
	}

	return
}

// CalculatePeriodCost will calculate the storage client's cost for one period (including all contracts)
func (cm *ContractManager) CalculatePeriodCost(rentPayment storage.RentPayment) (periodCost storage.PeriodCost) {
	activeContracts := cm.activeContracts.RetrieveAllContractsMetaData()

	cm.lock.RLock()
	defer cm.lock.RUnlock()

	for _, contract := range activeContracts {
		updatePrevContractCost(&periodCost, contract)
	}

	for _, contract := range cm.expiredContracts {
		host, exists := cm.hostManager.RetrieveHostInfo(contract.EnodeID)
		// it is possible that the expiredContract (got renewed but still not expired, old contract)
		// started within the current period. Therefore, add cost for that contract as well
		if contract.StartHeight >= cm.currentPeriod {
			updatePrevContractCost(&periodCost, contract)
		} else if exists && contract.EndHeight+host.WindowSize+maturityDelay > cm.blockHeight {
			// if the host exists, and the contract is still waiting for the storage proof
			// then it means the balance left in the contract is still withHeld and not
			// give back to the client yet
			periodCost.WithheldFund = periodCost.WithheldFund.Add(contract.ClientBalance)

			// update the withheldFundReleaseBlock to maximum block number
			if contract.EndHeight+host.WindowSize+maturityDelay >= periodCost.WithheldFundReleaseBlock {
				periodCost.WithheldFundReleaseBlock = contract.EndHeight + host.WindowSize + maturityDelay
			}

			// calculate the previous contract cost
			calculatePrevContractCost(&periodCost, contract)
		} else {
			// calculate the previous contract cost directly
			calculatePrevContractCost(&periodCost, contract)
		}
	}

	// calculate the unspent fund
	calculateContractUnspentFund(&periodCost, rentPayment.Fund)

	return
}

// calculateContractUnspentFund will be used to calculate the storage client's remaining contract fund
func calculateContractUnspentFund(pc *storage.PeriodCost, contractPayment common.BigInt) {
	totalContractCost := pc.ContractFees.Add(pc.UploadCost).Add(pc.DownloadCost).Add(pc.StorageCost)
	if contractPayment.Cmp(totalContractCost) > 0 {
		pc.UnspentFund = contractPayment.Sub(totalContractCost)
	}
}

// calculatePrevContractCost will be used to calculate the previous contract cost
func calculatePrevContractCost(pc *storage.PeriodCost, contract storage.ContractMetaData) {
	pc.PrevContractCost = pc.PrevContractCost.Add(contract.ContractFee).Add(contract.GasFee).
		Add(contract.UploadCost).Add(contract.DownloadCost).Add(contract.StorageCost)
}

// updatePrevContractCost will be used to update the previous contracts cost, which will be
// used to calculate the total storage client period cost
func updatePrevContractCost(pc *storage.PeriodCost, contract storage.ContractMetaData) {
	// calculate the contract fees
	pc.ContractFees = pc.ContractFees.Add(contract.ContractFee)
	pc.ContractFees = pc.ContractFees.Add(contract.GasFee)

	// calculate the upload, download, and storage cost
	pc.UploadCost = pc.UploadCost.Add(contract.UploadCost)
	pc.DownloadCost = pc.DownloadCost.Add(contract.DownloadCost)
	pc.StorageCost = pc.StorageCost.Add(contract.StorageCost)

	// get the total contract available payment
	pc.ContractFund = pc.ContractFund.Add(contract.TotalCost)
}
