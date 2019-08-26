// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/storage"
)

func init() {
	// initialize the erasure coding related params
	defaultNumSectors = 2
	defaultMinSectors = 1
}

// TestNewDefaultEvaluator test the functionality of newDefaultEvaluator
func TestNewDefaultEvaluator(t *testing.T) {
	shm := &StorageHostManager{}
	rent := storage.RentPayment{
		StorageHosts:       0,
		Period:             0,
		RenewWindow:        0,
		ExpectedStorage:    0,
		ExpectedUpload:     0,
		ExpectedDownload:   0,
		ExpectedRedundancy: 0,
	}
	de := newDefaultEvaluator(shm, rent)
	if de.rent.StorageHosts == 0 {
		t.Errorf("zero storage host is not corrected")
	}
	if de.rent.Period == 0 {
		t.Errorf("zero period is not corrected")
	}
	if de.rent.ExpectedUpload == 0 {
		t.Errorf("zero expected upload is not corrected")
	}
	if de.rent.ExpectedDownload == 0 {
		t.Errorf("zero expected download is not corrected")
	}
	if de.rent.ExpectedStorage == 0 {
		t.Errorf("zero expected storage is not corrected")
	}
	if de.rent.ExpectedRedundancy == 0 {
		t.Errorf("zero expectedRedundancy is not corrected")
	}
}

// TestEvaluateNegative test the the condition of zero or negative settings and check whether it might
// result in panic condition
func TestDefaultEvaluator_EvaluateDetailNegative(t *testing.T) {
	// Zeros in rent payment
	rent := storage.RentPayment{}
	// Negative settings in host info
	info := storage.HostInfo{
		HostExtConfig: storage.HostExtConfig{
			Deposit:                common.NewBigInt(-100),
			MaxDeposit:             common.NewBigInt(-100),
			ContractPrice:          common.NewBigInt(-100),
			DownloadBandwidthPrice: common.NewBigInt(-100),
			StoragePrice:           common.NewBigInt(-100),
			UploadBandwidthPrice:   common.NewBigInt(-100),
		},
		FirstSeen: 100,
	}
	shm := &StorageHostManager{}
	// Evaluate the details of the corner cases. The function shall never panic
	detail := newDefaultEvaluator(shm, rent).EvaluateDetail(info)
	// Check the result of the details. The scores should all be zero
	if detail.Evaluation < 0 {
		t.Errorf("evaluation is negative: %v", detail.Evaluation)
	}
	if detail.PresenceScore < 0 {
		t.Errorf("presence score is negative: %v", detail.PresenceScore)
	}
	if detail.DepositScore < 0 {
		t.Errorf("deposit score is negative: %v", detail.DepositScore)
	}
	if detail.InteractionScore < 0 {
		t.Errorf("interaction score is negative: %v", detail.InteractionScore)
	}
	if detail.ContractPriceScore < 0 {
		t.Errorf("contract price score is negative: %v", detail.ContractPriceScore)
	}
	if detail.StorageRemainingScore < 0 {
		t.Errorf("storage remaining score is negative: %v", detail.StorageRemainingScore)
	}
	if detail.UptimeScore < 0 {
		t.Errorf("uptime score is negative: %v", detail.UptimeScore)
	}
}

// TestDefaultEvaluator_calFinalScore test defaultEvaluator.calcFinalScore
func TestDefaultEvaluator_calFinalScore(t *testing.T) {
	tests := []struct {
		scs    defaultEvaluationScores
		expect int64
	}{
		{scs: defaultEvaluationScores{1, 1, 1, 1, 1, 1}, expect: scoreDefaultBase},
		{scs: defaultEvaluationScores{0, 0, 0, 0, 0, 0}, expect: minScore},
	}
	for i, test := range tests {
		de := &defaultEvaluator{}
		sc := de.calcFinalScore(&test.scs)
		if sc != test.expect {
			t.Errorf("Test %v: unexpected score. Got %v, Expect %v", i, sc, test.expect)
		}
	}
}

