// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package unit

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"strings"
)

// CurrencyUnit defines available units used for rentPayment fund
var CurrencyUnit = []string{"hump", "ghump", "dx"}

var CurrencyIndexMap = map[string]uint64{
	"hump":  1,
	"ghump": 1e9,
	"dx":    1e18,
}

// ParseCurrency will parse the user string input, and convert it into common.BigInt
// type in terms of wei, which is the smallest currency unit
func ParseCurrency(str string) (parsed common.BigInt, err error) {
	// remove all the white spaces and convert everything into lower case
	str = formatString(str)

	// check the suffix and convert the units into wei, which is the smallest unit
	// for the eth currency type
	for unit := range CurrencyIndexMap {
		// skip ether or dx because other currency unit also
		// includes these kind of suffix
		if unit == "hump" || unit == "dx" {
			continue
		}

		// check if the string contains the suffix and convert
		// the result into bigInt
		if strings.HasSuffix(str, unit) {
			return stringToBigInt(unit, str)
		}
	}

	// check if the suffix contains wei
	if strings.HasSuffix(str, "hump") {
		return stringToBigInt("hump", str)
	}

	// check if the suffix contains ether
	if strings.HasSuffix(str, "dx") {
		return stringToBigInt("dx", str)
	}

	// otherwise, return error
	err = fmt.Errorf("the provided currency unit is invalid. Here is a list of valid currency unit: %+v", CurrencyUnit)
	return
}

// FormatCurrency is used to format the currency for displaying purpose. The extra string will append
// to the unit
func FormatCurrency(fund common.BigInt, extra ...string) (formatted string) {
	var extraStr string
	if len(extra) > 0 {
		extraStr = strings.Join(extra, "")
	}

	if fund.IsEqual(common.BigInt0) {
		formatted = fmt.Sprintf("%v hump%v", fund, extraStr)
		return
	}

	// pick up the most suitable unit
	if value := fund.DivWithFloatResultUint64(CurrencyIndexMap["dx"]); value > 0.001 {
		formatted = fmt.Sprintf("%v dx%v", value, extraStr)
	} else if value := fund.DivWithFloatResultUint64(CurrencyIndexMap["ghump"]); value > 0.001 {
		formatted = fmt.Sprintf("%v Ghump%v", value, extraStr)
	} else {
		formatted = fmt.Sprintf("%v hump%v", fund, extraStr)
	}

	return
}
