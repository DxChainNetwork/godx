// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package storageclient

import (
	"fmt"

	"github.com/DxChainNetwork/godx/common/unit"
	"github.com/DxChainNetwork/godx/core/types"
	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

// ContractMetaDataAPIDisplay is the data structure used for console
// contract information display purposes
type ContractMetaDataAPIDisplay struct {
	ID                     string
	EnodeID                enode.ID
	LatestContractRevision types.StorageContractRevision
	StartHeight            string
	EndHeight              string

	ContractBalance string

	UploadCost   string
	DownloadCost string
	StorageCost  string
	TotalCost    string
	GasCost      string
	ContractFee  string

	UploadAbility string
	RenewAbility  string
	Canceled      string
}

// formatContractMetaData will format the contract meta data into a format of contract
func formatContractMetaData(data storage.ContractMetaData) (formatted ContractMetaDataAPIDisplay) {
	formatted.ID = data.ID.String()
	formatted.EnodeID = data.EnodeID
	formatted.LatestContractRevision = data.LatestContractRevision
	formatted.StartHeight = fmt.Sprintf("%v b", data.StartHeight)
	formatted.EndHeight = fmt.Sprintf("%v b", data.EndHeight)

	formatted.ContractBalance = unit.FormatCurrency(data.ContractBalance)
	formatted.UploadCost = unit.FormatCurrency(data.UploadCost)
	formatted.DownloadCost = unit.FormatCurrency(data.DownloadCost)
	formatted.StorageCost = unit.FormatCurrency(data.StorageCost)
	formatted.TotalCost = unit.FormatCurrency(data.TotalCost)
	formatted.GasCost = unit.FormatCurrency(data.GasCost)
	formatted.ContractFee = unit.FormatCurrency(data.ContractFee)

	formatted.UploadAbility, formatted.RenewAbility, formatted.Canceled =
		formatStatus(data.Status.UploadAbility, data.Status.RenewAbility, data.Status.Canceled)
	return
}

// formatStatus will format the storage contract status into human understandable format
func formatStatus(upload, renew, canceled bool) (formatUpload, formatRenew, formatCanceled string) {
	if upload {
		formatUpload = fmt.Sprintf("the contract can be used for data uploading")
	} else {
		formatUpload = fmt.Sprintf("the contract can not be used for data uploading")
	}

	if renew {
		formatRenew = fmt.Sprintf("the contract can be used for data downloading")
	} else {
		formatRenew = fmt.Sprintf("the contract can not be used for data downloading")
	}

	if canceled {
		formatCanceled = fmt.Sprintf("the contract has been canceled, it cannot be renewed or used for data uploading")
	} else {
		formatCanceled = fmt.Sprintf("the contract is still active")
	}
	return
}

// formatClientSetting will convert the ClientSetting data into more user friendly data type
// ClientSettingAPIDisplay, which is used for console display.
func formatClientSetting(setting storage.ClientSetting) (formatted storage.ClientSettingAPIDisplay) {
	formatted.EnableIPViolation = formatIPViolation(setting.EnableIPViolation)
	formatted.MaxUploadSpeed = unit.FormatSpeed(setting.MaxUploadSpeed)
	formatted.MaxDownloadSpeed = unit.FormatSpeed(setting.MaxDownloadSpeed)
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

// formatRentPayment is used to format rentPayment field for displaying
// purpose
func formatRentPayment(rent storage.RentPayment) (formatted storage.RentPaymentAPIDisplay) {
	formatted.Fund = unit.FormatCurrency(rent.Fund)
	formatted.StorageHosts = formatHosts(rent.StorageHosts)
	formatted.Period = unit.FormatTime(rent.Period)
	formatted.ExpectedStorage = unit.FormatStorage(rent.ExpectedStorage, true)
	formatted.ExpectedUpload = unit.FormatStorage(rent.ExpectedUpload, false)
	formatted.ExpectedDownload = unit.FormatStorage(rent.ExpectedDownload, false)
	formatted.ExpectedRedundancy = formatRedundancy(rent.ExpectedRedundancy)
	return
}

// formatHosts is used to format the rentPayment.StorageHosts field for displaying purpose
func formatHosts(hosts uint64) (formatted string) {
	return fmt.Sprintf("%v Hosts", hosts)
}

// formatRedundancy is used to format the redundancy setting for console
// displaying purpose
func formatRedundancy(redundancy float64) (formatted string) {
	return fmt.Sprintf("%v Copies", redundancy)
}
