package vm

const (
	RewardRatio   = "RewardRatio"
	FrozenAssets  = "FrozenAssets"
	ThawingPeriod = "ThawingPeriod"
)

//CandidateInfo additional relevant data when sending the candidateTx.
type CandidateInfo struct {
	RewardRatio uint8 `json:"rewardRatio"`
}
