// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"time"

	"github.com/DxChainNetwork/godx/storage"
)

// onlineBackend is the backend that gets whether the backend is online or not
type onlineBackend interface {
	Online() bool
}

// hostConfigUpdate calculate the try to update the host config wight the host info.
// It will update the uptime fields as well as the interaction fields.
func (shm *StorageHostManager) hostInfoUpdate(info storage.HostInfo, b onlineBackend, err error) error {
	// if error happens due to the backend is not online, directly return
	if err != nil && !b.Online() {
		return nil
	}
	// get the host info from the tree
	storedInfo, exist := shm.storageHostTree.RetrieveHostInfo(info.EnodeID)
	if exist {
		info = applyNewHostInfoToStoredHostInfo(info, storedInfo)
	}
	success := err == nil
	info = calcUptimeUpdate(info, success, uint64(time.Now().Unix()))
	info = calcInteractionUpdate(info, InteractionGetConfig, success, uint64(time.Now().Unix()))

	// Check whether to remove the host
	remove := whetherRemoveHost(info, shm.blockHeight)
	if remove {
		return shm.remove(storedInfo.EnodeID)
	}
	if exist {
		return shm.modify(info)
	} else {
		return shm.insert(info)
	}
}

// whetherRemoveHost decide whether to remove the host from host manager with the given host info.
// The decision is made upon whether the uprate is above the a certain criteria
func whetherRemoveHost(info storage.HostInfo, currentBlockHeight uint64) bool {
	upRate := getHostUpRate(info)
	criteria := calcHostRemoveCriteria(info, currentBlockHeight)
	if upRate > criteria {
		return false
	} else {
		return true
	}
}

// calcHostRemoveCriteria calculate the criteria for removing a host
func calcHostRemoveCriteria(info storage.HostInfo, currentBlockHeight uint64) float64 {
	timeDiff := float64(currentBlockHeight - info.FirstSeen)
	criteria := uptimeCap - (uptimeCap-critIntercept)/(timeDiff/float64(critRemoveBase)+1)
	return criteria
}
