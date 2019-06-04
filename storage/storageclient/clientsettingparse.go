// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
	"math/big"
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

func (sc *StorageClient) parseClientSetting(settings map[string]string) (clientSetting storage.ClientSetting, err error) {
	// get the previous settings
	clientSetting = sc.RetrieveClientSetting()

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
			period, err := parsePeriod(value)
			if err != nil {
				break
			}
			clientSetting.RentPayment.Period = period

		case key == "renew":
			renew, err := parseRenewWindow(value)
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

func parseFund(fund string) (parsed common.BigInt, err error) {

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

func fundUnitConversion(index int, fund *big.Int) (converted common.BigInt) {
	exp := 24 + 3*(index-3)
	mag := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
	converted = common.PtrBigInt(new(big.Int).Mul(fund, mag))
	return
}

func parseStorageHosts(hosts string) (parsed uint64, err error) {
	return
}

func parsePeriod(period string) (parsed uint64, err error) {
	return
}

func parseRenewWindow(renew string) (parsed uint64, err error) {
	return
}

func parseExpectedStorage(storage string) (parsed uint64, err error) {
	return
}

func parseExpectedUpload(upload string) (parsed uint64, err error) {
	return
}

func parseExpectedDownload(download string) (parsed uint64, err error) {
	return
}

func parseExpectedRedundancy(redundancy string) (parsed float64, err error) {
	return
}

func parseEnableIPViolation(enable string) (parsed bool, err error) {
	return
}

func parseMaxUploadSpeed(uploadSpeed string) (parsed int64, err error) {
	return
}

func parseMaxDownloadSpeed(downloadSpeed string) (parsed int64, err error) {
	return
}
