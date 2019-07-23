package unit

// The block generation rate for Ethereum is 15s/block. Therefore, 240 blocks
// can be generated in an hour
var (
	BlockPerMin    = uint64(4)
	BlockPerHour   = uint64(240)
	BlocksPerDay   = 24 * BlockPerHour
	BlocksPerWeek  = 7 * BlocksPerDay
	BlocksPerMonth = 30 * BlocksPerDay
	BlocksPerYear  = 365 * BlocksPerDay
)
