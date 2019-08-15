// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import "github.com/DxChainNetwork/godx/storage"

// InteractionType is the code for interactions
type InteractionType uint8

const (
	// InteractionInvalid is the code that's not valid
	InteractionInvalid InteractionType = iota

	// InteractionGetConfig is the code for client's get config interaction
	InteractionGetConfig

	// InteractionCreateContract is the interaction code for client's create contract
	// negotiation
	InteractionCreateContract

	// InteractionRenewContract is the interaction code for client's renew contract
	// negotiation
	InteractionRenewContract

	// InteractionUpload is the interaction code for client's upload negotiation
	InteractionUpload

	// InteractionDownload is the interaction code for client's download negotiation
	InteractionDownload
)

var (
	// interactionTypeToNameDict is the mapping from type to name string
	interactionTypeToNameDict = map[InteractionType]string{
		InteractionGetConfig:      "host config scan",
		InteractionCreateContract: "create contract",
		InteractionRenewContract:  "renew contract",
		InteractionUpload:         "upload",
		InteractionDownload:       "download",
	}

	// interactionNameToTypeDict is the mapping from name string to type
	interactionNameToTypeDict = map[string]InteractionType{
		"host config scan": InteractionGetConfig,
		"create contract":  InteractionCreateContract,
		"renew contract":   InteractionRenewContract,
		"upload":           InteractionUpload,
		"download":         InteractionDownload,
	}

	// interactonWeight is the mapping from interaction type to weight
	interactonWeight = map[InteractionType]float64{
		InteractionGetConfig:      1,
		InteractionCreateContract: 2,
		InteractionRenewContract:  2,
		InteractionUpload:         5,
		InteractionDownload:       10,
	}
)

// InteractionTypeToName translate the name of the interaction type to human readable name
func InteractionTypeToName(it InteractionType) string {
	if _, exist := interactionTypeToNameDict[it]; !exist {
		return ""
	}
	return InteractionTypeToName(it)
}

// InteractionNameToType translate the human readable interaction name to type
func InteractionNameToType(name string) InteractionType {
	if _, exist := interactionNameToTypeDict[name]; !exist {
		return InteractionInvalid
	}
	return interactionNameToTypeDict[name]
}

// interactionWeight return the weight of the interaction type
func interactionWeight(it InteractionType) float64 {
	if weight, exist := interactonWeight[it]; exist {
		return weight
	}
	return 0
}

// interactionInitiate initiate the interaction related fields, which gives the interaction factors
// an initial value.
func interactionInitiate(info *storage.HostInfo) {
	if info.SuccessfulInteractionFactor == 0 && info.FailedInteractionFactor == 0 {
		info.SuccessfulInteractionFactor = initialSuccessfulInteractionFactor
		info.FailedInteractionFactor = initialFailedInteractionFactor
	}
}
