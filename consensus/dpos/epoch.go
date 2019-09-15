// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import "errors"

var timeOfFirstBlock = int64(0)

const (
	// BlockInterval indicates that a block will be produced every 10 seconds
	BlockInterval = int64(10)

	// EpochInterval indicates that a new epoch will be elected every a day
	EpochInterval = int64(86400)
)

var (
	ErrInvalidMinedBlockTime = errors.New("invalid time to mined the block")
)

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

// calcBlockSlotAndEpochID calculate the block slot and epoch ID from the block time.
// If invalid block time stamp, ErrInvalidMinedBlockTime is returned.
func calcBlockSlotAndEpochID(blockTime int64) (epoch, slot int64, err error) {
	epoch = CalculateEpochID(blockTime)
	slot, err = calcBlockSlot(blockTime)
	return
}

// calcBlockSlot calculate the epoch ID and slot ID for the block time stamp.
// If not a valid slot, ErrInvalidMinedBlockTime will be returned.
func calcBlockSlot(blockTime int64) (int64, error) {
	offset := blockTime % EpochInterval
	if offset%BlockInterval != 0 {
		return 0, ErrInvalidMinedBlockTime
	}
	slot := offset / BlockInterval
	return slot, nil
}

// CalculateEpochID calculate the epoch ID given the block time
func CalculateEpochID(blockTime int64) int64 {
	return blockTime / EpochInterval
}
