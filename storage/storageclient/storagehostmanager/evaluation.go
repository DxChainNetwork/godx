// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"math"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehosttree"
)

// calculateEvaluationFunc will generate the function that returns the storage host evaluation
// for each factor
func (shm *StorageHostManager) calculateEvaluationFunc(rent storage.RentPayment) storagehosttree.EvaluationFunc {
	return func(info storage.HostInfo) storagehosttree.HostEvaluation {
		return storagehosttree.EvaluationCriteria{
			PresenceFactor:         shm.presenceFactorCalc(info),
			DepositFactor:          shm.depositFactorCalc(info, rent, shm),
			InteractionFactor:      shm.interactionFactorCalc(info),
			ContractPriceFactor:    shm.contractPriceFactorCalc(info, rent),
			StorageRemainingFactor: shm.storageRemainingFactorCalc(info, rent),
			UptimeFactor:           shm.uptimeFactorCalc(info),
		}
	}
}

// presenceFactorCalc calculates the factor value based on the existence of the
// storage host. The earlier it was discovered, the presence factor will be higher
// The factor is linear to the presence duration, capped at lowValueLimit on lowTimeLimit,
// and highValueLimit on highTimeLimit.
func (shm *StorageHostManager) presenceFactorCalc(info storage.HostInfo) float64 {
	// If first seen is larger than current block height, return 0
	if shm.blockHeight < info.FirstSeen {
		return 0
	}
	presence := shm.blockHeight - info.FirstSeen

	if presence <= lowTimeLimit {
		return lowValueLimit
	} else if presence >= highTimeLimit {
		return highValueLimit
	} else {
		factor := lowValueLimit + (highValueLimit-lowValueLimit)/float64(highTimeLimit-lowTimeLimit)*float64(presence-lowTimeLimit)
		return factor
	}
}

// depositFactorCalc calculates the factor value based on the storage host's deposit setting. The higher
// the deposit is, the higher evaluation it will get
func (shm *StorageHostManager) depositFactorCalc(info storage.HostInfo, rent storage.RentPayment, market hostMarket) float64 {
	// Evaluate the deposit of the host
	hostDeposit := evalHostDeposit(info, rent)
	// Evaluate the deposit of the market
	marketDeposit := evalHostMarketDeposit(rent, market)

	// DepositFactor is the function based on ratio between hostDeposit and marketDeposit.
	// The function is (x/n)/((x/n) + 1) n is the base divider which is float.
	// The larger the divider, the larger the deposit is to be encouraged
	ratio := hostDeposit.Float64() / marketDeposit.Float64()
	factor := ratio / (ratio + depositBaseDivider)
	return factor
}

// evalHostMarketDeposit evaluate the deposit based on market evaluate price
func evalHostMarketDeposit(settings storage.RentPayment, market hostMarket) common.BigInt {
	// Evaluate host deposit for market price
	marketPrice := market.GetMarketPrice()
	// Make the host info with necessary info from market price
	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			ContractPrice: marketPrice.ContractPrice,
			StoragePrice:  marketPrice.StoragePrice,
			Deposit:       marketPrice.Deposit,
			MaxDeposit:    marketPrice.MaxDeposit,
		},
	}
	return evalHostDeposit(info, settings)
}

// evalHostDeposit calculate the host deposit with host info and client rentPayment settings
func evalHostDeposit(info storage.HostInfo, settings storage.RentPayment) common.BigInt {
	// Update the rentPayment to non-zeros
	updateRentPaymentNonZero(&settings)

	// Calculate the contract fund.
	contractFund := estimateContractFund(settings)

	// Calculate the deposit
	deposit := contractFund.Sub(info.ContractPrice).Div(info.StoragePrice).Mult(info.Deposit)
	// If not enough fund, treat as 0
	if deposit.IsNeg() {
		deposit = common.BigInt0
	}
	// If deposit is larger than max deposit, set to max deposit
	if deposit.Cmp(info.MaxDeposit) > 0 {
		deposit = info.MaxDeposit
	}
	return deposit
}

// estimateContractFund estimate the contract fund from client settings.
// Renter fund is split among the hosts and Evaluated as 2/3 of the total fund
// TODO: implement this function which is used in contract manager, which should be used in
//       storage client
func estimateContractFund(settings storage.RentPayment) common.BigInt {
	return settings.Fund.MultUint64(2).DivUint64(3).DivUint64(settings.StorageHosts)
}

// storageRemainingFactorCalc calculates the factor value based on the storage remaining, the more storage
// space the storage host remained, higher evaluation it will got. The baseline for storage is set to
// required storage * storageBaseDivider
func (shm *StorageHostManager) storageRemainingFactorCalc(info storage.HostInfo, settings storage.RentPayment) float64 {
	ratio := float64(info.RemainingStorage) / float64(expectedStoragePerContract(settings))
	factor := ratio / (ratio + storageBaseDivider)
	return factor
}

// expectedStoragePerContract evaluate the storage per contract.
// TODO: The function should be imported from contract manager
func expectedStoragePerContract(settings storage.RentPayment) uint64 {
	return settings.ExpectedStorage * uint64(storage.DefaultNumSectors) / uint64(storage.DefaultMinSectors)
}

