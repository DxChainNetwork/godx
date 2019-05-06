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

func (hm *StorageHostManager) calculateEvaluationFunc(rent storage.RentPayment) storagehosttree.EvaluationFunc {
	return func(info storage.HostInfo) storagehosttree.HostEvaluation {
		return storagehosttree.EvaluationCriteria{
			AgeAdjustment:              hm.ageAdjustment(info),
			BurnAdjustment:             1,
			DepositAdjustment:          hm.depositAdjustment(info, rent),
			InteractionAdjustment:      hm.interactionAdjustment(info),
			PriceAdjustment:            hm.priceAdjustment(info, rent),
			StorageRemainingAdjustment: hm.storageRemainingAdjustment(info),
			UptimeAdjustment:           hm.uptimeAdjustment(info),
		}
	}
}

func (hm *StorageHostManager) ageAdjustment(info storage.HostInfo) float64 {
	// TODO (mzhang): the value need to be discussed with the team
	base := float64(1)
	if hm.blockHeight >= info.FirstSeen {
		age := hm.blockHeight - info.FirstSeen
		if age < 12000 {
			base = base * 2 / 3
		}
		if age < 6000 {
			base = base / 2
		}
		if age < 4000 {
			base = base / 2
		}
		if age < 2000 {
			base = base / 2
		}
		if age < 1000 {
			base = base / 3
		}
		if age < 576 {
			base = base / 3
		}
		if age < 288 {
			base = base / 3
		}
		if age < 144 {
			base = base / 3
		}
	}
	return base
}

func (hm *StorageHostManager) depositAdjustment(info storage.HostInfo, rent storage.RentPayment) float64 {
	// make sure RentPayment's fields are non zeros
	rentPaymentValidation(storage.RentPayment{})

	contractExpectedPayment := rent.Payment.DivUint64(rent.StorageHosts)
	contractExpectedStorage := float64(rent.ExpectedStorage) * rent.ExpectedRedundancy / float64(rent.StorageHosts)
	contractExpectedStorageTime := common.NewBigIntFloat64(contractExpectedStorage).MultUint64(rent.Period)

	// estimate storage host deposit
	hostDeposit := info.Deposit.Mult(contractExpectedStorageTime)
	possibleDeposit := info.MaxDeposit.Div(contractExpectedStorageTime)
	if possibleDeposit.Cmp(hostDeposit) < 0 {
		hostDeposit = possibleDeposit
	}

	cutoff := contractExpectedPayment.MultFloat64(depositFloor)

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

func (hm *StorageHostManager) interactionAdjustment(info storage.HostInfo) float64 {
	hs := info.HistoricSuccessfulInteractions + 30
	hf := info.HistoricFailedInteractions + 1
	ratio := hs / (hs + hf)
	return math.Pow(ratio, interactionExponentiation)
}

func (hm *StorageHostManager) priceAdjustment(info storage.HostInfo, rent storage.RentPayment) float64 {
	// TODO (mzhang): gas estimation is ignored for now, no big effect on the result. Is it necessary
	// to add it back?

	// make sure the rent has non-zero fields
	rentPaymentValidation(rent)

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
	clientContractPayment := rent.Payment.DivUint64(rent.StorageHosts)

	cutoff := clientContractPayment.MultFloat64(priceFloor).Float64()

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

func (hm *StorageHostManager) storageRemainingAdjustment(info storage.HostInfo) float64 {
	base := float64(1)
	if info.RemainingStorage < 100*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 80*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 40*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 20*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 15*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 10*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 5*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 3*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < 2*minStorage {
		base = base / 2
	}
	if info.RemainingStorage < minStorage {
		base = base / 2
	}
	return base
}

// uptimeAdjustment will punish the storage host who are frequently been offline
func (hm *StorageHostManager) uptimeAdjustment(info storage.HostInfo) float64 {

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
		return hm.uptimeEvaluation(info)
	}
}

func (hm *StorageHostManager) uptimeEvaluation(info storage.HostInfo) float64 {
	downtime := info.HistoricDowntime
	uptime := info.HistoricUptime
	recentScanTime := info.ScanRecords[0].Timestamp
	recentScanSuccess := info.ScanRecords[0].Success

	for _, record := range info.ScanRecords[1:] {
		// scan time validation
		if recentScanTime.After(record.Timestamp) {
			hm.log.Warn("Warning: the scan records is not sorted based on the scan time")
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
