// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import (
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"math/big"
)

// HostEvaluation defines an interface that include methods that used to calculate
// the storage host evaluation and evaluation details
type HostEvaluation interface {
	EvaluationDetail(eval common.BigInt, ignoreAge, ignoreUptime bool) EvaluationDetail
	Evaluation() common.BigInt
}

// EvaluationFunc is used to calculate storage host evaluation
type EvaluationFunc func(storage.HostInfo) HostEvaluation

// EvaluationDetail contains the detailed storage host evaluation factors
type EvaluationDetail struct {
	Evaluation     common.BigInt `json:"evaluation"`
	ConversionRate float64       `json:"conversionrate"`

	PresenceFactor         float64 `json:"presencefactor"`
	DepositFactor          float64 `json:"depositfactor"`
	InteractionFactor      float64 `json:"interactionfactor"`
	ContractPriceFactor    float64 `json:"contractpriceFactor"`
	StorageRemainingFactor float64 `json:"storageremainingfactor"`
	UptimeFactor           float64 `json:"uptimefactor"`
}

// EvaluationCriteria contains statistics that used to calculate the storage host evaluation
type EvaluationCriteria struct {
	PresenceFactor         float64
	DepositFactor          float64
	InteractionFactor      float64
	ContractPriceFactor    float64
	StorageRemainingFactor float64
	UptimeFactor           float64
}

// Evaluation will be used to calculate the storage host evaluation
func (ec EvaluationCriteria) Evaluation() common.BigInt {
	total := ec.PresenceFactor * ec.DepositFactor * ec.InteractionFactor *
		ec.ContractPriceFactor * ec.StorageRemainingFactor * ec.UptimeFactor

	// making sure the total is at least 1
	if total < 1 {
		total = 1
	}

	return common.NewBigInt(1).MultFloat64(total)
}

// EvaluationDetail will return storage host detailed evaluation, including evaluation criteria
func (ec EvaluationCriteria) EvaluationDetail(evalAll common.BigInt, ignoreAge, ignoreUptime bool) EvaluationDetail {
	if ignoreAge {
		ec.PresenceFactor = 1
	}
	if ignoreUptime {
		ec.UptimeFactor = 1
	}

	eval := ec.Evaluation()

	ratio := conversionRate(eval, evalAll)

	return EvaluationDetail{
		Evaluation:             eval,
		ConversionRate:         ratio,
		PresenceFactor:         ec.PresenceFactor,
		DepositFactor:          ec.DepositFactor,
		InteractionFactor:      ec.InteractionFactor,
		ContractPriceFactor:    ec.ContractPriceFactor,
		StorageRemainingFactor: ec.StorageRemainingFactor,
		UptimeFactor:           ec.UptimeFactor,
	}

}

// conversionRate calculate the rate of evalAll / (eval * 50)
func conversionRate(eval, evalAll common.BigInt) float64 {
	// eliminate 0 for denominator
	if evalAll.Cmp(common.NewBigInt(0)) <= 0 {
		evalAll = common.NewBigInt(1)
	}

	// evaluation increment
	eval = eval.MultInt(50)

	// return ratio
	//return eval.Div(evalAll).Float64()
	rate, _ := big.NewRat(0, 1).SetFrac(eval.BigIntPtr(), evalAll.BigIntPtr()).Float64()
	return rate
}
