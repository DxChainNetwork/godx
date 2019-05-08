// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

// FilterMode defines a list of storage host that needs to be filtered
// there are four kinds of filter mode defined below
type FilterMode int

// Four kinds of filter mode
//  1. error
//  2. disable: filter mode is not allowed
//  3. blocklist: filter out the storage host in the black list
//  4. whitelist: filter the white list storage host
const (
	FilterError FilterMode = iota
	DisableFilter
	ActivateBlacklist
	ActiveWhitelist
)
