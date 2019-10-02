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
	DefaultValidators = []ValidatorConfig{
		{
			Address:     common.HexToAddress("0xadee32fbde84bf5074d692bc68fc4b94d5e42f63"),
			Deposit:     common.NewBigIntUint64(100000),
			RewardRatio: 80,
		},
		{
			Address:     common.HexToAddress("0xdd8808c715d360bbb92b125baeb655be9bf05c22"),
			Deposit:     common.NewBigIntUint64(200000),
			RewardRatio: 70,
		},
		{
			Address:     common.HexToAddress("0x5286ca348e924330dbea9b388303458092a600d7"),
			Deposit:     common.NewBigIntUint64(300000),
			RewardRatio: 85,
		},
	}
)

// DposConfig is the consensus engine configs for delegated proof-of-stake based sealing.
type DposConfig struct {
	//Validators []common.Address `json:"validators"` // Genesis validator list
	Validators []ValidatorConfig `json:"validators"` // Genesis validator list
}

type ValidatorConfig struct {
	Address     common.Address `json:"address" gencodec:"required"`
	Deposit     common.BigInt  `json:"deposit" gencodec:"required"`
	RewardRatio uint64         `json:"rewardRatio"`
}

func DefaultDposConfig() *DposConfig {
	return &DposConfig{
		Validators: DefaultValidators,
	}
}

func (d *DposConfig) ParseValidators() (validators []common.Address) {
	for _, validator := range d.Validators {
		validators = append(validators, validator.Address)
	}
	return
}

// String implements the stringer interface, returning the consensus engine details.
func (d *DposConfig) String() string {
	return "dpos"
}

func (vc ValidatorConfig) MarshalJSON() ([]byte, error) {
	type ValidatorConfig struct {
		Address     common.Address        `json:"address" gencodec:"required"`
		Deposit     *math.HexOrDecimal256 `json:"deposit" gencodec:"required"`
		RewardRatio uint64                `json:"rewardRatio"`
	}
	var enc ValidatorConfig
	enc.Address = vc.Address
	enc.Deposit = (*math.HexOrDecimal256)(vc.Deposit.BigIntPtr())
	enc.RewardRatio = vc.RewardRatio
	return json.Marshal(&enc)
}

func (vc *ValidatorConfig) UnmarshalJSON(input []byte) error {
	type ValidatorConfig struct {
		Address     common.Address        `json:"address" gencodec:"required"`
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
	vc.Address = devc.Address
	return nil
}
