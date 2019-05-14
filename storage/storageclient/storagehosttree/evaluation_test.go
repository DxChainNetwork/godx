// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/DxChainNetwork/godx/common"
)

func init() {
	seed := time.Now().UTC().UnixNano()
	rand.Seed(seed)
}

func TestConversionRate(t *testing.T) {
	tables := []struct {
		eval    int64
		evalAll int64
		result  float64
	}{
		{50, 10000, 50 * 50 / 10000},
		{7, 1001, 7 * 50 / 1001},
		{5, 0, 5 * 50 / 1},
		{0, 10000, 0},
	}

	for _, table := range tables {
		val := conversionRate(common.NewBigInt(table.eval), common.NewBigInt(table.evalAll))
		if val != table.result {
			t.Errorf("error calculating conversion rate: inputs %v and %v. Expected %f, got %f",
				table.eval, table.evalAll, table.result, val)
		}
	}
}

func TestEvaluationCriteria_EvaluationDetail(t *testing.T) {
	ec := randomCriteria()
	ed := ec.EvaluationDetail(common.NewBigInt(1000), true, true)
	if ed.AgeAdjustment != 1 {
		t.Errorf("age adjustment is expected to be 1, instead got %v", ed.AgeAdjustment)
	}

	if ed.UptimeAdjustment != 1 {
		t.Errorf("uptime adjustment is expected to be 1, isntead got %v", ed.UptimeAdjustment)
	}

	if ed.BurnAdjustment != ec.BurnAdjustment {
		t.Errorf("burn adjustment is expected to be %v, got %v", ec.BurnAdjustment, ed.BurnAdjustment)
	}
}

func randomCriteria() EvaluationCriteria {
	return EvaluationCriteria{
		AgeAdjustment:              randFloat64(),
		BurnAdjustment:             randFloat64(),
		DepositAdjustment:          randFloat64(),
		InteractionAdjustment:      randFloat64(),
		PriceAdjustment:            randFloat64(),
		StorageRemainingAdjustment: randFloat64(),
		UptimeAdjustment:           randFloat64(),
	}
}

func partialRandomCriteria() EvaluationCriteria {

	return EvaluationCriteria{
		AgeAdjustment:              1,
		BurnAdjustment:             randFloat64(),
		DepositAdjustment:          randFloat64(),
		InteractionAdjustment:      randFloat64(),
		PriceAdjustment:            100,
		StorageRemainingAdjustment: randFloat64(),
		UptimeAdjustment:           randFloat64(),
	}
}

func randFloat64() float64 {
	return math.Abs(rand.Float64() * 5000)
}
