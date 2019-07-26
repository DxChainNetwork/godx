package storage

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"math/big"
	"strconv"
	"strings"
)

// CurrencyUnit defines available units used for rentPayment fund
var CurrencyUnit = []string{"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}

// TimeUnit defines available units used for period and renew
var TimeUnit = []string{"h", "b", "d", "w", "m", "y"}

// DataSizeUnit defines available units used for specifying expected storage size, expected upload size, and expected download size
var DataSizeUnit = []string{"kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}

// SpeedUnit defines available units used for specifying upload and download speed
var SpeedUnit = []string{"bps", "kbps", "mbps", "gbps", "tbps"}

var currencyIndexMap = map[string]uint64{
	"wei":        1,
	"kwei":       1e3,
	"mwei":       1e6,
	"gwei":       1e9,
	"microether": 1e12,
	"milliether": 1e15,
	"ether":      1e18,
}

var dataSizeMultiplier = map[string]uint64{
	"kb":  1e3,
	"mb":  1e6,
	"gb":  1e9,
	"tb":  1e12,
	"kib": 1 << 10,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
}

var speedMultiplier = map[string]int64{
	"bps":  1,
	"kbps": 1e3,
	"mbps": 1e6,
	"gbps": 1e9,
	"tbps": 1e12,
}

//ParseFund will parse the user string input, and convert it into common.BigInt
//type in terms of wei, which is the smallest currency unit
func ParseFund(str string) (parsed common.BigInt, err error) {

	// remove all the white spaces and convert everything into lower case
	str = formatString(str)

	// check the suffix and convert the units into wei, which is the smallest unit
	// for the eth currency type
	for unit := range currencyIndexMap {
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

// stringToBigInt will convert the string to common.BigInt type
func stringToBigInt(unit, fund string) (parsed common.BigInt, err error) {
	// from the currency indexMap, get the conversion rate
	conversionRate := currencyIndexMap[unit]
	var bigInt = new(big.Int)

	// remove the unit
	fund = strings.TrimSuffix(fund, unit)

	// check if the string contains only digit
	if !containsDigitOnly(fund) {
		err = fmt.Errorf("the fund provided is not valid, the fund must be numbers only with the valid unit, ex 100wei. Here is a list of valid currency unit: %+v", CurrencyUnit)
		return
	}

	// convert the string to *big.int
	if _, err = fmt.Sscan(fund, bigInt); err != nil {
		err = fmt.Errorf("failed to convert the string to *big.Int: %s", err.Error())
		return
	}

	// convert the result to common.BigInt
	parsed = common.PtrBigInt(bigInt)

	// unit conversion
	parsed = parsed.MultUint64(conversionRate)

	return
}

// containsDigitOnly checks if a string contains only digit
func containsDigitOnly(s string) (digitOnly bool) {
	notDigit := func(c rune) bool { return c < '0' || c > '9' }
	return strings.IndexFunc(s, notDigit) == -1
}

// ParseTime will parse the string version of period into uint64 type based on the
// unit provided. The supported units are blocks, hour, day, week, month, year
func ParseTime(str string) (parsed uint64, err error) {
	// format the string
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)
	unit := string(str[len(str)-1])

	switch {
	case unit == "h":
		// convert the time hour to number of blocks
		return ConvertUint64(str, BlockPerHour, "h")
	case unit == "b":
		// return the number of blocks directly
		return ConvertUint64(str, 1, "b")
	case unit == "d":
		// convert the time day to number of blocks
		return ConvertUint64(str, BlocksPerDay, "d")

	case unit == "w":
		// convert the time week to number of blocks
		return ConvertUint64(str, BlocksPerWeek, "w")

	case unit == "m":
		// convert the time month to number of blocks
		return ConvertUint64(str, BlocksPerMonth, "m")

	case unit == "y":
		// convert the time year to number of blocks
		return ConvertUint64(str, BlocksPerYear, "y")
	default:
		err = fmt.Errorf("valid unit must be provided: %v", TimeUnit)
		return
	}
}

// ConvertUint64 will convert data to the uint64 format
func ConvertUint64(data string, factor uint64, unit string) (parsed uint64, err error) {

	// remove the unit from the string
	data = strings.TrimSuffix(data, unit)

	if parsed, err = strconv.ParseUint(data, 10, 64); err != nil {
		err = fmt.Errorf("error parsing to uint64: %s", err.Error())
		return
	}

	parsed *= factor
	return
}

// ParseStorage will parse the string into uint64
func ParseStorage(str string) (parsed uint64, err error) {
	return DataSizeConverter(str)
}

// DataSizeConverter will convert the string with the unit into uint64 in the unit of byte
func DataSizeConverter(str string) (parsed uint64, err error) {
	// string format
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)

	// convert the data size into bytes
	for unit, multiplier := range dataSizeMultiplier {
		if strings.HasSuffix(str, unit) {
			return ConvertUint64(str, multiplier, unit)
		}
	}

	if strings.HasSuffix(str, "b") {
		return ConvertUint64(str, 1, "b")
	}

	err = fmt.Errorf("data provided does not have valid unit: %s. valid units are: %v",
		str, DataSizeUnit)
	return
}

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

// ParseSpeed will parse the string into maxUploadSpeed or maxDownloadSpeed which will be used
// to limit the speed while uploading/downloading data
func ParseSpeed(str string) (parsed int64, err error) {
	// remove whitespace and convert all to lower case
	str = formatString(str)

	// loop through the speedMultiplier
	for unit := range speedMultiplier {
		// unit bps is ignored due to each unit contains bps as suffix
		if unit == "bps" {
			continue
		}
		if strings.HasSuffix(str, unit) {
			return speedConvert(str, unit)
		}
	}

	// check the bps unit
	if strings.HasSuffix(str, "bps") {
		return speedConvert(str, "bps")
	}

	// there is no qualified unit
	err = fmt.Errorf("failed to parse the upload/download speed, unit must be included. Here is a list of available units: %s", SpeedUnit)
	return
}

// speedConvert will convert the upload and download speed to bps, in form of int64
func speedConvert(speed string, unit string) (parsed int64, err error) {
	// remove the suffix
	speed = strings.TrimSuffix(speed, unit)

	// parse the string into int64 format
	if parsed, err = strconv.ParseInt(speed, 10, 64); err != nil {
		err = fmt.Errorf("failed to parse the speed provided, unit must be included. Here is a list of available units: %s", SpeedUnit)
		return
	}

	// convert the the result into bps format
	parsed *= speedMultiplier[unit]

	return
}

// formatString will remove all spaces from the string and set the entire string into lower case
func formatString(s string) (formatted string) {
	s = strings.Replace(s, " ", "", -1)
	s = strings.ToLower(s)
	return s
}
