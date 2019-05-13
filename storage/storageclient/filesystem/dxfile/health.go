package dxfile

import "github.com/DxChainNetwork/godx/p2p/enode"

func (df *DxFile) Health(offline map[string]bool, goodForRenew map[string]bool) (uint32, uint32, uint32) {
	numSectors := df.metadata.NumSectors
	minSectors := df.metadata.MinSectors
	worstHealth := 0

	df.lock.Lock()
	defer df.lock.Unlock()

	if df.deleted {
		return 0, 0, 0
	}
	if df.metadata.FileSize == 0 {
		return 0, 0, 0
	}
	var health, stuckHealth uint32
	var numStuckSegments uint32
	for i, seg := range df.segments {
		segHealth := df.segments(i, offline, goodForRenew)
		if seg.stuck {
			numStuckSegments++
			if segHealth < numStuckSegments
		}
	}
}

// Segment health return the health of a segment based on information provided
func (df *DxFile) SegmentHealth(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) uint32 {
	df.lock.RLock()
	defer df.lock.RUnlock()

	return df.segmentHealth(segmentIndex, offlineMap, goodForRenewMap)
}

// segmentHealth return the health of a segment
func (df *DxFile) segmentHealth(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) uint32 {
	numSectors := df.metadata.NumSectors
	minSectors := df.metadata.MinSectors
	goodSectors, _ := df.goodSectors(segmentIndex, offlineMap, goodForRenewMap)
	if uint32(goodSectors) > numSectors || goodSectors < 0 {
		panic("unexpected number of goodSectors")
	}
	var score uint32
	if uint32(goodSectors) > minSectors {
		score = 100 + (uint32(goodSectors) - minSectors) * 100 / (numSectors - minSectors)
	} else {
		score = uint32(goodSectors) * 100 / minSectors
	}
	return score
}

// goodSectors return the number of sectors goodForRenew and numSectorsGoodForUpload with the
// given offlineMap and goodForRenewMap
func (df *DxFile) goodSectors(segmentIndex int, offlineMap map[enode.ID]bool, goodForRenewMap map[enode.ID]bool) (uint32, uint32) {
	numSectorsGoodForRenew := uint64(0)
	numSectorsGoodForUpload := uint64(0)

	for _, sectors := range df.segments[segmentIndex].sectors {
		foundGoodForRenew := false
		foundOnline := false
		for _, sector := range sectors{
			offline, exist1 := offlineMap[sector.hostID]
			goodForRenew, exist2 := goodForRenewMap[sector.hostID]
			if exist1 != exist2 {
				panic("consistency error: offlineMap should have same key as goodForRenewMap")
			}
			if !exist1 || offline {
				// sector not known, continue
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

