// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package dpos

import "errors"

var (
	// ErrAlreadyCandidate is the error that happens when an address try to become a candidate if
	// already a candidate
	ErrAlreadyCandidate = errors.New("already become candidate")

	// ErrAlreadyVote is the error that happens when an address try to vote if he has already voted
	ErrAlreadyVote = errors.New("already vote some candidates")

	// errInsufficientFrozenAssets is the error happens when subtracting frozen assets, the diff value is
	// larger the stored frozen assets
	errInsufficientFrozenAssets = errors.New("not enough frozen assets to subtract")
)
