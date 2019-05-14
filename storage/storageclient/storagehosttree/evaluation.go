// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

type HostEvaluation interface {
	EvaluationDetail(eval common.BigInt, ignoreAge, ignoreUptime bool) EvaluationDetail
	Evaluation() common.BigInt
}

type EvaluationFunc func(storage.HostInfo) HostEvaluation

type EvaluationDetail struct {
	Evaluation     common.BigInt `json:"evaluation"`
	ConversionRate float64       `json:"conversionrate"`

	AgeAdjustment              float64 `json:"agefactor"`
	BurnAdjustment             float64 `json:"burnfactor"`
	DepositAdjustment          float64 `json:"depositfactor"`
	InteractionAdjustment      float64 `json:"interactionfactor"`
	PriceAdjustment            float64 `json:"pricefactor"`
	StorageRemainingAdjustment float64 `json:"remainingstoragefactor"`
	UptimeAdjustment           float64 `json:"uptimefactor"`
}

type EvaluationCriteria struct {
	AgeAdjustment              float64
	BurnAdjustment             float64
	DepositAdjustment          float64
	InteractionAdjustment      float64
	PriceAdjustment            float64
	StorageRemainingAdjustment float64
	UptimeAdjustment           float64
}

func (ec EvaluationCriteria) Evaluation() common.BigInt {
	total := ec.AgeAdjustment * ec.BurnAdjustment * ec.DepositAdjustment * ec.InteractionAdjustment *
		ec.PriceAdjustment * ec.StorageRemainingAdjustment * ec.UptimeAdjustment

	return common.NewBigInt(1).MultFloat64(total)
}

func (ec EvaluationCriteria) EvaluationDetail(evalAll common.BigInt, ignoreAge, ignoreUptime bool) EvaluationDetail {
	if ignoreAge {
		ec.AgeAdjustment = 1
	}
	if ignoreUptime {
		ec.UptimeAdjustment = 1
	}

	eval := ec.Evaluation()

	ratio := conversionRate(eval, evalAll)

	return EvaluationDetail{
		Evaluation:                 eval,
		ConversionRate:             ratio,
		AgeAdjustment:              ec.AgeAdjustment,
		BurnAdjustment:             ec.BurnAdjustment,
		DepositAdjustment:          ec.DepositAdjustment,
		InteractionAdjustment:      ec.InteractionAdjustment,
		PriceAdjustment:            ec.PriceAdjustment,
		StorageRemainingAdjustment: ec.StorageRemainingAdjustment,
		UptimeAdjustment:           ec.UptimeAdjustment,
	}

}

func conversionRate(eval, evalAll common.BigInt) float64 {
	// eliminate 0 for denominator
	if evalAll.Cmp(common.NewBigInt(0)) <= 0 {
		evalAll = common.NewBigInt(1)
	}

	// evaluation increment
	eval = eval.MultInt(50)

	// return ratio
	return eval.Div(evalAll).Float64()
}
