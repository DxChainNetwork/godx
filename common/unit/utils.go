// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package unit

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"strconv"
	"strings"
)

// ParseBool will parse the string into boolean.
func ParseBool(str string) (parsed bool, err error) {
	// format the string
	str = formatString(str)

	// convert the string into boolean
	switch {
	case str == "true":
		return true, nil
	case str == "false":
		return false, nil
	default:
		err = fmt.Errorf("failed to convert the string %s into boolean", str)
		return
	}
}

// FormatBool format a bool value to string
func FormatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// ParseUint64 will convert data to the uint64 format
func ParseUint64(data string, factor uint64, unit string) (parsed uint64, err error) {
	// remove the unit from the string
	data = strings.TrimSuffix(data, unit)

	if parsed, err = strconv.ParseUint(data, 10, 64); err != nil {
		err = fmt.Errorf("error parsing to uint64: %s", err.Error())
		return
	}

	parsed *= factor
	return
}

// formatString will remove all spaces from the string and set the entire string into lower case
func formatString(s string) (formatted string) {
	s = strings.Replace(s, " ", "", -1)
	s = strings.ToLower(s)
	return s
}

// stringToBigInt will convert the string to common.BigInt type
func stringToBigInt(unit, fund string) (parsed common.BigInt, err error) {
	// from the currency indexMap, get the conversion rate
	conversionRate := currencyIndexMap[unit]
	var bigFloat = new(big.Float)
	var bigInt = new(big.Int)

	// remove the unit
	fund = strings.TrimSuffix(fund, unit)

	// convert the string to *big.int
	if _, err = fmt.Sscan(fund, bigFloat); err != nil {
		err = fmt.Errorf("failed to convert the string to *big.Int: %s", err.Error())
		return
	}

	parsedFloat := new(big.Float).Mul(bigFloat, new(big.Float).SetUint64(conversionRate))

	parsedFloat.Int(bigInt)

	// convert the result to common.BigInt
	parsed = common.PtrBigInt(bigInt)

	return
}
