// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"testing"
)

func TestContainsDigitOnly(t *testing.T) {
	tables := []struct {
		input     string
		allDigits bool
	}{
		{"1231232131232131231231321313", true},
		{"fjsdlkfjalkfjlkahdfklhfiwehflsdjfkjf", false},
		{"21rfewf2rewf4r4", false},
		{"fdafasdf", false},
	}

	for _, table := range tables {
		result := containsDigitOnly(table.input)
		if result != table.allDigits {
			t.Errorf("the string %s is expected allcontaindigit %t, got %t",
				table.input, table.allDigits, result)
		}
	}
}

func TestParseFund(t *testing.T) {
	tables := []struct {
		fund   string
		result common.BigInt
	}{
		{"100 h", common.NewBigInt(100)},
		{"100 DX", fundUnitConversion(currencyIndexMap["dx"], big.NewInt(100))},
		{"100 GDX", fundUnitConversion(currencyIndexMap["Gdx"], big.NewInt(100))},
		{"100  nDx", fundUnitConversion(currencyIndexMap["ndx"], big.NewInt(100))},
		{"99876Kdx", fundUnitConversion(currencyIndexMap["Kdx"], big.NewInt(99876))},
	}

	for _, table := range tables {

		// parse the fund
		parsed, err := parseFund(table.fund)
		if err != nil {
			t.Fatalf("failed to parse the fund %s: %s", table.fund, err.Error())
		}

		// parsed result verification
		if !parsed.IsEqual(table.result) {
			t.Errorf("the parsed value does not match with the expected result: expected %+v, got %+v",
				table.result, parsed)
		}
	}
}

func TestParseFundFail(t *testing.T) {
	var failedExpected = []string{
		"100dxc",   // does not match with any unit
		"100ldx",   // error, even the suffix is dx
		"120lgdx",  // error, even the suffix is dx
		"a1200gdx", // error, even the suffix is dx
		"120gdx",   // lower case g, does not match with any unit
		"120kdx",   // lower case k, does not match with any unit
	}

	for _, failedCase := range failedExpected {
		if _, err := parseFund(failedCase); err == nil {
			t.Errorf("error is expected with the input %s", failedCase)
		}

	}
}
