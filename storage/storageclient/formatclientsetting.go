// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/storage"
)

// formatClientSetting will convert the ClientSetting data into more user friendly data type
// ClientSettingAPIDisplay, which is used for console display.
func formatClientSetting(setting storage.ClientSetting) (formatted storage.ClientSettingAPIDisplay) {
	formatted.EnableIPViolation = formatIPViolation(setting.EnableIPViolation)
	formatted.MaxUploadSpeed = formatMaxUploadAndDownloadSpeed(setting.MaxUploadSpeed)
	formatted.MaxDownloadSpeed = formatMaxUploadAndDownloadSpeed(setting.MaxDownloadSpeed)
	formatted.RentPayment = formatRentPayment(setting.RentPayment)
	return
}

// formatIPViolation is used to format storage.ClientSetting.IPViolation field
func formatIPViolation(enabled bool) (formatted string) {
	if enabled {
		formatted = "Enabled: storage hosts from same network will be disabled"
	} else {
		formatted = "Disabled: storage client can sign contract with storage hosts from the same network"
	}
	return
}

// formatMaxUploadAndDownloadSpeed is used to format max upload and download field for displaying
// purpose
func formatMaxUploadAndDownloadSpeed(speed int64) (formatted string) {
	// if the speed is 0, means unlimited
	if speed == 0 {
		formatted = fmt.Sprintf("Unlimited")
		return
	}

	switch {
	case speed%speedMultiplier["tbps"] == 0:
		formatted = fmt.Sprintf("%v Tbps", speed/speedMultiplier["tbps"])
		return
	case speed%speedMultiplier["gbps"] == 0:
		formatted = fmt.Sprintf("%v Gbps", speed/speedMultiplier["gbps"])
		return
	case speed%speedMultiplier["mbps"] == 0:
		formatted = fmt.Sprintf("%v Mbps", speed/speedMultiplier["mbps"])
		return
	case speed%speedMultiplier["kbps"] == 0:
		formatted = fmt.Sprintf("%v Kbps", speed/speedMultiplier["kbps"])
		return
	default:
		formatted = fmt.Sprintf("%v bps", speed)
		return
	}
}

// formatRentPayment is used to format rentPayment field for displaying
// purpose
func formatRentPayment(rent storage.RentPayment) (formatted storage.RentPaymentAPIDisplay) {
	formatted.Fund = formatFund(rent.Fund)
	formatted.StorageHosts = formatHosts(rent.StorageHosts)
	formatted.Period = formatPeriodAndRenewWindow(rent.Period)
	formatted.RenewWindow = formatPeriodAndRenewWindow(rent.RenewWindow)
	formatted.ExpectedStorage = formatExpectedData(rent.ExpectedStorage, true)
	formatted.ExpectedUpload = formatExpectedData(rent.ExpectedUpload, false)
	formatted.ExpectedDownload = formatExpectedData(rent.ExpectedDownload, false)
	formatted.ExpectedRedundancy = formatRedundancy(rent.ExpectedRedundancy)
	return
}

// formatFund is used to format the rentPayment.Fund field for displaying purpose
func formatFund(fund common.BigInt) (formatted string) {
	switch {
	case fund.DivNoRemaining(currencyIndexMap["ether"]):
		formatted = fmt.Sprintf("%v ether", fund.DivUint64(currencyIndexMap["ether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["milliether"]):
		formatted = fmt.Sprintf("%v milliether", fund.DivUint64(currencyIndexMap["milliether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["microether"]):
		formatted = fmt.Sprintf("%v microether", fund.DivUint64(currencyIndexMap["microether"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["gwei"]):
		formatted = fmt.Sprintf("%v Gwei", fund.DivUint64(currencyIndexMap["gwei"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["mwei"]):
		formatted = fmt.Sprintf("%v Mwei", fund.DivUint64(currencyIndexMap["mwei"]))
		return
	case fund.DivNoRemaining(currencyIndexMap["kwei"]):
		formatted = fmt.Sprintf("%v Kwei", fund.DivUint64(currencyIndexMap["kwei"]))
		return
	default:
		formatted = fmt.Sprintf("%v wei", fund)
		return
	}
}

// formatHosts is used to format the rentPayment.StorageHosts field for displaying purpose
func formatHosts(hosts uint64) (formatted string) {
	return fmt.Sprintf("%v Hosts", hosts)
}

// formatPeriodAndRenewWindow is used to format the period and renewWindow field
// for displaying purpose
func formatPeriodAndRenewWindow(periodRenew uint64) (formatted string) {
	switch {
	case periodRenew%storage.BlocksPerYear == 0:
		formatted = fmt.Sprintf("%v Year(s)", periodRenew/storage.BlocksPerYear)
		return
	case periodRenew%storage.BlocksPerMonth == 0:
		formatted = fmt.Sprintf("%v Month(s)", periodRenew/storage.BlocksPerMonth)
		return
	case periodRenew%storage.BlocksPerWeek == 0:
		formatted = fmt.Sprintf("%v Week(s)", periodRenew/storage.BlocksPerWeek)
		return
	case periodRenew%storage.BlocksPerDay == 0:
		formatted = fmt.Sprintf("%v Day(s)", periodRenew/storage.BlocksPerDay)
		return
	case periodRenew%storage.BlockPerHour == 0:
		formatted = fmt.Sprintf("%v Day(s)", periodRenew/storage.BlockPerHour)
		return
	default:
		formatted = fmt.Sprintf("%v Minute(s)", float64(periodRenew)/float64(storage.BlockPerMin))
		return
	}
}

// formatExpectedData is used to format the data for console display purpose
func formatExpectedData(dataSize uint64, storage bool) (formatted string) {
	additionalInfo := ""
	if !storage {
		additionalInfo = "/block"
	}

	switch {
	case dataSize%dataSizeMultiplier["tib"] == 0:
		formatted = fmt.Sprintf("%v TiB%s", dataSize/dataSizeMultiplier["tib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["gib"] == 0:
		formatted = fmt.Sprintf("%v GiB%s", dataSize/dataSizeMultiplier["gib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["mib"] == 0:
		formatted = fmt.Sprintf("%v MiB%s", dataSize/dataSizeMultiplier["mib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["kib"] == 0:
		formatted = fmt.Sprintf("%v KiB%s", dataSize/dataSizeMultiplier["kib"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["tb"] == 0:
		formatted = fmt.Sprintf("%v TB%s", dataSize/dataSizeMultiplier["tb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["gb"] == 0:
		formatted = fmt.Sprintf("%v GB%s", dataSize/dataSizeMultiplier["gb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["mb"] == 0:
		formatted = fmt.Sprintf("%v MB%s", dataSize/dataSizeMultiplier["mb"], additionalInfo)
		return
	case dataSize%dataSizeMultiplier["kb"] == 0:
		formatted = fmt.Sprintf("%v KB%s", dataSize/dataSizeMultiplier["kb"], additionalInfo)
		return
	default:
		formatted = fmt.Sprintf("%v B%s", dataSize, additionalInfo)
		return
	}
}

// formatRedundancy is used to format the redundancy setting for console
// displaying purpose
func formatRedundancy(redundancy float64) (formatted string) {
	return fmt.Sprintf("%v Copies", redundancy)
}
