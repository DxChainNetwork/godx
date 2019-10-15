// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/storage"
)

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
			fund, err = unit.ParseCurrency(value)
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
			period, err = unit.ParseTime(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the period value: %s", err.Error())
				break
			}
			clientSetting.RentPayment.Period = period

		case key == "violation":
			var status bool
			status, err = unit.ParseBool(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the ip violation: %s", err.Error())
				break
			}
			clientSetting.EnableIPViolation = status

		case key == "uploadspeed":
			var uploadSpeed int64
			uploadSpeed, err = unit.ParseSpeed(value)
			if err != nil {
				err = fmt.Errorf("failed to parse the uplaod speed: %s", err.Error())
				break
			}
			clientSetting.MaxUploadSpeed = uploadSpeed

		case key == "downloadspeed":
			var downloadSpeed int64
			downloadSpeed, err = unit.ParseSpeed(value)
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

// parseStorageHosts will parse the string version of storage hosts into uint64 type
func parseStorageHosts(hosts string) (parsed uint64, err error) {
	return unit.ParseUint64(hosts, 1, "")
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
