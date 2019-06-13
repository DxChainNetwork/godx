// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"math/big"
	"strconv"
	"strings"
)

// currencyUnit defines available units used for rentPayment fund
var currencyUnit = []string{"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}

// timeUnit defines available units used for period and renew
var timeUnit = []string{"h", "b", "d", "w", "m", "y"}

// dataSizeUnit defines available units used for specifying expected storage size, expected upload size, and expected download size
var dataSizeUnit = []string{"kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}

// speedUint defines available units used for specifying upload and download speed
var speedUnit = []string{"bps", "kbps", "mbps", "gbps", "tbps"}

// parseClientSetting will take client settings in a map format, where both key and value are strings. Then, those value will be parsed
// and transfer them to storage.ClientSetting data structure
func parseClientSetting(settings map[string]string, prevSetting storage.ClientSetting) (clientSetting storage.ClientSetting, err error) {
	// get the previous settings
	clientSetting = prevSetting

	// parse the ClientSettingAPIDisplay
	for key, value := range settings {
		switch {
		case key == "fund":
			var fund common.BigInt
			fund, err = parseFund(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the fund value: %s", err.Error())
				break
			}
			clientSetting.RentPayment.Fund = fund

		case key == "hosts":
			var hosts uint64
			hosts, err = parseStorageHosts(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the hosts value: %s", err.Error())
				break
			}
			clientSetting.RentPayment.StorageHosts = hosts

		case key == "period":
			var period uint64
			period, err = parsePeriodAndRenew(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the period value: %s", err.Error())
				break
			}
			clientSetting.RentPayment.Period = period

		case key == "renew":
			var renew uint64
			renew, err = parsePeriodAndRenew(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the renew value: %s", err.Error())
				break
			}
			clientSetting.RentPayment.RenewWindow = renew

		case key == "storage":
			var expectedStorage uint64
			expectedStorage, err = parseExpectedStorage(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the expected storage: %s", err.Error())
				break
			}
			clientSetting.RentPayment.ExpectedStorage = expectedStorage

		case key == "upload":
			var expectedUpload uint64
			expectedUpload, err = parseExpectedUpload(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the expected upload: %s", err.Error())
				break
			}
			clientSetting.RentPayment.ExpectedUpload = expectedUpload

		case key == "download":
			var expectedDownload uint64
			expectedDownload, err = parseExpectedDownload(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the expected download: %s", err.Error())
				break
			}
			clientSetting.RentPayment.ExpectedDownload = expectedDownload

		case key == "redundancy":
			var redundancy float64
			redundancy, err = parseExpectedRedundancy(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the redundancy: %s", err.Error())
				break
			}
			clientSetting.RentPayment.ExpectedRedundancy = redundancy

		case key == "violation":
			var status bool
			status, err = parseEnableIPViolation(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the ip violation: %s", err.Error())
				break
			}
			clientSetting.EnableIPViolation = status

		case key == "uploadspeed":
			var uploadSpeed int64
			uploadSpeed, err = parseSpeed(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the uplaod speed: %s", err.Error())
				break
			}
			clientSetting.MaxUploadSpeed = uploadSpeed

		case key == "downloadspeed":
			var downloadSpeed int64
			downloadSpeed, err = parseSpeed(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the download speed: %s", err.Error())
				break
			}
			clientSetting.MaxDownloadSpeed = downloadSpeed

		default:
			err = fmt.Errorf("the key entered: %s is not valid. Here is a list of available keys: %+v",
				key, keys)
			break
		}

		// if got error in the switch case, break the loop directly
		if err != nil {
			break
		}
	}

	return
}

//parseFund will parse the user string input, and convert it into common.BigInt
//type in terms of wei, which is the smallest currency unit
func parseFund(fund string) (parsed common.BigInt, err error) {

	// remove all the white spaces and convert everything into lower case
	fund = formatString(fund)

	// check the suffix and convert the units into wei, which is the smallest unit
	// for the eth currency type
	for unit, _ := range currencyIndexMap {
		// skip wei or ether because other currency unit also
		// includes these kind of suffix, such as milliether and
		// kwei
		if unit == "wei" || unit == "ether" {
			continue
		}

		// check if the string contains the suffix and convert
		// the result into bigInt
		if strings.HasSuffix(fund, unit) {
			return stringToBigInt(unit, fund)
		}
	}

	// check if the suffix contains wei
	if strings.HasSuffix(fund, "wei") {
		return stringToBigInt("wei", fund)
	}

	// check if the suffix contains ether
	if strings.HasSuffix(fund, "ether") {
		return stringToBigInt("ether", fund)
	}

	// otherwise, return error
	err = fmt.Errorf("the provided currency unit is invalid. Here is a list of valid currency unit: %+v", currencyUnit)
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
		err = fmt.Errorf("the fund provided is not valid, the fund must be numbers only with the valid unit, ex 100wei. Here is a list of valid currency unit: %+v", currencyUnit)
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

// parseStorageHosts will parse the string version of storage hosts into uint64 type
func parseStorageHosts(hosts string) (parsed uint64, err error) {
	return convertUint64(hosts, 1, "")
}

// parsePeriodAndRenew will parse the string version of period into uint64 type based on the
// unit provided. The supported units are blocks, hour, day, week, month, year
func parsePeriodAndRenew(periodRenew string) (parsed uint64, err error) {
	// format the string
	periodRenew = strings.Replace(periodRenew, " ", "", -1)
	periodRenew = strings.ToLower(periodRenew)
	unit := string(periodRenew[len(periodRenew)-1])

	switch {
	case unit == "h":
		// convert the time hour to number of blocks
		return convertUint64(periodRenew, storage.BlockPerHour, "h")
	case unit == "b":
		// return the number of blocks directly
		return convertUint64(periodRenew, 1, "b")
	case unit == "d":
		// convert the time day to number of blocks
		return convertUint64(periodRenew, storage.BlocksPerDay, "d")

	case unit == "w":
		// convert the time week to number of blocks
		return convertUint64(periodRenew, storage.BlocksPerWeek, "w")

	case unit == "m":
		// convert the time month to number of blocks
		return convertUint64(periodRenew, storage.BlocksPerMonth, "m")

	case unit == "y":
		// convert the time year to number of blocks
		return convertUint64(periodRenew, storage.BlocksPerYear, "y")
	default:
		err = fmt.Errorf("valid unit must be provided: %v", timeUnit)
		return
	}
}

// convertUint64 will convert data to the uint64 format
func convertUint64(data string, factor uint64, unit string) (parsed uint64, err error) {

	// remove the unit from the string
	data = strings.TrimSuffix(data, unit)

	if parsed, err = strconv.ParseUint(data, 10, 64); err != nil {
		err = fmt.Errorf("error parsing to uint64: %s", err.Error())
		return
	}

	parsed *= factor
	return
}

// parseExpectedStorage will parse the string into uint64
func parseExpectedStorage(storage string) (parsed uint64, err error) {
	return dataSizeConverter(storage)
}

// dataSizeConverter will convert the string with the unit into uint64 in the unit of byte
func dataSizeConverter(dataSize string) (parsed uint64, err error) {
	// string format
	dataSize = strings.Replace(dataSize, " ", "", -1)
	dataSize = strings.ToLower(dataSize)

	// convert the data size into bytes
	for unit, multiplier := range dataSizeMultiplier {
		if strings.HasSuffix(dataSize, unit) {
			return convertUint64(dataSize, multiplier, unit)
		}
	}

	if strings.HasSuffix(dataSize, "b") {
		return convertUint64(dataSize, 1, "b")
	}

	err = fmt.Errorf("data provided does not have valid unit: %s. valid units are: %v",
		dataSize, dataSizeUnit)
	return
}

// parseExpectedUpload will parse the string into the form of rentPayment.ExpectedUpload
func parseExpectedUpload(upload string) (parsed uint64, err error) {
	if parsed, err = dataSizeConverter(upload); err != nil {
		return
	}

	// in terms of bytes / month
	parsed = parsed / storage.BlocksPerMonth
	return
}

// parseExpectedDownload will parse the string into the form of rentPayment.ExpectedDownload
func parseExpectedDownload(download string) (parsed uint64, err error) {
	if parsed, err = dataSizeConverter(download); err != nil {
		return
	}

	// in terms of bytes / month
	parsed = parsed / storage.BlocksPerMonth
	return
}

// parseExpectedRedundancy will parse the string into the form of rentPayment.ExpectedRedundancy
func parseExpectedRedundancy(redundancy string) (parsed float64, err error) {
	if parsed, err = strconv.ParseFloat(redundancy, 64); err != nil {
		err = fmt.Errorf("error parsing the redundancy into float64: %s", err.Error())
		return
	}

	return
}

// parseEnableIPViolation will parse the string into boolean, which is used to indicate if the
// IP Violation checking is enabled
func parseEnableIPViolation(enable string) (parsed bool, err error) {
	// format the string
	enable = formatString(enable)

	// convert the string into boolean
	switch {
	case enable == "true":
		return true, nil
	case enable == "false":
		return false, nil
	default:
		err = fmt.Errorf("failed to convert the string %s into boolean", enable)
		return
	}
}

// parseSpeed will parse the string into maxUploadSpeed or maxDownloadSpeed which will be used
// to limit the speed while uploading/downloading data
func parseSpeed(uploadDownloadSpeed string) (parsed int64, err error) {
	// remove whitespace and convert all to lower case
	uploadDownloadSpeed = formatString(uploadDownloadSpeed)

	// loop through the speedMultiplier
	for unit, _ := range speedMultiplier {
		// unit bps is ignored due to each unit contains bps as suffix
		if unit == "bps" {
			continue
		}
		if strings.HasSuffix(uploadDownloadSpeed, unit) {
			return speedConvert(uploadDownloadSpeed, unit)
		}
	}

	// check the bps unit
	if strings.HasSuffix(uploadDownloadSpeed, "bps") {
		return speedConvert(uploadDownloadSpeed, "bps")
	}

	// there is no qualified unit
	err = fmt.Errorf("failed to parse the upload/download speed, unit must be included. Here is a list of available units: %s", speedUnit)
	return
}

// speedConvert will convert the upload and download speed to bps, in form of int64
func speedConvert(speed string, unit string) (parsed int64, err error) {
	// remove the suffix
	speed = strings.TrimSuffix(speed, unit)

	// parse the string into int64 format
	if parsed, err = strconv.ParseInt(speed, 10, 64); err != nil {
		err = fmt.Errorf("failed to parse the speed provided, unit must be included. Here is a list of available units: %s", speedUnit)
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

// clientSettingGetDefault will take the clientSetting and check if any filed in the RentPayment is zero
// if so, set the value to default value
func clientSettingGetDefault(setting storage.ClientSetting) (newSetting storage.ClientSetting) {
	if setting.RentPayment.Fund.IsEqual(common.BigInt0) {
		setting.RentPayment.Fund = storage.DefaultRentPayment.Fund
	}

	if setting.RentPayment.StorageHosts == 0 {
		setting.RentPayment.StorageHosts = storage.DefaultRentPayment.StorageHosts
	}

	if setting.RentPayment.Period == 0 {
		setting.RentPayment.Period = storage.DefaultRentPayment.Period
	}

	if setting.RentPayment.RenewWindow == 0 {
		setting.RentPayment.RenewWindow = storage.DefaultRentPayment.RenewWindow
	}

	if setting.RentPayment.ExpectedStorage == 0 {
		setting.RentPayment.ExpectedStorage = storage.DefaultRentPayment.ExpectedStorage
	}

	if setting.RentPayment.ExpectedUpload == 0 {
		setting.RentPayment.ExpectedUpload = storage.DefaultRentPayment.ExpectedUpload
	}

	if setting.RentPayment.ExpectedDownload == 0 {
		setting.RentPayment.ExpectedDownload = storage.DefaultRentPayment.ExpectedDownload
	}

	if setting.RentPayment.ExpectedRedundancy == 0 {
		setting.RentPayment.ExpectedRedundancy = storage.DefaultRentPayment.ExpectedRedundancy
	}

	return setting
}