func TestPresenceScoreCalc(t *testing.T) {
	tests := []struct {
		presence uint64
	}{
		{lowTimeLimit},
		{lowTimeLimit + 1},
		{10},
		{100},
		{1000},
		{10000},
		{100000},
		{highTimeLimit - 1},
		{highTimeLimit},
		{highTimeLimit + 1},
	}
	for _, test := range tests {
		// first Seen must not be 0
		firstSeen := uint64(10)
		hm := &fakeHostMarket{
			blockHeight: test.presence + firstSeen,
		}
		info := storage.HostInfo{
			FirstSeen: firstSeen,
		}
		score := presenceScoreCalc(info, hm)
		if test.presence <= lowTimeLimit {
			if score != lowValueLimit {
				t.Errorf("low limit test failed")
			}
		} else if test.presence >= highTimeLimit {
			if score != highValueLimit {
				t.Errorf("high limit test failed")
			}
		} else {
			// Calculate the value before and after the limit, and check whether the
			// score is incrementing
			if test.presence == 0 || test.presence == math.MaxUint64 {
				continue
			}
			factorSmaller := presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen + 1}, hm)
			factorLarger := presenceScoreCalc(storage.HostInfo{FirstSeen: firstSeen - 1}, hm)
			if factorSmaller >= score || score >= factorLarger {
				t.Errorf("Near range %d the score not incrementing", test.presence)
			}
		}
	}
}

// TestIllegalPresenceFactorCalc test the illegal case of host manager's blockHeight is smaller
// than the first seen, which should yield a factor of 0
func TestIllegalPresenceScoreCalc(t *testing.T) {
	firstSeen := uint64(30)
	blockHeight := uint64(10)
	hm := &fakeHostMarket{
		blockHeight: blockHeight,
	}
	info := storage.HostInfo{
		FirstSeen: firstSeen,
	}
	score := presenceScoreCalc(info, hm)
	if firstSeen > blockHeight && score != 0 {
		t.Errorf("Illegal input for presence factor calculation does not give 0 factor")
	}
}

// TestDepositFactorCalc test the functionality of StorageHostManagerdepositFactorCalc
func TestDepositScoreCalc(t *testing.T) {
	tests := []struct {
		ratio float64
	}{
		{0},
		{0.001},
		{0.01},
		{0.1},
		{1},
		{10},
		{100},
		{1000},
	}
	type record struct{ ratio, sc float64 }
	var results []record
	for index, test := range tests {
		rent := storage.RentPayment{
			Fund:         common.NewBigIntUint64(10000000),
			StorageHosts: 1,
		}
		marketDeposit := common.NewBigIntUint64(10000)
		storagePrice := common.NewBigIntUint64(5000)
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				StoragePrice: storagePrice,
				Deposit:      marketDeposit.MultFloat64(test.ratio),
				MaxDeposit:   common.NewBigIntUint64(math.MaxUint64),
			},
		}
		market := &fakeHostMarket{
			storagePrice: storagePrice,
			deposit:      marketDeposit,
			maxDeposit:   common.NewBigIntUint64(math.MaxUint64),
		}
		res := depositScoreCalc(info, rent, market)
		// Check the result is within range [0, 1)
		if res < 0 || res >= 1 {
			t.Errorf("Test %d illegal factor. Got %v", index, res)
		}
		results = append(results, record{test.ratio, res})
	}
	for i := 0; i != len(results)-1; i++ {
		if results[i].sc >= results[i+1].sc {
			t.Fatalf("score not incrementing. [%v: %v] -> [%v: %v]", results[i].ratio, results[i].sc,
				results[i+1].ratio, results[i+1].sc)
		}
	}
	// Display the results
	for _, result := range results {
		t.Logf("At storage ratio %7.4f, got score %7.4f", result.ratio, result.sc)
	}
}

// TestEvalHostMarketDeposit test the normal functionality of TestEvalHostMarketDeposit
func TestEvalHostMarketDeposit(t *testing.T) {
	tests := []struct {
		contractPrice common.BigInt
		storagePrice  common.BigInt
		deposit       common.BigInt
		maxDeposit    common.BigInt

		fund     common.BigInt
		numHosts uint64

		expectedDeposit common.BigInt
	}{
		{
			contractPrice:   common.NewBigIntUint64(0),
			storagePrice:    common.NewBigIntUint64(100),
			deposit:         common.NewBigIntUint64(200),
			maxDeposit:      common.NewBigIntUint64(30000),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			expectedDeposit: common.NewBigIntUint64(20000),
		},
	}
	for index, test := range tests {
		market := &fakeHostMarket{
			contractPrice: test.contractPrice,
			storagePrice:  test.storagePrice,
			deposit:       test.deposit,
			maxDeposit:    test.maxDeposit,
		}
		settings := storage.RentPayment{
			Fund:         test.fund,
			StorageHosts: test.numHosts,
		}
		res := evalHostMarketDeposit(settings, market)
		if res.Cmp(test.expectedDeposit) != 0 {
			t.Errorf("test %d deposit not expected. Got %v, Expect %v", index, res, test.expectedDeposit)
		}
	}
}

