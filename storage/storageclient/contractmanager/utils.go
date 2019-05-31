package contractmanager

import "github.com/DxChainNetwork/godx/storage"

// isOffline will check if a storage host is online or not
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
