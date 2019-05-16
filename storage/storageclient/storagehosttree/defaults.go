// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehosttree

import "errors"

// Definition of common error messages
var (
	ErrHostExists         = errors.New("storage host existed in the tree already")
	ErrHostNotExists      = errors.New("storage host cannot be found from the tree")
	ErrEvaluationTooLarge = errors.New("provided evaluation must be less than the total evaluation of the tree")
	ErrNodeNotOccupied    = errors.New("node returned is not occupied")
)

// IPV4 Prefix Length of the IP network
const (
	IPv4PrefixLength = 24
)