// TestEvalHostDeposit test evalHostDeposit
func TestEvalHostDeposit(t *testing.T) {
	tests := []struct {
		contractPrice          common.BigInt
		fund                   common.BigInt
		numHosts               uint64
		storagePrice           common.BigInt
		depositPrice           common.BigInt
		maxDoposit             common.BigInt
		uploadBandwithPrice    common.BigInt
		downloadBandwidthPrice common.BigInt
		expectedDeposit        common.BigInt
	}{
		{
			// normal case, the maxDeposit is larger than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(30000),
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, multiple hosts
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(45000),
			numHosts:        3,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(30000),
			expectedDeposit: common.NewBigIntUint64(20000),
		},
		{
			// normal case, the maxDeposit is smaller than deposit requested
			contractPrice:   common.NewBigIntUint64(0),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(15000), // maxDeposit is not enough
			expectedDeposit: common.NewBigIntUint64(15000), // result capped at max deposit
		},
		{
			// client fund is not enough to pay the contract price
			contractPrice:   common.NewBigIntUint64(20000),
			fund:            common.NewBigIntUint64(15000),
			numHosts:        1,
			storagePrice:    common.NewBigIntUint64(100),
			depositPrice:    common.NewBigIntUint64(200),
			maxDoposit:      common.NewBigIntUint64(15000),
			expectedDeposit: common.BigInt0,
		},
	}
	for index, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          test.contractPrice,
				StoragePrice:           test.storagePrice,
				Deposit:                test.depositPrice,
				MaxDeposit:             test.maxDoposit,
				UploadBandwidthPrice:   test.uploadBandwithPrice,
				DownloadBandwidthPrice: test.downloadBandwidthPrice,
			},
		}
		settings := storage.RentPayment{
			Fund:         test.fund,
			StorageHosts: test.numHosts,
		}
		res := evalHostDeposit(info, settings)
		if res.Cmp(test.expectedDeposit) != 0 {
			t.Errorf("Test %d deposit not expected. Got %v, Expect %v", index, res, test.expectedDeposit)
		}
	}
}

// TestStorageRemainingFactorCalc test the functionality of storageRemainingScoreCalc
func TestStorageRemainingFactorCalc(t *testing.T) {
	expectedStorage := uint64(1e6)
	tests := []struct {
		remainingStorage uint64
		numHosts         uint64
	}{
		{0, 1},
		{1e3, 1},
		{3.3e3, 3},
		{3e4, 3},
		{3e5, 3},
		{3e6, 3},
		{3e7, 3},
		{3e8, 3},
		{3e9, 3},
	}
	type record struct {
		ratio float64
		sc    float64
	}
	var results []record
	for _, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: test.remainingStorage,
			},
		}
		settings := storage.RentPayment{
			ExpectedStorage: expectedStorage,
			StorageHosts:    test.numHosts,
		}
		ratio := float64(test.remainingStorage) / float64(expectedStoragePerContract(settings))
		res := storageRemainingScoreCalc(info, settings)
		if res < 0 || res >= 1 {
			t.Errorf("invalid result: %v", res)
		}
		results = append(results, record{ratio, res})
	}
	for i := 0; i != len(results)-1; i++ {
		if results[i].sc >= results[i+1].sc {
			t.Fatalf("score not incrementing. [%v: %v] -> [%v: %v]", results[i].ratio, results[i].sc,
				results[i+1].ratio, results[i+1].sc)
		}
	}
	// Display the results
	for _, result := range results {
		t.Logf("At storage ratio %7.4f, got score %7.4f", result.ratio, result.sc)
	}
}

func TestContractCostScoreCalc(t *testing.T) {
	tests := []struct {
		priceRatio float64
	}{
		{0.01}, {0.1}, {1}, {2}, {3}, {5}, {10}, {100},
	}
	for i, test := range tests {
		// The price ratio is obtained by setting the contract price only
		marketContractPrice := common.NewBigInt(1e6)
		hostContractPrice := marketContractPrice.MultFloat64(test.priceRatio)
		m := &fakeHostMarket{contractPrice: marketContractPrice}
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice: hostContractPrice,
			},
		}
		rent := storage.RentPayment{
			StorageHosts: 1,
		}
		res := contractCostScoreCalc(info, rent, m)
		if res > 10 || res < 0 {
			t.Fatalf("Test %v: invalid score: %v", i, res)
		}
	}
}

