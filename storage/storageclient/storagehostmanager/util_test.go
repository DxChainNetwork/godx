// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"
)

func TestActiveHostInfoGenerator(t *testing.T) {
	shm := newHostManagerTestData()
	infos := make(map[string]struct{})

	for i := 0; i < 10; i++ {
		hi := activeHostInfoGenerator()
		infos[hi.EnodeID.String()] = struct{}{}
		err := shm.insert(hi)
		if err != nil {
			t.Fatalf("error: insert failed")
		}
	}

	api := NewPublicStorageHostManagerAPI(shm)
	activeInfos := api.ActiveStorageHosts()
	for _, info := range activeInfos {
		_, exists := infos[info.EnodeID.String()]
		if !exists {
			t.Errorf("error: storage host with ID %s should be active: numScanRecords: %v, success: %t, acceptContract %t",
				info.EnodeID.String(), len(info.ScanRecords), info.ScanRecords[len(info.ScanRecords)-1].Success,
				info.AcceptingContracts)
		}
	}
}
