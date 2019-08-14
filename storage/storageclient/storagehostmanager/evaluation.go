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
			DepositFactor:          shm.depositFactorCalc(info, rent),
			InteractionFactor:      shm.interactionFactorCalc(info),
			ContractPriceFactor:    shm.contractPriceFactorCalc(info, rent),
			StorageRemainingFactor: shm.storageRemainingFactorCalc(info),
			UptimeFactor:           shm.uptimeFactorCalc(info),
		}
	}
}

// presenceFactorCalc calculates the factor value based on the existence of the
// storage host. The earlier it was discovered, the presence factor will be higher
func (shm *StorageHostManager) presenceFactorCalc(info storage.HostInfo) float64 {
	var base float64 = 1

	switch presence := shm.blockHeight - info.FirstSeen; {
	case presence < 0:
		return base
	case presence < 144:
		return base / 972
	case presence < 288:
		return base / 324
	case presence < 576:
		return base / 108
	case presence < 1000:
		return base / 36
	case presence < 2000:
		return base / 12
	case presence < 4000:
		return base / 6
	case presence < 6000:
		return base / 3
	case presence < 12000:
		return base * 2 / 3
	}

	return base
}

// depositFactorCalc calculates the factor value based on the storage host's deposit setting. The higher
// the deposit is, the higher evaluation it will get
func (shm *StorageHostManager) depositFactorCalc(info storage.HostInfo, rent storage.RentPayment) float64 {

	// make sure RentPayment's fields are non zeros
	rentPaymentValidation(storage.RentPayment{})

	contractExpectedFund := rent.Fund.DivUint64(rent.StorageHosts)
	contractExpectedStorage := float64(rent.ExpectedStorage) * rent.ExpectedRedundancy / float64(rent.StorageHosts)
	contractExpectedStorageTime := common.NewBigIntFloat64(contractExpectedStorage).MultUint64(rent.Period)

	// estimate storage host deposit
	hostDeposit := info.Deposit.Mult(contractExpectedStorageTime)

	possibleDeposit := info.MaxDeposit.Div(contractExpectedStorageTime)
	if possibleDeposit.Cmp(hostDeposit) < 0 {
		hostDeposit = possibleDeposit
	}

	cutoff := contractExpectedFund.MultFloat64(depositFloor)

	if hostDeposit.Cmp(cutoff) < 0 {
		cutoff = hostDeposit
	}

	if hostDeposit.Cmp(common.BigInt1) < 0 {
		hostDeposit = common.BigInt1
	}

	if cutoff.Cmp(common.BigInt1) < 0 {
		cutoff = common.BigInt1
	}

	ratio := hostDeposit.Div(cutoff)
	smallWeight := math.Pow(cutoff.Float64(), depositExponentialSmall)
	largeWeight := math.Pow(ratio.Float64(), depositExponentialLarge)

	return smallWeight * largeWeight
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
	rentPaymentValidation(rent)

	// calculate the host expected price
	expectedDataDownload := common.NewBigIntUint64(rent.ExpectedDownload).MultUint64(rent.Period).DivUint64(rent.StorageHosts)
	expectedDataUpload := common.NewBigIntUint64(rent.ExpectedUpload).MultUint64(rent.Period).DivUint64(rent.StorageHosts)
	expectedDataStored := common.NewBigIntUint64(rent.ExpectedStorage).MultFloat64(rent.ExpectedRedundancy).DivUint64(rent.StorageHosts)
	expectedDataStoredTime := expectedDataStored.MultUint64(rent.Period)

	// double the contract price due to one early contract renew (first one)
	contractPrice := info.ContractPrice.MultInt64(2)
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

// storageRemainingFactorCalc calculates the factor value based on the storage remaining, the more storage
// space the storage host remained, higher evaluation it will got
func (shm *StorageHostManager) storageRemainingFactorCalc(info storage.HostInfo) float64 {
	var base float64 = 1

	switch rs := info.RemainingStorage; {
	case rs < minStorage:
		return base / 1024
	case rs < 2*minStorage:
		return base / 512
	case rs < 3*minStorage:
		return base / 256
	case rs < 5*minStorage:
		return base / 128
	case rs < 10*minStorage:
		return base / 64
	case rs < 15*minStorage:
		return base / 32
	case rs < 20*minStorage:
		return base / 16
	case rs < 40*minStorage:
		return base / 8
	case rs < 80*minStorage:
		return base / 4
	case rs < 100*minStorage:
		return base / 2
	}

	return base
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

// rentPaymentValidation will validate the rent payment provided by the storage client
// eliminate any zero values by changing them to one
func rentPaymentValidation(rent storage.RentPayment) {
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
