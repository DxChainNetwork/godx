// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package contractmanager

import "github.com/DxChainNetwork/godx/storage"

// isOffline will check if a storage host is online or not based on the number of scanRecords
// and the successful rate of the records
func isOffline(host storage.HostInfo) (offline bool) {
	if len(host.ScanRecords) < 1 {
		return true
	}

	if len(host.ScanRecords) == 1 {
		return !host.ScanRecords[0].Success
	}

	offline = !(host.ScanRecords[len(host.ScanRecords)-1].Success || host.ScanRecords[len(host.ScanRecords)-2].Success)
	return
}
