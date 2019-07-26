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
var CurrencyUnit = []string{"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}

var CurrencyIndexMap = map[string]uint64{
	"wei":        1,
	"kwei":       1e3,
	"mwei":       1e6,
	"gwei":       1e9,
	"microether": 1e12,
	"milliether": 1e15,
	"ether":      1e18,
}

// ParseCurrency will parse the user string input, and convert it into common.BigInt
// type in terms of wei, which is the smallest currency unit
func ParseCurrency(str string) (parsed common.BigInt, err error) {
	// remove all the white spaces and convert everything into lower case
	str = formatString(str)

	// check the suffix and convert the units into wei, which is the smallest unit
	// for the eth currency type
	for unit := range CurrencyIndexMap {
		// skip wei or ether because other currency unit also
		// includes these kind of suffix, such as milliether and
		// kwei
		if unit == "wei" || unit == "ether" {
			continue
		}

		// check if the string contains the suffix and convert
		// the result into bigInt
		if strings.HasSuffix(str, unit) {
			return stringToBigInt(unit, str)
		}
	}

	// check if the suffix contains wei
	if strings.HasSuffix(str, "wei") {
		return stringToBigInt("wei", str)
	}

	// check if the suffix contains ether
	if strings.HasSuffix(str, "ether") {
		return stringToBigInt("ether", str)
	}

	// otherwise, return error
	err = fmt.Errorf("the provided currency unit is invalid. Here is a list of valid currency unit: %+v", CurrencyUnit)
	return
}

// FormatCurrency is used to format the currency for displaying purpose
func FormatCurrency(fund common.BigInt) (formatted string) {
	switch {
	case fund.DivNoRemaining(CurrencyIndexMap["ether"]):
		formatted = fmt.Sprintf("%v ether", fund.DivUint64(CurrencyIndexMap["ether"]))
		return
	case fund.DivNoRemaining(CurrencyIndexMap["milliether"]):
		formatted = fmt.Sprintf("%v milliether", fund.DivUint64(CurrencyIndexMap["milliether"]))
		return
	case fund.DivNoRemaining(CurrencyIndexMap["microether"]):
		formatted = fmt.Sprintf("%v microether", fund.DivUint64(CurrencyIndexMap["microether"]))
		return
	case fund.DivNoRemaining(CurrencyIndexMap["gwei"]):
		formatted = fmt.Sprintf("%v Gwei", fund.DivUint64(CurrencyIndexMap["gwei"]))
		return
	case fund.DivNoRemaining(CurrencyIndexMap["mwei"]):
		formatted = fmt.Sprintf("%v Mwei", fund.DivUint64(CurrencyIndexMap["mwei"]))
		return
	case fund.DivNoRemaining(CurrencyIndexMap["kwei"]):
		formatted = fmt.Sprintf("%v Kwei", fund.DivUint64(CurrencyIndexMap["kwei"]))
		return
	default:
		formatted = fmt.Sprintf("%v wei", fund)
		return
	}
}