// TestStorageRemainingScoreCalc test the functionality of storageRemainingScoreCalc.
// The returned score should be within range [0, 1), and increment as remaining storage increases
func TestStorageRemainingScoreCalc(t *testing.T) {
	baseStorage := uint64(10000000000)
	tests := []struct {
		remainingStorage uint64
		expectedStorage  uint64
		numHosts         uint64
	}{
		{0, baseStorage, 1},
		{uint64(0.1 * float64(baseStorage)), baseStorage, 1},
		{uint64(0.5 * float64(baseStorage)), baseStorage, 1},
		{baseStorage, baseStorage * 30, 30},
		{baseStorage * 3, baseStorage, 1},
		{10 * baseStorage, baseStorage, 1},
		{30 * baseStorage, baseStorage, 1},
		{100 * baseStorage, baseStorage, 1},
	}
	type scoreRecord struct{ ratio, sc float64 }
	var results []scoreRecord
	for _, test := range tests {
		ratio := float64(test.remainingStorage) * float64(test.numHosts) * float64(defaultMinSectors) /
			float64(defaultNumSectors) / float64(test.expectedStorage)
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				RemainingStorage: test.remainingStorage,
			},
		}
		settings := storage.RentPayment{
			ExpectedStorage: test.expectedStorage,
			StorageHosts:    test.numHosts,
		}
		sc := storageRemainingScoreCalc(info, settings)
		// Check whether the score is within range 0 to 1
		if sc < 0 || sc >= 1 {
			t.Fatalf("unexpected score %v. Not within range [0, 1)", sc)
		}
		// Add the result to results
		results = append(results, scoreRecord{ratio, sc})
	}
	// Test whether the result is incrementing
	for i := 0; i != len(results)-1; i++ {
		if results[i].sc >= results[i+1].sc {
			t.Fatalf("score not incrementing. [%v: %v] -> [%v: %v]", results[i].ratio, results[i].sc, results[i+1].ratio, results[i+1].sc)
		}
	}
	for _, result := range results {
		t.Logf("At remainingStorage / expectedStorage ratio %7.4f, got score %7.4f", result.ratio, result.sc)
	}
}

// TestEvalContractCost test the functionality of evalContractCost
func TestEvalContractCost(t *testing.T) {
	tests := []struct {
		contractPrice    common.BigInt
		storagePrice     common.BigInt
		uploadPrice      common.BigInt
		downloadPrice    common.BigInt
		numHosts         uint64
		period           uint64
		expectedUpload   uint64
		expectedDownload uint64
		expectedStorage  uint64
		expectedCost     common.BigInt
	}{
		{
			contractPrice: common.NewBigInt(1),
			numHosts:      1,
			expectedCost:  common.NewBigInt(2),
		},
		{
			storagePrice:    common.NewBigInt(10),
			numHosts:        1,
			period:          10,
			expectedStorage: 5,
			expectedCost:    common.NewBigInt(1000),
		},
		{
			uploadPrice:    common.NewBigInt(100),
			numHosts:       1,
			expectedUpload: 10,
			expectedCost:   common.NewBigInt(1000),
		},
		{
			downloadPrice:    common.NewBigInt(100),
			numHosts:         1,
			expectedDownload: 10,
			expectedCost:     common.NewBigInt(1000),
		},
		{
			contractPrice:    common.NewBigInt(1),
			storagePrice:     common.NewBigInt(10),
			uploadPrice:      common.NewBigInt(100),
			downloadPrice:    common.NewBigInt(100),
			numHosts:         1,
			period:           10,
			expectedUpload:   10,
			expectedDownload: 10,
			expectedStorage:  5,
			expectedCost:     common.NewBigInt(3002),
		},
	}
	for i, test := range tests {
		info := storage.HostInfo{
			HostExtConfig: storage.HostExtConfig{
				ContractPrice:          test.contractPrice,
				StoragePrice:           test.storagePrice,
				UploadBandwidthPrice:   test.uploadPrice,
				DownloadBandwidthPrice: test.downloadPrice,
			},
		}
		rent := storage.RentPayment{
			StorageHosts:     test.numHosts,
			Period:           test.period,
			ExpectedStorage:  test.expectedStorage,
			ExpectedDownload: test.expectedDownload,
			ExpectedUpload:   test.expectedUpload,
		}
		cost := evalContractCost(info, rent)
		if cost.Cmp(test.expectedCost) != 0 {
			t.Errorf("Test %v: cost not expected. Got %v, Expect %v", i, cost, test.expectedCost)
		}
	}
}

