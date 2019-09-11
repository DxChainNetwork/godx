// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package params

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/math"
)

var (
	DefaultValidators = map[common.Address]ValidatorConfig{
		common.HexToAddress("0x60c8947134be7c0604a866a0462542eb0dcf71f9"): {
			Deposit:     common.NewBigIntUint64(100000),
			RewardRatio: 80,
		},

		common.HexToAddress("0x58a366c3c1a735bf3d09f2a48a014a8ebc64457c"): {
			Deposit:     common.NewBigIntUint64(200000),
			RewardRatio: 70,
		},

		common.HexToAddress("0x801ee9587ea0d52fe477755a3e91d7244e6556a3"): {
			Deposit:     common.NewBigIntUint64(300000),
			RewardRatio: 85,
		},

		common.HexToAddress("0xcde55147efd18f79774676d5a8674d94d00b4c9a"): {
			Deposit:     common.NewBigIntUint64(400000),
			RewardRatio: 75,
		},

		common.HexToAddress("0x31de5dbe50885d9632935dec507f806baf1027c0"): {
			Deposit:     common.NewBigIntUint64(500000),
			RewardRatio: 100,
		},
	}
)

// DposConfig is the consensus engine configs for delegated proof-of-stake based sealing.
type DposConfig struct {
	//Validators []common.Address `json:"validators"` // Genesis validator list
	Validators map[common.Address]ValidatorConfig `json:"validators"` // Genesis validator list
}

type ValidatorConfig struct {
	Deposit     common.BigInt `json:"deposit" gencodec:"required"`
	RewardRatio uint64        `json:"rewardRatio"`
}

func DefaultDposConfig() *DposConfig {
	return &DposConfig{
		Validators: DefaultValidators,
	}
}

func (d *DposConfig) ParseValidators() (validators []common.Address) {
	for validator, _ := range d.Validators {
		validators = append(validators, validator)
	}
	return
}

// String implements the stringer interface, returning the consensus engine details.
func (d *DposConfig) String() string {
	return "dpos"
}

func (vc ValidatorConfig) MarshalJSON() ([]byte, error) {
	type ValidatorConfig struct {
		Deposit     *math.HexOrDecimal256 `json:"deposit" gencodec:"required"`
		RewardRatio uint64                `json:"rewardRatio"`
	}
	var enc ValidatorConfig
	enc.Deposit = (*math.HexOrDecimal256)(vc.Deposit.BigIntPtr())
	enc.RewardRatio = vc.RewardRatio
	return json.Marshal(&enc)
}

func (vc *ValidatorConfig) UnmarshalJSON(input []byte) error {
	type ValidatorConfig struct {
		Deposit     *math.HexOrDecimal256 `json:"deposit" gencodec:"required"`
		RewardRatio uint64                `json:"rewardRatio"`
	}
	var devc ValidatorConfig
	if err := json.Unmarshal(input, &devc); err != nil {
		return err
	}

	// assign values to ValidatorConfig
	if devc.Deposit == nil {
		return errors.New("missing required field 'deposit' for ValidatorConfig")
	}
	deposit := (*big.Int)(devc.Deposit)
	vc.Deposit = common.PtrBigInt(deposit)
	vc.RewardRatio = devc.RewardRatio
	return nil
}
