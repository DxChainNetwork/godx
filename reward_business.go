package ethereum

import (
	"github.com/DxChainNetwork/godx/common"
)

const (
	EpochInterval          = uint64(86400)
	RewardedCandidateCount = 50
)

var (
	minRewardPerEpoch  = common.NewBigInt(1e18)
	minRewardPerBlock  = common.NewBigInt(1e18)
	minCandidateReward = common.NewBigInt(1e18)

	// deposit grades
	g1 = common.NewBigInt(1e18).MultInt64(1e3)
	g2 = common.NewBigInt(1e18).MultInt64(1e6)
	g3 = common.NewBigInt(1e18).MultInt64(1e9)
)

// 请求gdx节点的RPC接口：GetVoteLastEpoch，获取指定delegator在last周期触发选举时的vote
func GetVoteLastEpoch(delegator common.Address) common.BigInt {
	return common.NewBigInt(0)
}

// 请求gdx节点的RPC接口：GetVoteDuration，获取指定delegator在last周期触发选举时的质押时长duration
func GetVoteDuration(delegator common.Address) uint64 {
	return 0
}

// 请求gdx节点的RPC接口：GetValidatorDepositLastEpoch，获取指定的validator在last周期中竞选时的质押deposit
func GetValidatorDepositLastEpoch(validator common.Address) common.BigInt {
	return common.NewBigInt(0)
}

// 请求gdx节点的RPC接口：GetSubstituteCandidatesLastEpoch，获取last周期中备胎候选人列表
func GetSubstituteCandidatesLastEpoch() []common.Address {
	return nil
}

// 请求gdx节点的RPC接口：GetCandidateDepositLastEpoch，获取指定的candidate在last周期中
func GetCandidateDepositLastEpoch(candidate common.Address) common.BigInt {
	return common.NewBigInt(0)
}

// 活动一：奖励delegator在质押时长内，获得的deposit收益（类似于日常生活中的定期理财）
func RewardDelegator(delegator common.Address) common.BigInt {
	deposit := GetVoteLastEpoch(delegator)
	rewardPerEpoch := common.NewBigInt(0)
	duration := GetVoteDuration(delegator)
	passedEpochs := duration / uint64(EpochInterval)

	/*
		deposit < 1e3 dx: rewardPerEpoch = 1 dx
		deposit < 1e6 dx: rewardPerEpoch = 10 dx
		deposit < 1e9 dx: rewardPerEpoch = 100 dx
		deposit >= 1e9 dx: rewardPerEpoch = 1000 dx
	*/
	switch {
	case deposit.Cmp(g1) == -1:
		rewardPerEpoch = minRewardPerEpoch
		break
	case deposit.Cmp(g2) == -1:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(10)
		break
	case deposit.Cmp(g3) == -1:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(100)
		break
	default:
		rewardPerEpoch = minRewardPerEpoch.MultInt64(1000)
	}

	return rewardPerEpoch.MultUint64(passedEpochs)
}

// 活动二：奖励validator额外的出块奖励，根据deposit相应奖励每出一个block的钱
func RewardValidator(validator common.Address) common.BigInt {
	rewardPerBlock := common.NewBigInt(0)
	deposit := GetValidatorDepositLastEpoch(validator)

	/*
		deposit < 1e3 dx: rewardPerBlock = 1 dx
		deposit < 1e6 dx: rewardPerBlock = 2 dx
		deposit < 1e9 dx: rewardPerBlock = 3 dx
		deposit >= 1e9 dx: rewardPerBlock = 5 dx
	*/
	switch {
	case deposit.Cmp(g1) == -1:
		rewardPerBlock = minRewardPerBlock
		break
	case deposit.Cmp(g2) == -1:
		rewardPerBlock = minRewardPerBlock.MultInt64(2)
		break
	case deposit.Cmp(g3) == -1:
		rewardPerBlock = minRewardPerBlock.MultInt64(3)
		break
	default:
		rewardPerBlock = minRewardPerBlock.MultInt64(5)
	}

	return rewardPerBlock
}

// 活动三：奖励validator额外的出块奖励，根据deposit相应奖励每出一个block的钱
func RewardSubstituteCandidates(candidates []common.Address) map[common.Address]common.BigInt {
	SubstituteCandidates := GetSubstituteCandidatesLastEpoch()
	rewardedCandidates := make([]common.Address, 0)
	if len(SubstituteCandidates) < RewardedCandidateCount {
		rewardedCandidates = SubstituteCandidates
	} else {
		rewardedCandidates = SubstituteCandidates[:RewardedCandidateCount]
	}

	result := make(map[common.Address]common.BigInt, 0)
	additionalReward := common.NewBigInt(0)
	for _, candidate := range rewardedCandidates {
		deposit := GetCandidateDepositLastEpoch(candidate)
		/*
			deposit < 1e3 dx: additionalReward = 1 dx
			deposit < 1e6 dx: additionalReward = 10 dx
			deposit < 1e9 dx: additionalReward = 100 dx
			deposit >= 1e9 dx: additionalReward = 1000 dx
		*/
		switch {
		case deposit.Cmp(g1) == -1:
			additionalReward = minCandidateReward
			break
		case deposit.Cmp(g2) == -1:
			additionalReward = minCandidateReward.MultInt64(10)
			break
		case deposit.Cmp(g3) == -1:
			additionalReward = minCandidateReward.MultInt64(100)
			break
		default:
			additionalReward = minCandidateReward.MultInt64(1000)
		}

		result[candidate] = additionalReward
	}
	return result
}