// TestInteractionScoreCalc test the functionality of interactionScoreCalc
func TestInteractionScoreCalc(t *testing.T) {
	baseTotalFactor := float64(1000)
	tests := []struct {
		successToFailedRatio float64
	}{
		{0.01}, {0.1}, {1}, {2}, {3}, {5}, {10}, {100},
	}
	type record struct{ ratio, sc float64 }
	var results []record
	for i, test := range tests {
		successFactor := baseTotalFactor / (1 + test.successToFailedRatio) * test.successToFailedRatio
		failedFactor := baseTotalFactor / (1 + test.successToFailedRatio)
		info := storage.HostInfo{
			SuccessfulInteractionFactor: successFactor,
			FailedInteractionFactor:     failedFactor,
		}
		res := interactionScoreCalc(info)
		if res < 0 || res > 1 {
			t.Fatalf("Test %v: invalid interaction score: %v", i, res)
		}
		results = append(results, record{test.successToFailedRatio, res})
	}
	// Test whether the result is incrementing
	for i := 0; i != len(results)-1; i++ {
		if results[i].sc >= results[i+1].sc {
			t.Fatalf("score not incrementing. [%v: %v] -> [%v: %v]", results[i].ratio, results[i].sc, results[i+1].ratio, results[i+1].sc)
		}
	}
	for _, result := range results {
		t.Logf("At remainingStorage / expectedStorage ratio %7.2f, got score %7.4f", result.ratio, result.sc)
	}
}

// TestInteractionScoreCorner test the corner case for interaction score calculation
func TestInteractionScoreCorner(t *testing.T) {
	info := storage.HostInfo{}
	// The calculation shall not panic for this corner case
	res := interactionScoreCalc(info)
	expect := float64(1)
	if res != expect {
		t.Errorf("interaction score not expected. Got %v, Expect %v", res, expect)
	}
}

// TestUptimeScoreCalc test uptimeScoreCalc
func TestUptimeScoreCalc(t *testing.T) {
	baseTotalFactor := float64(1000)
	tests := []struct {
		upToDownRatio float64
	}{
		{0.01}, {0.1}, {1}, {2}, {3}, {5}, {10}, {100},
	}
	type record struct{ ratio, sc float64 }
	var results []record
	for i, test := range tests {
		upFactor := baseTotalFactor / (1 + test.upToDownRatio) * test.upToDownRatio
		downFactor := baseTotalFactor / (1 + test.upToDownRatio)
		info := storage.HostInfo{
			AccumulatedUptime:   upFactor,
			AccumulatedDowntime: downFactor,
		}
		res := uptimeScoreCalc(info)
		if res < 0 || res > 1 {
			t.Fatalf("Test %v: invalid interaction score: %v", i, res)
		}
		if test.upToDownRatio/(test.upToDownRatio+1) >= uptimeCap {
			if res != 1 {
				t.Fatalf("Test %v: uptime ratio larger than cap, score not 1: %v", i, res)
			}
		}
		results = append(results, record{test.upToDownRatio, res})
	}
	// Test whether the result is incrementing
	for i := 0; i != len(results)-1; i++ {
		if results[i].sc > results[i+1].sc {
			t.Fatalf("score not incrementing. [%v: %v] -> [%v: %v]", results[i].ratio, results[i].sc, results[i+1].ratio, results[i+1].sc)
		}
	}
	for _, result := range results {
		t.Logf("At remainingStorage / expectedStorage ratio %7.2f, got score %7.4f", result.ratio, result.sc)
	}
}

// TestUptimeScoreCalcCorner test the corner case for uptimeScoreCalc
func TestUptimeScoreCalcCorner(t *testing.T) {
	info := storage.HostInfo{}
	// The calculation shall not panic for this corner case
	res := uptimeScoreCalc(info)
	expect := float64(1)
	if res != expect {
		t.Errorf("uptime score not expected. Got %v, Expect %v", res, expect)
	}
}
