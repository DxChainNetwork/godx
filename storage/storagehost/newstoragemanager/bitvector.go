// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"github.com/DxChainNetwork/godx/common/math"
)

// bitVector is used to represent a boolean vector of size 64
// the decimal number is considered as binary, and each bit
// indicating the true or false at an index
type bitVector uint64

// isFree check if the value at given index is free
func (vec bitVector) isFree(idx uint64) bool {
	var mask bitVector = 1 << idx
	value := vec & mask
	return value>>idx == 0
}

// setUsage set given index to 1
func (vec *bitVector) setUsage(idx uint64) {
	var mask bitVector = 1 << idx
	*vec = *vec | mask
}

// clearUsage clear given index to 0
func (vec *bitVector) clearUsage(idx uint64) {
	var mask bitVector = math.MaxUint64
	mask = mask - 1<<idx
	*vec = *vec & bitVector(mask)
}

// emptyUsage create a new empty bitVector slice used as usage in storageFolder with the
// expected size
func emptyUsage(size uint64) (usage []bitVector) {
	numSectors := sizeToNumSectors(size)
	usageSize := numSectors / bitVectorGranularity
	if numSectors%bitVectorGranularity != 0 {
		usageSize++
	}
	usage = make([]bitVector, usageSize)
	return
}

func expandUsage(usage []bitVector, targetNumSectors uint64) (res []bitVector) {
	targetUsageSize := targetNumSectors / bitVectorGranularity
	if targetNumSectors%bitVectorGranularity != 0 {
		targetUsageSize++
	}
	curSize := len(usage)
	res = append(usage, make([]bitVector, int(targetUsageSize)-curSize)...)
	return
}

func shrinkUsage(usage []bitVector, targetNumSectors uint64) (res []bitVector) {
	targetUsageSize := targetNumSectors / bitVectorGranularity
	if targetNumSectors%bitVectorGranularity != 0 {
		targetUsageSize++
	}
	res = usage[:targetUsageSize]
	return
}
