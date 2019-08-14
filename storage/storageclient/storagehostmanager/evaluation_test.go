// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"fmt"
	"testing"

	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/storage"
)

func TestPresenceFactorCalc(t *testing.T) {
	tests := []struct {
		presence uint64
	}{
		{lowTimeLimit},
		{lowTimeLimit + 1},
		{10},
		{100},
		{1000},
		{10000},
		{100000},
		{highTimeLimit - 1},
		{highTimeLimit},
		{highTimeLimit + 1},
	}
	for _, test := range tests {
		// first Seen must not be 0
		firstSeen := uint64(10)
		shm := &StorageHostManager{
			blockHeight: test.presence + firstSeen,
		}
		info := storage.HostInfo{
			FirstSeen: firstSeen,
		}
		factor := shm.presenceFactorCalc(info)
		if test.presence <= lowTimeLimit {
			if factor != lowValueLimit {
				t.Errorf("low limit test failed")
			}
		} else if test.presence >= highTimeLimit {
			if factor != highValueLimit {
				t.Errorf("high limit test failed")
			}
		} else {
			// Calculate the value before and after the limit, and check whether the
			// factor is incrementing
			if test.presence == 0 || test.presence == math.MaxUint64 {
				continue
			}
			factorSmaller := shm.presenceFactorCalc(storage.HostInfo{FirstSeen: firstSeen + 1})
			factorLarger := shm.presenceFactorCalc(storage.HostInfo{FirstSeen: firstSeen - 1})
			if factorSmaller >= factor || factor >= factorLarger {
				t.Errorf("Near range %d the factor not incrementing", test.presence)
			}
		}
	}
}

// TestIllegalPresenceFactorCalc test the illegal case of host manager's blockHeight is smaller
// than the first seen, which should yield a factor of 0
func TestIllegalPresenceFactorCalc(t *testing.T) {
	firstSeen := uint64(30)
	blockHeight := uint64(10)
	shm := &StorageHostManager{
		blockHeight: blockHeight,
	}
	info := storage.HostInfo{
		FirstSeen: firstSeen,
	}
	factor := shm.presenceFactorCalc(info)
	fmt.Println(factor)
	if firstSeen > blockHeight && factor != 0 {
		t.Errorf("Illegal input for presence factor calculation does not give 0 factor")
	}
}
