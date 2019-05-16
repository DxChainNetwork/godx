// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package dxfile

import "github.com/DxChainNetwork/godx/p2p/enode"

// repairHealthThreshold is the threshold that file with smaller health is marked as Stuck and
// to be repaired
const repairHealthThreshold = 175

// Health return check for dxFile's segments and return the health, stuckHealth, and numStuckSegments
func (df *DxFile) Health(offline map[enode.ID]bool, goodForRenew map[enode.ID]bool) (uint32, uint32, uint32) {
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
		segHealth := df.segmentHealth(i, offline, goodForRenew)
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
func (df *DxFile) SegmentHealth(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) uint32 {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.segmentHealth(segmentIndex, offlineMap, goodForRenewMap)
}

// segmentHealth return the health of a Segment.
// Health 0~100: unrecoverable from contracts
// Health 100~200: recoverable
// Health 200: No fix needed
func (df *DxFile) segmentHealth(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) uint32 {
	numSectors := df.metadata.NumSectors
	minSectors := df.metadata.MinSectors
	goodSectors, _ := df.goodSectors(segmentIndex, offlineMap, goodForRenewMap)
	if uint32(goodSectors) > numSectors || goodSectors < 0 {
		panic("unexpected number of goodSectors")
	}
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
func (df *DxFile) goodSectors(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) (uint32, uint32) {
	numSectorsGoodForRenew := uint64(0)
	numSectorsGoodForUpload := uint64(0)

	for _, sectors := range df.segments[segmentIndex].Sectors {
		foundGoodForRenew := false
		foundOnline := false
		for _, sector := range sectors {
			offline, exist1 := offlineMap[sector.HostID]
			goodForRenew, exist2 := goodForRenewMap[sector.HostID]
			if exist1 != exist2 {
				panic("consistency error: offlineMap should have same key as goodForRenewMap")
			}
			if !exist1 || offline {
				// Sector not known, continue
				continue
			}
			if goodForRenew {
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

// cmpHealth compare two health. The cmpHealth result returns the priority the health related Segment should be fixed
// 200 < 100 < 150 < 199 < 0 < 50 < 99
func CmpHealth(h1, h2 uint32) int {
	if h1 == h2 {
		return 0
	}
	if h1 == 200 || h2 == 200 {
		if h1 == 200 {
			return -1
		}
		return 1
	}
	if (h1 >= 100) == (h2 >= 100) {
		if h1 < h2 {
			return -1
		}
		return 1
	}
	if h1 >= 100 {
		return -1
	}
	return 1
}
