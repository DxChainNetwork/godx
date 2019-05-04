// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

type FilterMode int

const (
	FilterError FilterMode = iota
	DisableFilter
	ActivateBlacklist
	ActiveWhitelist
)
