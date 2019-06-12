// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import (
	"github.com/DxChainNetwork/godx/storage"
)

// The larger the health value, the healthier the DxFile.
// Health 0 ~ 100: unrecoverable from contracts
// Health 100 ~ 200: recoverable
// Health 200: No fix needed

const (
	// RepairHealthThreshold is the threshold that file with smaller health is marked as Stuck and
	// to be repaired
	RepairHealthThreshold = 175

	// StuckThreshold is the threshold that defines the threshold between the stuck and unstuck segments
	StuckThreshold = 100

	// CompleteHealthThreshold is that this file completely upload all sectors
	CompleteHealthThreshold = 200
)

// Health return check for dxFile's segments and return the health, stuckHealth, and numStuckSegments
// Health 0~100: unrecoverable from contracts
// Health 100~200: recoverable
// Health 200: No fix needed
func (df *DxFile) Health(table storage.HostHealthInfoTable) (uint32, uint32, uint32) {
	df.lock.RLock()
	defer df.lock.RUnlock()

	if df.deleted {
		return 0, 0, 0
	}
	if df.metadata.FileSize == 0 {
		return 0, 0, 0
	}
	health := uint32(200)
	stuckHealth := uint32(200)
	// health, stuckHealth should be the minimum value of the segment health
	var numStuckSegments uint32
	for i, seg := range df.segments {
		segHealth := df.segmentHealth(i, table)
		if seg.Stuck {
			numStuckSegments++
			if segHealth < stuckHealth {
				stuckHealth = segHealth
			}
		} else if segHealth < health {
			health = segHealth
		}
	}
	return health, stuckHealth, numStuckSegments
}

// SegmentHealth return the health of a Segment based on information provided
// Health 0~100: unrecoverable from contracts
// Health 100~200: recoverable
// Health 200: No fix needed
func (df *DxFile) SegmentHealth(segmentIndex int, table storage.HostHealthInfoTable) uint32 {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.segmentHealth(segmentIndex, table)
}

// segmentHealth return the health of a Segment. The larger the health value, the healthier the DxFile
// Health 0~100: unrecoverable from contracts
// Health 100~200: recoverable
// Health 200: No fix needed
func (df *DxFile) segmentHealth(segmentIndex int, table storage.HostHealthInfoTable) uint32 {
	numSectors := df.metadata.NumSectors
	minSectors := df.metadata.MinSectors
	goodSectors, _ := df.goodSectors(segmentIndex, table)
	var score uint32
	if uint32(goodSectors) > minSectors {
		score = 100 + (uint32(goodSectors)-minSectors)*100/(numSectors-minSectors)
	} else {
		score = uint32(goodSectors) * 100 / minSectors
	}
	return score
}

// goodSectors return the number of Sectors goodForRenew and numSectorsGoodForUpload with the
// given offlineMap and goodForRenewMap
func (df *DxFile) goodSectors(segmentIndex int, table storage.HostHealthInfoTable) (uint32, uint32) {
	numSectorsGoodForRenew := uint64(0)
	numSectorsGoodForUpload := uint64(0)

	for _, sectors := range df.segments[segmentIndex].Sectors {
		foundGoodForRenew := false
		foundOnline := false
		for _, sector := range sectors {
			info, exist := table[sector.HostID]
			if !exist || info.Offline {
				// sector not known in table or host is offline, continue to next entry
				continue
			}
			if info.GoodForRenew {
				foundGoodForRenew = true
				break
			}
			foundOnline = true
		}
		if foundGoodForRenew {
			numSectorsGoodForRenew++
			numSectorsGoodForUpload++
		} else if foundOnline {
			numSectorsGoodForUpload++
		}
	}
	return uint32(numSectorsGoodForRenew), uint32(numSectorsGoodForUpload)
}

// CmpRepairPriority compare two health. The compare result returns the priority the health related Segment should be fixed
// The priority is determined by the follows:
// When the file is not recoverable from contract (health 0~99), it has the highest property to recover from disk
// When the file is recoverable (health 100~199), it is then prioritized.
// When the file is totally health, there is no need to recover.
// Thus the priority of recovery is as follows: 175 == RepairHealthThreshold
// (175 ~ 200) < (0 ~ 99) < 174 < 100
// In the following expression p(h) means the priority of the health
// If p(h1) == p(h2), return 0
// If p(h1) < p(h2), return -1
// If p(h1) > p(h2), return 1
func CmpRepairPriority(h1, h2 uint32) int {
	noFix1, noFix2 := h1 >= RepairHealthThreshold, h2 >= RepairHealthThreshold
	canFix1, canFix2 := h1 >= StuckThreshold, h2 >= StuckThreshold
	if h1 == h2 || (noFix1 && noFix2) || (!canFix1 && !canFix2) {
		return 0
	}
	if noFix1 || noFix2 {
		if noFix1 {
			return -1
		}
		return 1
	}
	if !canFix1 || !canFix2 {
		if !canFix1 {
			return -1
		}
		return 1
	}
	if h1 > h2 {
		return -1
	}
	return 1
}
