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
			Address:     common.HexToAddress("0xccdfb5a54db1d805ca24a33b8b15f49d8945bb4b"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x116204ed3e5749ab6c1318314300dbabf5aa972b"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xa60e0361cffc636da87ea8cb246e7926870621c9"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xf6a0da1f0b9a8d4e6b4392fe60a5e1f99c6aa873"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x08edc1328ba5236b151d273f7ad4703c1585def1"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x11720ac932723d5df4221dad1420ea6695acc68a"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x4a83c1c000ac1d1a8c06e8d18dc641c8530e0625"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x9374268a703851b2302b35d4de62c2f498099514"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x7d038901709e9d8cdb040e52e54c493bc726a05d"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x944eb166879a0b9a5e3c7a0512836a4d22a0bb47"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xf7925a26ebf873cea35fbe2f8278a8cc94fad801"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x31423da09afc3202844131da5888f9acd5593c6f"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x8fd7c0503e27b35ee6ef79c21559b4195536b780"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x9c0d5b2713ebebfa9ec0819186809cbd510554e8"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x18cead672df01dbd808fcc7e0e988bdc67551de5"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x2f03b5b4416e7ce86773f1908939291787cb8086"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xe61aa6815e667f1dff7d257ad2c1f30d02bcf3da"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xdfb1cca8d7299b0210086745dc7dad14a03ce2d3"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xc8c85cae16d18076741e2f94528d6ffaa5960fa1"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0xbbb1244fd311481e68253f9995a02715ac22ece6"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
		{
			Address:     common.HexToAddress("0x816f4378aca62e72d75ead38495bb896f72eefce"),
			Deposit:     common.NewBigIntUint64(1e18).MultInt64(10000),
			RewardRatio: 30,
		},
	}

	// TODO: specify the real donated account address
	// DefaultDonatedAccount is the address that receive some donation when producing a new block
	DefaultDonatedAccount = common.HexToAddress("0xabc")

	// DefaultSpinnerTime is the default time to use lucky spinner
	DefaultSpinnerTime = int64(1584489598)

	// DefaultDip8BlockNumber the default block number to tigger dip7, 8 hard fork
	DefaultDip8BlockNumber = int64(2000000)
)

// DposConfig is the consensus engine configs for delegated proof-of-stake based sealing.
type DposConfig struct {
	//Validators []common.Address `json:"validators"` // Genesis validator list
	Validators      []ValidatorConfig `json:"validators"`     // Genesis validator list
	DonatedAccount  common.Address    `json:"donatedAccount"` // address for receiving donation
	SFSpinnerTime   *int64            `json:"sfSpinnerTime, omitempty"`
	Dip8BlockNumber *int64            `json:"dip8BlockNumber, omitempty"`
}

type ValidatorConfig struct {
	Address     common.Address `json:"address" gencodec:"required"`
	Deposit     common.BigInt  `json:"deposit" gencodec:"required"`
	RewardRatio uint64         `json:"rewardRatio"`
}

func DefaultDposConfig() *DposConfig {
	return &DposConfig{
		Validators:      DefaultValidators,
		DonatedAccount:  DefaultDonatedAccount,
		SFSpinnerTime:   &DefaultSpinnerTime,
		Dip8BlockNumber: &DefaultDip8BlockNumber,
	}
}

func (d *DposConfig) ParseValidators() (validators []common.Address) {
	for _, validator := range d.Validators {
		validators = append(validators, validator.Address)
	}
	return
}

// IsLuckySpinner decides whether we should use lucky spinner or not
func (d *DposConfig) IsLuckySpinner(blockTime int64) bool {
	if d.SFSpinnerTime == nil || blockTime >= *d.SFSpinnerTime {
		return true
	}
	return false
}

// IsDip8 indicates whether we should hard fork to dip7 and dip8
func (d *DposConfig) IsDip8(blockNumber int64) bool {
	if d.Dip8BlockNumber == nil || blockNumber >= *d.Dip8BlockNumber {
		return true
	}
	return false
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
