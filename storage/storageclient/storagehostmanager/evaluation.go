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
	if marketDeposit.Cmp(common.BigInt0) == 0 {
		marketDeposit = common.BigInt1
	}
	ratio := hostDeposit.Float64() / marketDeposit.Float64()
	factor := ratio / (ratio + depositBaseDivider)
	return factor
}

// storageRemainingFactorCalc calculates the factor value based on the storage remaining, the more storage
// space the storage host remained, higher evaluation it will got. The baseline for storage is set to
// required storage * storageBaseDivider
func (shm *StorageHostManager) storageRemainingFactorCalc(info storage.HostInfo, settings storage.RentPayment) float64 {
	ratio := float64(info.RemainingStorage) / float64(expectedStoragePerContract(settings))
	factor := ratio / (ratio + storageBaseDivider)
	return factor
}

// interactionFactorCalc calculates the factor value based on the historical success interactions
// and failed interactions. More success interactions will cause higher evaluation
func (shm *StorageHostManager) interactionFactorCalc(info storage.HostInfo) float64 {
	successRatio := info.SuccessfulInteractionFactor / (info.SuccessfulInteractionFactor + info.FailedInteractionFactor)
	return math.Pow(successRatio, interactionExponentialIndex)
}

// contractPriceFactorCalc calculates the factor value based on the contract price that storage host requested
// the lower the price is, the higher the storage host evaluation will be
func (shm *StorageHostManager) contractPriceFactorCalc(info storage.HostInfo, rent storage.RentPayment, market hostMarket) float64 {
	// regulate host info and rent payment
	regulateHostInfo(&info)
	regulateRentPayment(&rent)

	// Evaluate the cost of host and market
	hostContractCost := evalContractCost(info, rent)
	marketContractCost := evalMarketContractCost(market, rent)
	if marketContractCost.Cmp(common.BigInt0) <= 0 {
		marketContractCost = common.BigInt1
	}

	// calculate the ratio
	ratio := hostContractCost.Float64() / marketContractCost.Float64()
	// If ratio is smaller than 0.1, the factor has value 10; Else the factor has value 1/x
	if ratio <= 0.1 {
		return 10
	} else {
		return 1 / ratio
	}
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

// evalHostDeposit calculate the host deposit with host info and client rentPayment settings
func evalHostDeposit(info storage.HostInfo, settings storage.RentPayment) common.BigInt {
	// regulate the rentPayment to non-zeros
	regulateRentPayment(&settings)
	// regulate the host info
	regulateHostInfo(&info)

	// Calculate the contract fund.
	contractFund := estimateContractFund(settings)

	// Calculate the deposit
	storageFund := contractFund.Sub(info.ContractPrice)
	if storageFund.Cmp(common.NewBigIntUint64(0)) < 0 {
		storageFund = common.BigInt0
	}
	deposit := storageFund.Div(info.StoragePrice).Mult(info.Deposit)
	// If deposit is larger than max deposit, set to max deposit
	if deposit.Cmp(info.MaxDeposit) > 0 {
		deposit = info.MaxDeposit
	}
	return deposit
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

// evalContractCost evaluate the host price based on host's financial settings
// and client's expected storage sizes. The storage price is estimated as the sum
// of contract price, storage price, upload price and download price
func evalContractCost(info storage.HostInfo, settings storage.RentPayment) common.BigInt {
	// Calculate the contract price
	contractPrice := info.ContractPrice.MultInt(2)
	// Calculate the storage price
	storagePrice := info.StoragePrice.MultUint64(settings.Period).MultUint64(expectedStoragePerContract(settings))
	// Calculate the upload price
	uploadPrice := info.UploadBandwidthPrice.MultUint64(expectedUploadSizePerContract(settings))
	// Calculate the download price
	downloadPrice := info.DownloadBandwidthPrice.MultUint64(expectedDownloadSizePerContract(settings))

	// sum up all cost
	sum := common.BigInt0.Add(contractPrice).Add(storagePrice).Add(uploadPrice).Add(downloadPrice)
	return sum
}

// evalMarketContractCost evaluate the market contract price cost
func evalMarketContractCost(market hostMarket, settings storage.RentPayment) common.BigInt {
	// Get the price from market
	marketPrice := market.GetMarketPrice()

	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			ContractPrice:          marketPrice.ContractPrice,
			StoragePrice:           marketPrice.StoragePrice,
			UploadBandwidthPrice:   marketPrice.UploadPrice,
			DownloadBandwidthPrice: marketPrice.DownloadPrice,
		},
	}
	return evalContractCost(info, settings)
}

// regulateRentPayment check the rent, and update the zero fields to 1
func regulateRentPayment(rent *storage.RentPayment) {
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

// regulateHostInfo regulate the host info. If it has negative values, change it to 0;
// If some specified fields (storage price)  have zero values, change it to 1;
func regulateHostInfo(info *storage.HostInfo) {
	if info.Deposit.IsNeg() {
		info.Deposit = common.BigInt0
	}
	if info.MaxDeposit.IsNeg() {
		info.MaxDeposit = common.BigInt0
	}
	if info.ContractPrice.IsNeg() {
		info.ContractPrice = common.BigInt0
	}
	if info.DownloadBandwidthPrice.IsNeg() {
		info.DownloadBandwidthPrice = common.BigInt0
	}
	if info.UploadBandwidthPrice.IsNeg() {
		info.UploadBandwidthPrice = common.BigInt0
	}
	if info.StoragePrice.Cmp(common.BigInt0) <= 0 {
		info.StoragePrice = common.BigInt1
	}
}

// estimateContractFund estimate the contract fund from client settings.
// Renter fund is split among the hosts and Evaluated as 2/3 of the total fund
// TODO: implement this function which is used in contract manager, which should be used in
//       storage client
func estimateContractFund(settings storage.RentPayment) common.BigInt {
	return settings.Fund.MultUint64(2).DivUint64(3).DivUint64(settings.StorageHosts)
}

// expectedStoragePerContract evaluate the storage per contract.
// TODO: The function should be imported from contract manager
func expectedStoragePerContract(settings storage.RentPayment) uint64 {
	return settings.ExpectedStorage * uint64(storage.DefaultNumSectors) / uint64(storage.DefaultMinSectors) / uint64(settings.StorageHosts)
}

// expectedUploadSizePerContract evaluate the expected upload size per contract
func expectedUploadSizePerContract(settings storage.RentPayment) uint64 {
	return settings.ExpectedUpload / uint64(settings.StorageHosts)
}

// expectedDownloadSizePerContract evaluate the expected download size per contract
func expectedDownloadSizePerContract(settings storage.RentPayment) uint64 {
	return settings.ExpectedDownload / uint64(settings.StorageHosts)
}
