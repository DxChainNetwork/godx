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

var currencyIndexMap = map[string]int{
	"ndx": 0,
	"udx": 1,
	"mdx": 2,
	"dx":  3,
	"Kdx": 4,
	"Mdx": 5,
	"Gdx": 6,
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

func parseClientSetting(settings map[string]string, prevSetting storage.ClientSetting) (clientSetting storage.ClientSetting, err error) {
	// get the previous settings
	clientSetting = prevSetting

	// parse the ClientSettingAPI
	for key, value := range settings {
		switch {
		case key == "fund":
			fund, err := parseFund(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.Fund = fund

		case key == "hosts":
			hosts, err := parseStorageHosts(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.StorageHosts = hosts

		case key == "period":
			period, err := parsePeriodAndRenew(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.Period = period

		case key == "renew":
			renew, err := parsePeriodAndRenew(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.RenewWindow = renew

		case key == "storage":
			expectedStorage, err := parseExpectedStorage(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.ExpectedStorage = expectedStorage

		case key == "upload":
			expectedUpload, err := parseExpectedUpload(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.ExpectedUpload = expectedUpload

		case key == "download":
			expectedDownload, err := parseExpectedDownload(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.ExpectedDownload = expectedDownload

		case key == "redundancy":
			redundancy, err := parseExpectedRedundancy(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.ExpectedRedundancy = redundancy

		case key == "violation":
			status, err := parseEnableIPViolation(value)
			if err != nil {
				break
			}
			clientSetting.EnableIPViolation = status

		case key == "uploadspeed":
			uploadSpeed, err := parseMaxUploadSpeed(value)
			if err != nil {
				break
			}
			clientSetting.MaxUploadSpeed = uploadSpeed

		case key == "downloadspeed":
			downloadSpeed, err := parseMaxDownloadSpeed(value)
			if err != nil {
				break
			}
			clientSetting.MaxDownloadSpeed = downloadSpeed
		}
	}

	return
}

// parseFund will parse the user string input, and convert it into common.BigInt
// type in terms of hump, which is the smallest currency unit
func parseFund(fund string) (parsed common.BigInt, err error) {

	// verify the length first
	if len(fund) < 3 {
		err = fmt.Errorf("currency unit is expected")
		return
	}

	// remove all white spaces, and convert the character d and x to lower case
	fund = strings.Replace(fund, " ", "", -1)
	fund = strings.Replace(fund, "D", "d", -1)
	fund = strings.Replace(fund, "X", "x", -1)

	// check the special uint h
	if strings.HasSuffix(fund, "h") {
		return stringToBigInt("h", fund, -1)
	}

	// check the suffix and convert the units to hump, which is the smallest unit
	// for the dx currency
	for unit, index := range currencyIndexMap {
		if unit == "dx" {
			continue
		}

		if strings.HasSuffix(fund, unit) {
			return stringToBigInt(unit, fund, index)
		}
	}

	// at the end, check if the string contains dx suffix
	if strings.HasSuffix(fund, "dx") {
		return stringToBigInt("dx", fund, currencyIndexMap["dx"])
	}

	// if the provided currency unit cannot be found from the currency unit list
	// return error
	err = fmt.Errorf("the provided currency unit is invalid")
	return
}

// stringToBigInt will convert the string to common.BigInt type
func stringToBigInt(unit, fund string, index int) (parsed common.BigInt, err error) {
	var bigInt = new(big.Int)

	// remove the unit
	fund = strings.TrimSuffix(fund, unit)

	// check if the string contains only digit
	if !containsDigitOnly(fund) {
		err = fmt.Errorf("the fund is expected to not contain any letter %s", fund)
		return
	}

	// convert the string to *big.int
	if _, err = fmt.Sscan(fund, bigInt); err != nil {
		err = fmt.Errorf("failed to convert the string to *big.Int: %s", err.Error())
		return
	}

	// convert the unit to hump, and convert the type to common.BigInt
	if unit != "h" {
		parsed = fundUnitConversion(index, bigInt)
		return
	}

	parsed = common.PtrBigInt(bigInt)
	return
}

// containsDigitOnly checks if a string contains only digit
func containsDigitOnly(s string) (digitOnly bool) {
	notDigit := func(c rune) bool { return c < '0' || c > '9' }
	return strings.IndexFunc(s, notDigit) == -1
}

// fundUnitConversion will convert the fund into unit hump, which is the smallest
// currency unit
func fundUnitConversion(index int, fund *big.Int) (converted common.BigInt) {
	exp := 24 + 3*(index-3)
	mag := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
	converted = common.PtrBigInt(new(big.Int).Mul(fund, mag))
	return
}

// parseStorageHosts will parse the string version of storage hosts into uint64 type
func parseStorageHosts(hosts string) (parsed uint64, err error) {
	var parsedInt int64
	if parsedInt, err = strconv.ParseInt(hosts, 10, 64); err != nil {
		err = fmt.Errorf("error parsing the storage hsots: %s", err.Error())
		return
	}
	parsed = uint64(parsedInt)
	return
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
		err = fmt.Errorf("the unit provided is not valid: %s", unit)
		return
	}
}

func convertUint64(data string, factor uint64, unit string) (parsed uint64, err error) {
	var parsedInt int64

	// remove the unit from the string
	data = strings.TrimSuffix(data, unit)

	if parsedInt, err = strconv.ParseInt(data, 10, 64); err != nil {
		err = fmt.Errorf("error parsing to uint64, invalid unit: %s", err.Error())
		return
	}

	parsed = uint64(parsedInt) * factor
	return
}

func parseExpectedStorage(storage string) (parsed uint64, err error) {
	return dataSizeConverter(storage)
}

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

	err = fmt.Errorf("data provided does not have valid unit: %s", dataSize)
	return
}

func parseExpectedUpload(upload string) (parsed uint64, err error) {
	if parsed, err = dataSizeConverter(upload); err != nil {
		return
	}

	// in terms of bytes / month
	parsed = parsed / storage.BlocksPerMonth
	return
}

func parseExpectedDownload(download string) (parsed uint64, err error) {
	if parsed, err = dataSizeConverter(download); err != nil {
		return
	}

	// in terms of bytes / month
	parsed = parsed / storage.BlocksPerMonth
	return
}

func parseExpectedRedundancy(redundancy string) (parsed float64, err error) {
	if parsed, err = strconv.ParseFloat(redundancy, 64); err != nil {
		err = fmt.Errorf("error parsing the redundancy into float64: %s", err.Error())
		return
	}

	return
}

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

func parseMaxUploadSpeed(uploadSpeed string) (parsed int64, err error) {
	// in terms of bytes/seconds
	uploadSpeed = formatString(uploadSpeed)
	return strconv.ParseInt(uploadSpeed, 10, 64)
}

func parseMaxDownloadSpeed(downloadSpeed string) (parsed int64, err error) {
	// in terms of bytes/seconds
	formatString(downloadSpeed)
	return strconv.ParseInt(downloadSpeed, 10, 64)
}

func formatString(s string) (formatted string) {
	s = strings.Replace(s, " ", "", -1)
	s = strings.ToLower(s)
	return s
}

func clientSettingValidation(setting storage.ClientSetting) (newSetting storage.ClientSetting) {
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
