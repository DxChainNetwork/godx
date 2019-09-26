// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"fmt"
	"math"
	"time"

	"github.com/DxChainNetwork/godx/p2p/enode"
	"github.com/DxChainNetwork/godx/storage"
)

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
		InteractionRenewContract:  5,
		InteractionUpload:         5,
		InteractionDownload:       10,
	}
)

// String return the string representation of the InteractionType
func (it InteractionType) String() string {
	if _, exist := interactionTypeToNameDict[it]; !exist {
		return ""
	}
	return interactionTypeToNameDict[it]
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
		info.LastInteractionTime = uint64(time.Now().Unix())
	}
}

// IncrementSuccessfulInteractions will update storage host's interactions factors
func (shm *StorageHostManager) IncrementSuccessfulInteractions(id enode.ID, interactionType InteractionType) {
	if err := shm.updateInteraction(id, interactionType, true); err != nil {
		shm.log.Warn("Increment successful interactions", "err", err)
		return
	}
	return
}

// IncrementFailedInteractions will update storage host's interactions factors
func (shm *StorageHostManager) IncrementFailedInteractions(id enode.ID, interactionType InteractionType) {
	if err := shm.updateInteraction(id, interactionType, false); err != nil {
		shm.log.Warn("Increment failed interactions", "err", err)
		return
	}
	return
}

// updateInteraction update the host info with the give id, interaction type ,and whether successful or not
func (shm *StorageHostManager) updateInteraction(id enode.ID, interactionType InteractionType, success bool) error {
	shm.lock.Lock()
	defer shm.lock.Unlock()

	// get the storage host
	info, exist := shm.storageHostTree.RetrieveHostInfo(id)
	if !exist {
		return fmt.Errorf("failed to retrive host info [%v]", id)
	}
	info = calcInteractionUpdate(info, interactionType, success, uint64(time.Now().Unix()))
	// Evaluate the score and update the host info
	score := shm.hostEvaluator.Evaluate(info)
	if err := shm.storageHostTree.HostInfoUpdate(info, score); err != nil {
		return fmt.Errorf("failed to update host info: %v", err)
	}
	return nil
}

// calcInteractionUpdate update the host info with the give interaction type and whether the interaction
// is successful
func calcInteractionUpdate(info storage.HostInfo, interactionType InteractionType, success bool, now uint64) storage.HostInfo {
	// Calculate the weight for the interaction
	weight := interactionWeight(interactionType)
	// Apply the decay the host info
	processDecay(&info, now)
	if success {
		updateSuccessfulInteraction(&info, weight)
	} else {
		updateFailedInteraction(&info, weight)
	}
	updateInteractionRecord(&info, interactionType, success, now)
	return info
}

// processDecay calculate and apply the decay factor to the interaction factors
func processDecay(info *storage.HostInfo, now uint64) {
	// Calculate the decay factor
	timePassed := now - info.LastInteractionTime
	decay := math.Pow(interactionDecay, float64(timePassed))

	// Apply the decay
	info.SuccessfulInteractionFactor *= decay
	info.FailedInteractionFactor *= decay
	info.LastInteractionTime = now
}

// updateInteractionRecord add the current interaction record to the host info
// If the host info has already got 10 or more records, only keep the most recent 10 records
func updateInteractionRecord(info *storage.HostInfo, interactionType InteractionType, success bool,
	now uint64) {
	info.InteractionRecords = append(info.InteractionRecords, storage.HostInteractionRecord{
		Time:            time.Unix(int64(now), 0),
		InteractionType: interactionType.String(),
		Success:         success,
	})
	if len(info.InteractionRecords) > maxNumInteractionRecord {
		info.InteractionRecords = info.InteractionRecords[len(info.InteractionRecords)-maxNumInteractionRecord:]
	}
}

// updateSuccessfulInteraction update the successful factor based on weight
func updateSuccessfulInteraction(info *storage.HostInfo, weight float64) {
	info.SuccessfulInteractionFactor += weight
}

// updateFailedInteraction update the
func updateFailedInteraction(info *storage.HostInfo, weight float64) {
	info.FailedInteractionFactor += weight
}
