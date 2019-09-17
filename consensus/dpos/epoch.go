// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import "github.com/DxChainNetwork/godx/consensus"

var timeOfFirstBlock = int64(0)

// expectedBlocksPerValidatorInEpoch return the expected number of blocks to be produced
// for each validator in an epoch. The input timeFirstBlock and curTime is passed in to
// calculate for the expected epoch number
func expectedBlocksPerValidatorInEpoch(timeFirstBlock, curTime int64) int64 {
	numBlocks := expectedBlocksInEpoch(timeFirstBlock, curTime)
	return numBlocks / MaxValidatorSize
}

// expectedBlocksInEpoch return the expected blocks to be produced in the epoch.
// The value is only different when currently is in the first block
func expectedBlocksInEpoch(timeFirstBlock int64, curTime int64) int64 {
	epochDuration := EpochInterval
	// First epoch duration may lt epoch interval,
	// while the first block time wouldn't always align with epoch interval,
	// so calculate the first epoch duration with first block time instead of epoch interval,
	// prevent the validators were kickout incorrectly.
	if diff := curTime - timeFirstBlock; diff < EpochInterval {
		epochDuration = diff
	}
	return epochDuration / BlockInterval
}

// calcBlockSlot calculate slot ID for the block time stamp.
// If not a valid slot, errInvalidMinedBlockTime will be returned.
func calcBlockSlot(blockTime int64) (int64, error) {
	offset := blockTime % EpochInterval
	if offset%BlockInterval != 0 {
		return 0, errInvalidMinedBlockTime
	}
	slot := offset / BlockInterval
	return slot, nil
}

// CalculateEpochID calculate the epoch ID given the block time
func CalculateEpochID(blockTime int64) int64 {
	return blockTime / EpochInterval
}

// updateTimeOfFirstBlockIfNecessary update the value of timeOfFirstBlock if the value is not assigned
func updateTimeOfFirstBlockIfNecessary(chain consensus.ChainReader) {
	if timeOfFirstBlock == 0 {
		if firstBlockHeader := chain.GetHeaderByNumber(1); firstBlockHeader != nil {
			timeOfFirstBlock = firstBlockHeader.Time.Int64()
		}
	}
}
