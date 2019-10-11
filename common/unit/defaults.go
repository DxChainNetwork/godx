// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package unit

// The block generation rate for gdx with dpos engine is 10s/block.
// Therefore, 360 blocks can be generated in an hour
const (
	BlocksPerMin   = uint64(4)
	BlocksPerHour  = uint64(360)
	BlocksPerDay   = 24 * BlocksPerHour
	BlocksPerWeek  = 7 * BlocksPerDay
	BlocksPerMonth = 30 * BlocksPerDay
	BlocksPerYear  = 365 * BlocksPerDay
)