// uptimeFactorCalc will punish the storage host who are frequently been offline
func (shm *StorageHostManager) uptimeFactorCalc(info storage.HostInfo) float64 {
	switch length := len(info.ScanRecords); length {
	case 0:
		return 0.25
	case 1:
		if info.ScanRecords[0].Success {
			return 0.75
		}
		return 0.25
	case 2:
		if info.ScanRecords[0].Success && info.ScanRecords[1].Success {
			return 0.85
		}
		if info.ScanRecords[0].Success || info.ScanRecords[1].Success {
			return 0.50
		}
		return 0.05
	default:
		return shm.uptimeEvaluation(info)
	}
}

// interactionFactorCalc calculates the factor value based on the historical success interactions
// and failed interactions. More success interactions will cause higher evaluation
func (shm *StorageHostManager) interactionFactorCalc(info storage.HostInfo) float64 {
	hs := info.HistoricSuccessfulInteractions + 30
	hf := info.HistoricFailedInteractions + 1
	ratio := hs / (hs + hf)
	return math.Pow(ratio, interactionExponentiation)
}

// contractPriceFactorCalc calculates the factor value based on the contract price that storage host requested
// the lower the price is, the higher the storage host evaluation will be
func (shm *StorageHostManager) contractPriceFactorCalc(info storage.HostInfo, rent storage.RentPayment) float64 {
	// make sure the rent has non-zero fields
	updateRentPaymentNonZero(&rent)

	// calculate the host expected price
	expectedDataDownload := common.NewBigIntUint64(rent.ExpectedDownload).MultUint64(rent.Period).DivUint64(rent.StorageHosts)
	expectedDataUpload := common.NewBigIntUint64(rent.ExpectedUpload).MultUint64(rent.Period).DivUint64(rent.StorageHosts)
	expectedDataStored := common.NewBigIntUint64(rent.ExpectedStorage).MultFloat64(rent.ExpectedRedundancy).DivUint64(rent.StorageHosts)
	expectedDataStoredTime := expectedDataStored.MultUint64(rent.Period)

	// double the contract price due to one early contract renew (first one)
	contractPrice := info.ContractPrice.MultInt(2)
	downloadPrice := expectedDataDownload.Mult(info.DownloadBandwidthPrice)
	uploadPrice := expectedDataUpload.Mult(info.UploadBandwidthPrice)
	storagePrice := expectedDataStoredTime.Mult(info.StoragePrice)
	hostContractPrice := contractPrice.Add(downloadPrice).Add(uploadPrice).Add(storagePrice).Float64()

	// storage client expected payment per contract
	clientContractFund := rent.Fund.DivUint64(rent.StorageHosts)

	cutoff := clientContractFund.MultFloat64(priceFloor).Float64()

	if hostContractPrice < cutoff {
		cutoff = hostContractPrice
	}

	// sanity check
	if hostContractPrice < 1 {
		hostContractPrice = 1
	}

	if cutoff < 1 {
		cutoff = 1
	}

	ratio := hostContractPrice / cutoff

	smallWeight := math.Pow(cutoff, priceExponentiationSmall)
	largeWeight := math.Pow(ratio, priceExponentiationLarge)

	return 1 / (smallWeight * largeWeight)
}

// uptimeEvaluation will evaluate the uptime the storage host has
func (shm *StorageHostManager) uptimeEvaluation(info storage.HostInfo) float64 {
	downtime := info.HistoricDowntime
	uptime := info.HistoricUptime
	recentScanTime := info.ScanRecords[0].Timestamp
	recentScanSuccess := info.ScanRecords[0].Success

	for _, record := range info.ScanRecords[1:] {
		// scan time validation
		if recentScanTime.After(record.Timestamp) {
			shm.log.Warn("the scan records is not sorted based on the scan time")
			continue
		}

		if recentScanSuccess {
			uptime += record.Timestamp.Sub(recentScanTime)
		} else {
			downtime += record.Timestamp.Sub(recentScanTime)
		}

		// update the recentScanTime and recentScanSuccess
		recentScanTime = record.Timestamp
		recentScanSuccess = record.Success
	}

	// if both time are 0
	if uptime == 0 && downtime == 0 {
		return 0.001
	}

	// calculate uptime ratio
	uptimeRatio := float64(uptime) / float64(uptime+downtime)
	if uptimeRatio > 0.98 {
		uptimeRatio = 0.98
	}
	uptimeRatio += 0.02

	maxDowntime := 0.03 * float64(len(info.ScanRecords))
	if uptimeRatio < 1-maxDowntime {
		uptimeRatio = 1 - maxDowntime
	}

	exp := 200 * math.Min(1-uptimeRatio, 0.30)
	return math.Pow(uptimeRatio, exp)
}

// updateRentPaymentNonZero check the rent, and update the zero fields to 1
func updateRentPaymentNonZero(rent *storage.RentPayment) {
	if rent.StorageHosts == 0 {
		rent.StorageHosts = 1
	}
	if rent.Period == 0 {
		rent.Period = 1
	}
	if rent.ExpectedStorage == 0 {
		rent.ExpectedStorage = 1
	}
	if rent.ExpectedUpload == 0 {
		rent.ExpectedUpload = 1
	}
	if rent.ExpectedDownload == 0 {
		rent.ExpectedDownload = 1
	}
	if rent.ExpectedRedundancy == 0 {
		rent.ExpectedRedundancy = 1
	}
}
