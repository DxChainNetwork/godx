// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import (
	"encoding/binary"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/consensus"
	"github.com/DxChainNetwork/godx/params"
)

var timeOfFirstBlock = int64(0)

// expectedBlocksPerValidatorInEpoch return the expected number of blocks to be produced
// for each validator in an epoch. The input timeFirstBlock and curTime is passed in to
// calculate for the expected epoch number
func expectedBlocksPerValidatorInEpoch(timeFirstBlock, blockNumber int64, curTime int64, state stateDB, config *params.DposConfig) int64 {
	numBlocks := expectedBlocksInEpoch(timeFirstBlock, curTime)
	maxValidatorSize := calMaxValidatorsNumbers(blockNumber, curTime, state, config)
	return numBlocks / int64(maxValidatorSize)
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

// getEpochTotalDepositAddress return the total candidates deposit address with the epoch
func getEpochTotalDepositAddress(epoch int64) common.Address {
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(epoch))
	b := append([]byte(PrefixEpochDepositAddr), epochBytes...)
	depositAddress := common.BytesToAddress(b)
	return depositAddress
}

// getCandidateNumberAddress return the candidates numbers address with the epoch
func getCandidateNumberAddress(epoch int64) common.Address {
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(epoch))
	b := append([]byte(PrefixCandidatesNumberAddr), epochBytes...)
	addr := common.BytesToAddress(b)
	return addr
}
